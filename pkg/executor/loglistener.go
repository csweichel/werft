package executor

import (
	"bufio"
	"fmt"
	"io"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

type logListener struct {
	Clientset kubernetes.Interface
	Job       string
	Namespace string

	listener map[string]io.Closer
	started  time.Time
	ch       chan string
	closed   bool
	mu       sync.RWMutex
}

// Listen establishes a log listener for a job
func listenToLogs(client kubernetes.Interface, job, namespace string) <-chan string {
	ll := &logListener{
		Clientset: client,
		Job:       job,
		Namespace: namespace,
		started:   time.Now(),
		ch:        make(chan string),
		listener:  make(map[string]io.Closer),
	}
	go ll.Start()

	return ll.ch
}

// Write writes to the log listeners channel
func (ll *logListener) Write(b []byte) (n int, err error) {
	ll.mu.RLock()
	defer ll.mu.RUnlock()
	if ll.closed {
		return 0, io.EOF
	}

	ll.ch <- string(b)
	return len(b), nil
}

func (ll *logListener) Close() {
	ll.mu.Lock()
	defer ll.mu.Unlock()

	if ll.closed {
		return
	}

	for id, stp := range ll.listener {
		stp.Close()
		delete(ll.listener, id)
	}

	ll.closed = true
	close(ll.ch)
}

func (ll *logListener) Start() {
	podwatch, err := ll.Clientset.CoreV1().Pods(ll.Namespace).Watch(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", LabelJobName, ll.Job),
	})
	if err != nil {
		log.WithError(err).Warn("cannot watch for pod events")
		ll.Close()
		return
	}
	defer podwatch.Stop()

	for {
		e := <-podwatch.ResultChan()
		if e.Object == nil {
			// Closed because of error
			return
		}
		pod, ok := e.Object.(*corev1.Pod)
		if !ok {
			// not a pod
			return
		}

		switch e.Type {
		case watch.Added, watch.Modified:
			var statuses []corev1.ContainerStatus
			statuses = append(statuses, pod.Status.InitContainerStatuses...)
			statuses = append(statuses, pod.Status.ContainerStatuses...)

			for _, c := range statuses {
				if c.State.Running != nil {
					ll.tail(pod.Name, c.Name)
				}
			}
		case watch.Deleted:
			var statuses []corev1.ContainerStatus
			statuses = append(statuses, pod.Status.InitContainerStatuses...)
			statuses = append(statuses, pod.Status.ContainerStatuses...)

			for _, c := range statuses {
				if c.State.Terminated != nil {
					ll.stopTailing(pod.Name, c.Name)
				}
			}
		}
	}
}

func (ll *logListener) tail(pod, container string) {
	ll.mu.Lock()
	defer ll.mu.Unlock()

	id := fmt.Sprintf("%s/%s", pod, container)
	_, ok := ll.listener[id]
	if ok {
		// we're already listening
		return
	}

	// we have to start listenting
	req := ll.Clientset.CoreV1().Pods(ll.Namespace).GetLogs(pod, &corev1.PodLogOptions{
		Container: container,
		Follow:    true,
		Previous:  false,
	})
	logs, err := req.Stream()
	if err != nil {
		log.WithError(err).Debug("cannot connect to logs")
		return
	}
	ll.listener[id] = logs

	// forward the logs
	go func() {
		scanner := bufio.NewScanner(logs)
		for scanner.Scan() {
			line := scanner.Text()
			ll.ch <- line
		}
	}()
}

func (ll *logListener) stopTailing(pod, container string) {
	ll.mu.Lock()
	defer ll.mu.Unlock()

	id := fmt.Sprintf("%s/%s", pod, container)
	stp, ok := ll.listener[id]
	if !ok {
		// we're already listening
		return
	}

	stp.Close()
	delete(ll.listener, id)
}

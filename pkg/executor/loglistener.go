package executor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
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
	Labels    labelSet

	listener map[string]io.Closer
	started  time.Time
	closed   bool
	mu       sync.RWMutex

	out  io.Reader
	in   io.WriteCloser
	inmu sync.Mutex
}

// Listen establishes a log listener for a job
func listenToLogs(client kubernetes.Interface, job, namespace string, labels labelSet) io.Reader {
	ll := &logListener{
		Clientset: client,
		Job:       job,
		Namespace: namespace,
		Labels:    labels,
		started:   time.Now(),
		listener:  make(map[string]io.Closer),
	}
	ll.out, ll.in = io.Pipe()
	go ll.Start()

	return ll.out
}

func (ll *logListener) Close() error {
	ll.mu.Lock()
	defer ll.mu.Unlock()

	if ll.closed {
		return nil
	}

	for id, stp := range ll.listener {
		stp.Close()
		delete(ll.listener, id)
	}

	ll.closed = true
	ll.inmu.Lock()
	defer ll.inmu.Unlock()

	return ll.in.Close()
}

func (ll *logListener) Start() {
	podwatch, err := ll.Clientset.CoreV1().Pods(ll.Namespace).Watch(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", ll.Labels.LabelJobName, ll.Job),
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
					var prefix string
					if isSidecar := strings.Contains(pod.Annotations[ll.Labels.AnnotationSidecars], c.Name); isSidecar {
						prefix = fmt.Sprintf("[%s] ", c.Name)
					}
					go ll.tail(pod.Name, c.Name, prefix)
				}
			}
		case watch.Deleted:
			var statuses []corev1.ContainerStatus
			statuses = append(statuses, pod.Status.InitContainerStatuses...)
			statuses = append(statuses, pod.Status.ContainerStatuses...)

			for _, c := range statuses {
				if c.State.Terminated != nil {
					go ll.stopTailing(pod.Name, c.Name)
				}
			}
		}
	}
}

func (ll *logListener) tail(pod, container, prefix string) {
	var once sync.Once

	ll.mu.Lock()
	defer once.Do(ll.mu.Unlock)

	id := fmt.Sprintf("%s/%s", pod, container)
	_, ok := ll.listener[id]
	if ok {
		// we're already listening
		return
	}

	log.WithField("id", id).Debug("tailing container")

	// we have to start listenting
	req := ll.Clientset.CoreV1().Pods(ll.Namespace).GetLogs(pod, &corev1.PodLogOptions{
		Container: container,
		Follow:    true,
		Previous:  false,
	})
	logs, err := req.Stream(context.Background())
	if err != nil {
		log.WithError(err).Debug("cannot connect to logs")
		return
	}
	ll.listener[id] = logs
	once.Do(ll.mu.Unlock)

	// forward the logs line by line to ensure we don't mix the output of different conainer
	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		line := scanner.Text()
		ll.inmu.Lock()
		ll.in.Write([]byte(prefix + line + "\n"))
		ll.inmu.Unlock()
	}
}

func (ll *logListener) stopTailing(pod, container string) {
	ll.mu.Lock()
	defer ll.mu.Unlock()

	id := fmt.Sprintf("%s/%s", pod, container)
	stp, ok := ll.listener[id]
	if !ok {
		// we're not listening
		return
	}

	log.WithField("id", id).Debug("stopped tailing container")

	stp.Close()
	delete(ll.listener, id)
}

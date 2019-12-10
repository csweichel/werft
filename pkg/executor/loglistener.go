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

	started time.Time
	ch      chan string
	closed  bool
	mu      sync.RWMutex
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

// Listen establishes a log listener for a job
func listenToLogs(client kubernetes.Interface, job, namespace string) <-chan string {
	ll := &logListener{
		Clientset: client,
		Job:       job,
		Namespace: namespace,
		started:   time.Now(),
		ch:        make(chan string),
	}
	go ll.Start()

	return ll.ch
}

func (ll *logListener) Close() {
	ll.mu.Lock()
	defer ll.mu.Unlock()

	if ll.closed {
		return
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

	fwdLogs := func(pod, container string) {
		defer recover()

		log.WithField("pod", pod).WithField("container", container).Debug("forwarding logs")
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
		defer logs.Close()

		scanner := bufio.NewScanner(logs)
		for scanner.Scan() {
			ll.mu.RLock()
			closed := ll.closed
			ll.mu.RUnlock()
			if closed {
				break
			}

			line := scanner.Text()
			ll.ch <- line
		}
	}

	// TODO: initially get container, don't just wait for it

	var (
		lastPod       string
		lastContainer string
	)
	for {
		evt := <-podwatch.ResultChan()
		if evt.Type == watch.Deleted {
			break
		}

		podobj := evt.Object.(*corev1.Pod)
		var container string
		if podobj.Status.Phase != corev1.PodRunning {
			container = podobj.Spec.InitContainers[0].Name
		} else {
			container = podobj.Spec.Containers[0].Name
		}

		if lastPod == podobj.Name && lastContainer == container {
			continue
		}
		lastPod = podobj.Name
		lastContainer = container

		go fwdLogs(podobj.Name, container)
	}
}

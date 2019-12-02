package executor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	v1 "github.com/32leaves/keel/pkg/api/v1"
	log "github.com/sirupsen/logrus"
	"github.com/technosophos/moniker"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// LabelKeelMarker is the label applied to all jobs and configmaps. This label can be used
	// to search for keel job objects in Kubernetes.
	LabelKeelMarker = "keel.sh/job"

	// UserDataAnnotationPrefix is prepended to all user annotations added to jobs
	UserDataAnnotationPrefix = "userdata.keel.sh"

	// AnnotationFailureLimit is the annotation denoting the max times a job may fail
	AnnotationFailureLimit = "keel.sh/failureLimit"
)

// Config configures the executor
type Config struct {
	Namespace     string `json:"namespace"`
	EventTraceLog string `json:"eventTraceLog,omitempty"`
}

// ActiveJob is a currently running job
type ActiveJob struct {
	Annotations map[string]string
}

// NewExecutor creates a new job center instance
func NewExecutor(config Config, client kubernetes.Interface) *Executor {
	return &Executor{
		OnError:  func(err error) {},
		OnUpdate: func(status *v1.JobStatus) {},

		config: config,
		client: client,
	}
}

// Executor starts and watches jobs running in Kubernetes
type Executor struct {
	// OnError is called if something goes wrong with the continuous operation of the executor
	OnError func(err error)

	// OnUpdate is called when the status of a job changes.
	// Beware: this function can be called several times with the same status.
	OnUpdate func(status *v1.JobStatus)

	config     Config
	client     kubernetes.Interface
	KubeConfig *rest.Config
}

// Run starts the executor and returns immediately
func (js *Executor) Run() {
	go js.monitorJobs()
	go js.doHousekeeping()
}

type startOptions struct {
	JobName     string
	Modifier    []func(*corev1.Pod)
	Annotations map[string]string
}

// StartOpt configures a job at startup
type StartOpt func(*startOptions)

// WithBackoff configures the backoff behaviour of a job
func WithBackoff(limit int) StartOpt {
	return func(opts *startOptions) {
		opts.Modifier = append(opts.Modifier, func(j *corev1.Pod) {
			j.Annotations[AnnotationFailureLimit] = fmt.Sprintf("%d", limit)
		})
	}
}

// WithAnnotation sets a single annotation on a job
func WithAnnotation(key, value string) StartOpt {
	return func(opts *startOptions) {
		if opts.Annotations == nil {
			opts.Annotations = make(map[string]string)
		}
		opts.Annotations[key] = value
	}
}

// WithAnnotations sets all annotations of a job
func WithAnnotations(annotations map[string]string) StartOpt {
	return func(opts *startOptions) {
		opts.Annotations = annotations
	}
}

// WithName sets the name of the job
func WithName(name string) StartOpt {
	return func(opts *startOptions) {
		opts.JobName = name
	}
}

// Start starts a new job
func (js *Executor) Start(podspec corev1.PodSpec, options ...StartOpt) (id string, err error) {
	opts := startOptions{
		JobName: fmt.Sprintf("keel-%s", strings.ReplaceAll(moniker.New().Name(), " ", "-")),
	}
	for _, opt := range options {
		opt(&opts)
	}

	// TODO: store as configmap for housekeeping

	annotations := make(map[string]string)
	for key, val := range opts.Annotations {
		annotations[fmt.Sprintf("%s/%s", UserDataAnnotationPrefix, key)] = val
	}

	if podspec.RestartPolicy != corev1.RestartPolicyNever && podspec.RestartPolicy != corev1.RestartPolicyOnFailure {
		podspec.RestartPolicy = corev1.RestartPolicyOnFailure
	}

	meta := metav1.ObjectMeta{
		Name: opts.JobName,
		Labels: map[string]string{
			LabelKeelMarker: "true",
			LabelJobName:    opts.JobName,
		},
		Annotations: annotations,
	}
	poddesc := corev1.Pod{
		ObjectMeta: meta,
		Spec:       podspec,
	}
	for _, opt := range opts.Modifier {
		opt(&poddesc)
	}

	if log.GetLevel() == log.DebugLevel {
		dbg, _ := json.MarshalIndent(poddesc, "", "  ")
		log.Debugf("scheduling job\n%s", dbg)
	}

	job, err := js.client.CoreV1().Pods(js.config.Namespace).Create(&poddesc)
	if err != nil {
		return "", err
	}

	return job.Name, nil
}

func (js *Executor) monitorJobs() {
	incoming, err := js.client.CoreV1().Pods(js.config.Namespace).Watch(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=true", LabelKeelMarker),
	})
	if err != nil {
		js.OnError(xerrors.Errorf("cannot watch jobs, monitor is shutting down: %w", err))
	}

	for evt := range incoming.ResultChan() {
		obj := evt.Object.(*corev1.Pod)
		js.handleJobEvent(evt.Type, obj)
	}

	// TODO: handle reconnect
	// TODO: handle graceful shutdown
}

func (js *Executor) handleJobEvent(evttpe watch.EventType, obj *corev1.Pod) {
	status, err := getStatus(obj)
	js.writeEventTraceLog(status, obj)
	if err != nil {
		js.OnError(err)
		return
	}

	js.OnUpdate(status)
	err = js.actOnUpdate(status, obj)
	if err != nil {
		js.OnError(err)
		return
	}
}

func (js *Executor) actOnUpdate(status *v1.JobStatus, obj *corev1.Pod) error {
	if status.Phase == v1.JobPhase_PHASE_DONE {
		gracePeriod := int64(30)
		policy := metav1.DeletePropagationForeground

		err := js.client.CoreV1().Pods(js.config.Namespace).Delete(obj.Name, &metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
			PropagationPolicy:  &policy,
		})
		if err != nil {
			return err
		}

		return nil
	}

	return nil
}

func (js *Executor) writeEventTraceLog(status *v1.JobStatus, obj *corev1.Pod) {
	// make sure we recover from a panic in this function - not that we expect this to ever happen
	//nolint:errcheck
	defer recover()

	if js.config.EventTraceLog == "" {
		return
	}

	var out io.Writer
	if js.config.EventTraceLog == "-" {
		out = os.Stdout
	} else {
		f, err := os.OpenFile(js.config.EventTraceLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer f.Close()

		out = f
	}

	type eventTraceEntry struct {
		Time   string        `json:"time"`
		Status *v1.JobStatus `json:"status"`
		Job    *corev1.Pod   `json:"job"`
	}
	// If writing the event trace log fails that does nothing to harm the function of ws-manager.
	// In fact we don't even want to react to it, hence the nolint.
	//nolint:errcheck
	json.NewEncoder(out).Encode(eventTraceEntry{Time: time.Now().Format(time.RFC3339), Status: status, Job: obj})
}

// Logs provides the log output of a running job. If the job is unknown, nil is returned.
func (js *Executor) Logs(id string) <-chan string {
	// provide log chan
	return nil
}

func (js *Executor) doHousekeeping() {
	// check our state and watch for non-existent jobs/events that we missed
}

// Find finds currently running jobs
func (js *Executor) Find(filter []*v1.AnnotationFilter, limit int64) ([]v1.JobStatus, error) {
	_, err := js.client.BatchV1().Jobs(js.config.Namespace).List(metav1.ListOptions{
		Limit: limit,
	})
	if err != nil {
		return nil, err
	}

	// TOOD: transform jobs
	return nil, fmt.Errorf("finish this shit")
}

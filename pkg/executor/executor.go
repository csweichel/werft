package executor

import (
	"fmt"

	v1 "github.com/32leaves/keel/pkg/api/v1"
	"golang.org/x/xerrors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

const (
	// LabelKeelMarker is the label applied to all jobs and configmaps. This label can be used
	// to search for keel job objects in Kubernetes.
	LabelKeelMarker = "keel.sh/job"

	// UserDataAnnotationPrefix is prepended to all user annotations added to jobs
	UserDataAnnotationPrefix = "keel.sh/userdata"
)

// Config configures the executor
type Config struct {
	Namespace string
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

	config Config
	client kubernetes.Interface
}

// Run starts the executor and returns immediately
func (js *Executor) Run() {
	go js.monitorJobs()
	go js.doHousekeeping()
}

type startOptions struct {
	JobName     string
	Modifier    []func(*batchv1.Job)
	Annotations map[string]string
}

// StartOpt configures a job at startup
type StartOpt func(*startOptions)

// WithBackoff configures the backoff behaviour of a job
func WithBackoff(limit int) StartOpt {
	return func(opts *startOptions) {
		opts.Modifier = append(opts.Modifier, func(j *batchv1.Job) {
			val := int32(limit)
			j.Spec.BackoffLimit = &val
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

// Start starts a new job
func (js *Executor) Start(podspec corev1.PodSpec, options ...StartOpt) (id string, err error) {
	opts := startOptions{
		JobName: "generate-random-name",
	}
	for _, opt := range options {
		opt(&opts)
	}

	// TODO: store as configmap for housekeeping

	annotations := make(map[string]string)
	for key, val := range opts.Annotations {
		annotations[fmt.Sprintf("%s/%s", UserDataAnnotationPrefix, key)] = val
	}

	meta := metav1.ObjectMeta{
		Name: opts.JobName,
		Labels: map[string]string{
			LabelKeelMarker: "true",
			LabelJobName:    opts.JobName,
		},
		Annotations: annotations,
	}
	jobdesc := batchv1.Job{
		ObjectMeta: meta,
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: meta,
				Spec:       podspec,
			},
		},
	}
	for _, opt := range opts.Modifier {
		opt(&jobdesc)
	}

	job, err := js.client.BatchV1().Jobs(js.config.Namespace).Create(&jobdesc)
	if err != nil {
		return "", err
	}

	return job.Name, nil
}

func (js *Executor) monitorJobs() {
	incoming, err := js.client.BatchV1().Jobs(js.config.Namespace).Watch(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=true", LabelKeelMarker),
	})
	if err != nil {
		js.OnError(xerrors.Errorf("cannot watch jobs, monitor is shutting down: %w", err))
	}

	for evt := range incoming.ResultChan() {
		obj := evt.Object.(*batchv1.Job)
		js.handleJobEvent(evt.Type, obj)
	}

	// TODO: handle reconnect
	// TODO: handle graceful shutdown
}

func (js *Executor) handleJobEvent(evttpe watch.EventType, obj *batchv1.Job) {
	status, err := getStatus(obj)
	if err != nil {
		js.OnError(xerrors.Errorf("cannot compute job status: %w", err))
		return
	}

	js.OnUpdate(status)
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

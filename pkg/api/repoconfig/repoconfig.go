package repoconfig

import (
	werftv1 "github.com/32leaves/werft/pkg/api/v1"
	corev1 "k8s.io/api/core/v1"
)

// C is the struct we expect to find in the repo root which configures how we build things
type C struct {
	DefaultJob string `yaml:"defaultJob"`
}

// TemplatePath returns the path to the job template in the repo
func (rc *C) TemplatePath(trigger werftv1.JobTrigger) string {
	return rc.DefaultJob
}

// ShouldRun determines based on the repo config if the job should run
func (rc *C) ShouldRun(trigger werftv1.JobTrigger) bool {
	return true
}

// JobSpec is the format of the files we expect to find when starting jobs
type JobSpec struct {
	// Desc describes the purpose of this job spec.
	Desc string `yaml:"description,omitempty"`

	// Pod is the actual job spec to start. Prior to deploying this to Kubernetes, we'll run this
	// as a Go template.
	Pod *corev1.PodSpec `yaml:"pod"`

	// Mutex makes job execution exclusive, with new ones canceling the currently running one.
	// For example: job A is running at the moment, and job B is about to start. If A and B share the
	// same mutex, B will cancel A.
	Mutex string `yaml:"lock,omitempty"`

	// Args describe annotations which this job expects. This list is only used on the UI when manually
	// starting the job.
	// This is list is neither exhaustive (i.e. jobs can use annotations not listed here), nor binding
	// (i.e. jobs can run even when annotations listed here are not present). What matters for a job to
	// run is only if Kubernetes accepts the produced podspec.
	Args []ArgSpec `yaml:"args,omitempty"`
}

// ArgSpec specifies an argument/annotation for a job.
type ArgSpec struct {
	Name string `yaml:"name"`
	Req  bool   `yaml:"required"`
	Desc string `yaml:"description"`
}

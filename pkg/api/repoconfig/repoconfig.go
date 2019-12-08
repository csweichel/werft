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
	// Pod is the actual job spec to start. Prior to deploying this to Kubernetes, we'll run this
	// as a Go template.
	Pod *corev1.PodSpec `yaml:"pod"`
}

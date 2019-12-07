package repoconfig

import v1 "github.com/32leaves/werft/pkg/api/v1"

// C is the struct we expect to find in the repo root which configures how we build things
type C struct {
	DefaultJob string `yaml:"defaultJob"`
}

// TemplatePath returns the path to the job template in the repo
func (rc *C) TemplatePath(trigger v1.JobTrigger) string {
	return rc.DefaultJob
}

// ShouldRun determines based on the repo config if the job should run
func (rc *C) ShouldRun(trigger v1.JobTrigger) bool {
	return true
}

package keel

import (
	"fmt"
)

// JobContext configures the context of a CI job
type JobContext struct {
	Owner    string
	Repo     string
	Revision string
	Trigger  JobTrigger
}

// JobTrigger determines what triggered the job
type JobTrigger string

const (
	// JobTriggerPush means someone pushed to the repo
	JobTriggerPush JobTrigger = "push"
)

func (jc *JobContext) String() string {
	return fmt.Sprintf("%s/%s@%s", jc.Owner, jc.Repo, jc.Revision)
}

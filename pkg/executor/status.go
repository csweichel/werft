package executor

import (
	"strings"

	v1 "github.com/32leaves/keel/pkg/api/v1"
	"golang.org/x/xerrors"
	batchv1 "k8s.io/api/batch/v1"
)

const (
	// LabelJobName adds the ID of the job to the k8s object
	LabelJobName = "keel.sh/jobName"
)

// extracts the phase from the job object
func getStatus(obj *batchv1.Job) (*v1.JobStatus, error) {
	name, hasName := getJobName(obj)
	if !hasName {
		return nil, xerrors.Errorf("job has no name: %v", obj.Name)
	}

	// TODO: implement me

	return &v1.JobStatus{
		Name:  name,
		Phase: v1.JobPhase_PHASE_UNKNOWN,
	}, nil
}

func getJobName(obj *batchv1.Job) (id string, ok bool) {
	id, ok = obj.Labels[LabelJobName]
	return
}

func getUserData(obj *batchv1.Job) map[string]string {
	res := make(map[string]string)
	for key, val := range obj.Annotations {
		if strings.HasPrefix(key, UserDataAnnotationPrefix) {
			res[strings.TrimPrefix(key, UserDataAnnotationPrefix)] = val
		}
	}
	return res
}

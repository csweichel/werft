package executor

import (
	"strconv"
	"strings"

	v1 "github.com/32leaves/keel/pkg/api/v1"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
)

const (
	// LabelJobName adds the ID of the job to the k8s object
	LabelJobName = "keel.sh/jobName"
)

// extracts the phase from the job object
func getStatus(obj *corev1.Pod) (status *v1.JobStatus, err error) {
	name, hasName := getJobName(obj)
	if !hasName {
		return nil, xerrors.Errorf("job has no name: %v", obj.Name)
	}

	status = &v1.JobStatus{
		Name:  name,
		Phase: v1.JobPhase_PHASE_UNKNOWN,
		Conditions: &v1.JobConditions{
			Success: true,
		},
	}

	var (
		statuses      = append(obj.Status.InitContainerStatuses, obj.Status.ContainerStatuses...)
		anyFailed     bool
		maxRestart    int32
		allTerminated = len(statuses) != 0
	)
	for _, cs := range statuses {
		if w := cs.State.Waiting; w != nil && w.Reason == "ErrImagePull" {
			status.Phase = v1.JobPhase_PHASE_DONE
			status.Conditions.Success = false
			status.Details = w.Message
			return
		}

		if cs.State.Terminated != nil {
			if cs.State.Terminated.ExitCode != 0 {
				anyFailed = true
			}
		} else {
			allTerminated = false
		}

		if cs.RestartCount >= maxRestart {
			maxRestart = cs.RestartCount
		}
	}
	status.Conditions.FailureCount = maxRestart
	status.Conditions.Success = !(anyFailed || maxRestart > getFailureLimit(obj))

	if obj.DeletionTimestamp != nil {
		status.Phase = v1.JobPhase_PHASE_CLEANUP
		return
	}
	if maxRestart > getFailureLimit(obj) {
		status.Phase = v1.JobPhase_PHASE_DONE
		return
	}
	if allTerminated {
		status.Phase = v1.JobPhase_PHASE_DONE
		return
	}

	switch obj.Status.Phase {
	case corev1.PodPending:
		status.Phase = v1.JobPhase_PHASE_PREPARING
		return
	case corev1.PodRunning:
		status.Phase = v1.JobPhase_PHASE_RUNNING
	}

	return
}

func getFailureLimit(obj *corev1.Pod) int32 {
	val := obj.Annotations[AnnotationFailureLimit]
	if val == "" {
		val = "0"
	}

	res, _ := strconv.ParseInt(val, 10, 32)
	return int32(res)
}

func getJobName(obj *corev1.Pod) (id string, ok bool) {
	id, ok = obj.Labels[LabelJobName]
	return
}

func getUserData(obj *corev1.Pod) map[string]string {
	res := make(map[string]string)
	for key, val := range obj.Annotations {
		if strings.HasPrefix(key, UserDataAnnotationPrefix) {
			res[strings.TrimPrefix(key, UserDataAnnotationPrefix)] = val
		}
	}
	return res
}

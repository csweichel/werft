package executor

import (
	"encoding/json"
	"strconv"
	"strings"

	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
)

const (
	// LabelJobName adds the ID of the job to the k8s object
	LabelJobName = "werft.sh/jobName"

	// LabelMutex makes jobs findable via their mutex
	LabelMutex = "werft.sh/mutex"
)

// extracts the phase from the job object
func getStatus(obj *corev1.Pod) (status *v1.JobStatus, err error) {
	defer func() {
		if status != nil && status.Phase == v1.JobPhase_PHASE_DONE {
			status.Metadata.Finished = ptypes.TimestampNow()
		}
	}()

	name, hasName := getJobName(obj)
	if !hasName {
		return nil, xerrors.Errorf("job has no name: %v", obj.Name)
	}

	rawmd, ok := obj.Annotations[AnnotationMetadata]
	if !ok {
		return nil, xerrors.Errorf("job has no metadata")
	}
	var md v1.JobMetadata
	err = jsonpb.UnmarshalString(rawmd, &md)
	if err != nil {
		return nil, xerrors.Errorf("cannot unmarshal metadata: %w", err)
	}

	var results []*v1.JobResult
	if c, ok := obj.Annotations[AnnotationResults]; ok {
		err = json.Unmarshal([]byte(c), &results)
		if err != nil {
			return nil, xerrors.Errorf("cannot unmarshal results: %w", err)
		}
	}

	_, canReplay := obj.Annotations[AnnotationCanReplay]
	status = &v1.JobStatus{
		Name:     name,
		Metadata: &md,
		Phase:    v1.JobPhase_PHASE_UNKNOWN,
		Conditions: &v1.JobConditions{
			Success:   true,
			CanReplay: canReplay,
		},
		Results: results,
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

	if msg, failed := obj.Annotations[AnnotationFailed]; failed {
		status.Phase = v1.JobPhase_PHASE_DONE
		if obj.DeletionTimestamp != nil {
			status.Phase = v1.JobPhase_PHASE_CLEANUP
		}
		status.Conditions.Success = false
		status.Details = msg

		return
	}
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

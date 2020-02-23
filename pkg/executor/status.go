package executor

import (
	"encoding/json"
	"strconv"
	"time"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
)

// extracts the phase from the job object
func getStatus(obj *corev1.Pod, labels labelSet) (status *v1.JobStatus, err error) {
	defer func() {
		if status != nil && status.Phase == v1.JobPhase_PHASE_DONE {
			status.Metadata.Finished = ptypes.TimestampNow()
		}
	}()

	name, hasName := getJobName(obj, labels)
	if !hasName {
		return nil, xerrors.Errorf("job has no name: %v", obj.Name)
	}

	rawmd, ok := obj.Annotations[labels.AnnotationMetadata]
	if !ok {
		return nil, xerrors.Errorf("job has no metadata")
	}
	var md v1.JobMetadata
	err = jsonpb.UnmarshalString(rawmd, &md)
	if err != nil {
		return nil, xerrors.Errorf("cannot unmarshal metadata: %w", err)
	}

	var results []*v1.JobResult
	if c, ok := obj.Annotations[labels.AnnotationResults]; ok {
		err = json.Unmarshal([]byte(c), &results)
		if err != nil {
			return nil, xerrors.Errorf("cannot unmarshal results: %w", err)
		}
	}

	annotationCanReplay := labels.AnnotationCanReplay
	_, canReplay := obj.Annotations[annotationCanReplay]

	annotationWaitUntil := labels.AnnotationWaitUntil
	var waitUntil *timestamp.Timestamp
	if wt, ok := obj.Annotations[annotationWaitUntil]; ok {
		ts, err := time.Parse(time.RFC3339, wt)
		if err != nil {
			return nil, xerrors.Errorf("cannot parse %s annotation: %w", annotationWaitUntil, err)
		}
		waitUntil, err = ptypes.TimestampProto(ts)
		if err != nil {
			return nil, xerrors.Errorf("cannot convert %s annotation: %w", annotationWaitUntil, err)
		}
	}

	status = &v1.JobStatus{
		Name:     name,
		Metadata: &md,
		Phase:    v1.JobPhase_PHASE_UNKNOWN,
		Conditions: &v1.JobConditions{
			Success:   true,
			CanReplay: canReplay,
			WaitUntil: waitUntil,
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
	status.Conditions.Success = !(anyFailed || maxRestart > getFailureLimit(obj, labels))
	status.Conditions.DidExecute = obj.Status.Phase != "" || len(statuses) > 0

	if msg, failed := obj.Annotations[labels.AnnotationFailed]; failed {
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
	if maxRestart > getFailureLimit(obj, labels) {
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

func getFailureLimit(obj *corev1.Pod, labels labelSet) int32 {
	val := obj.Annotations[labels.AnnotationFailureLimit]
	if val == "" {
		val = "0"
	}

	res, _ := strconv.ParseInt(val, 10, 32)
	return int32(res)
}

func getJobName(obj *corev1.Pod, labels labelSet) (id string, ok bool) {
	id, ok = obj.Labels[labels.LabelJobName]
	return
}

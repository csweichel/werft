package main

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/csweichel/werft/pkg/api/v1/mock"
	"github.com/csweichel/werft/pkg/logcutter"
	"github.com/csweichel/werft/pkg/plugin/client"
	"github.com/golang/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"

	_ "embed"
)

func TestOtelExporterPlugin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		jobName = "test-job"
		jobMD   = &v1.JobMetadata{
			Owner:      "someone",
			Repository: &v1.Repository{},
			Trigger:    v1.JobTrigger_TRIGGER_MANUAL,
			Created:    timestamppb.New(time.UnixMilli(0)),
		}
	)

	var statusIdx int
	status := []*v1.JobStatus{
		{
			Name:     jobName,
			Metadata: jobMD,
			Phase:    v1.JobPhase_PHASE_PREPARING,
		},
	}

	sub := mock.NewMockWerftService_SubscribeClient(ctrl)
	sub.EXPECT().Recv().DoAndReturn(func() (*v1.SubscribeResponse, error) {
		if statusIdx >= len(status) {
			time.Sleep(1 * time.Millisecond)
			return nil, io.EOF
		}
		res := &v1.SubscribeResponse{Result: status[statusIdx]}
		statusIdx++
		return res, nil
	}).AnyTimes()

	logevt, errchan := logcutter.DefaultCutter.Slice(bytes.NewReader([]byte(logtext)))

	logsClient := mock.NewMockWerftService_ListenClient(ctrl)
	logsClient.EXPECT().Recv().DoAndReturn(func() (*v1.ListenResponse, error) {
		select {
		case err := <-errchan:
			return nil, err
		case evt := <-logevt:
			if evt != nil && evt.Type != v1.LogSliceType_SLICE_CONTENT {
				time.Sleep(1 * time.Millisecond)
				t.Log(evt)
			}
			return &v1.ListenResponse{Content: &v1.ListenResponse_Slice{Slice: evt}}, nil
		}
	}).AnyTimes()

	werftClient := mock.NewMockWerftServiceClient(ctrl)
	werftClient.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Return(sub, nil)
	werftClient.EXPECT().Listen(gomock.Any(), gomock.Any()).Return(logsClient, nil)

	plugin := &otelExporterPlugin{}
	err := plugin.Run(context.Background(), &Config{
		Exporter: OTelExporterStdout,
	}, &client.Services{WerftServiceClient: werftClient})
	if err != nil {
		t.Fatal(err)
	}
}

//go:embed example-log.txt
var logtext string

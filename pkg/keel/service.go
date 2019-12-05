package keel

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"

	v1 "github.com/32leaves/keel/pkg/api/v1"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StartLocalJob starts a job whoose content is uploaded
func (srv *Service) StartLocalJob(inc v1.KeelService_StartLocalJobServer) error {
	req, err := inc.Recv()
	if err != nil {
		return err
	}
	if req.GetMetadata() == nil {
		return status.Error(codes.InvalidArgument, "first request must contain metadata")
	}
	md := *req.GetMetadata()
	log.WithField("name", md).Debug("StartLocalJob - received metadata")

	var (
		configYAML []byte
		jobYAML    []byte
		phase      int
	)
	const (
		phaseConfigYaml   = 0
		phaseJobYaml      = 1
		phaseWorkspaceTar = 2
	)
	for {
		req, err = inc.Recv()
		if err != nil {
			return err
		}
		if req.GetConfigYaml() != nil {
			if phase != phaseConfigYaml {
				return status.Error(codes.InvalidArgument, "expected config yaml")
			}

			configYAML = append(configYAML, req.GetConfigYaml()...)
			continue
		}
		if req.GetJobYaml() != nil {
			if phase == phaseConfigYaml {
				phase = phaseJobYaml
			}
			if phase != phaseJobYaml {
				return status.Error(codes.InvalidArgument, "expected job yaml")
			}

			jobYAML = append(jobYAML, req.GetJobYaml()...)
			continue
		}
		if req.GetWorkspaceTar() != nil {
			phase = phaseWorkspaceTar
			break
		}
	}

	if len(configYAML) == 0 {
		return status.Error(codes.InvalidArgument, "config YAML must not be empty")
	}
	if len(jobYAML) == 0 {
		return status.Error(codes.InvalidArgument, "job YAML must not be empty")
	}

	ts := newTarStreamAdapter(inc, req.GetWorkspaceTar())

	dbgf, err := os.OpenFile("/tmp/dbg.tar.gz", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	defer dbgf.Close()
	ts = io.TeeReader(ts, dbgf)

	fp := func(path string) (io.ReadCloser, error) {
		if path == PathKeelConfig {
			return ioutil.NopCloser(bytes.NewReader(configYAML)), nil
		}
		return ioutil.NopCloser(bytes.NewReader(jobYAML)), nil
	}
	cp := &LocalContentProvider{
		FileProvider: fp,
		TarStream:    ts,
		Namespace:    srv.Executor.Config.Namespace,
		Kubeconfig:   srv.Executor.KubeConfig,
		Clientset:    srv.Executor.Client,
	}
	jobStatus, err := srv.RunJob(inc.Context(), md, cp)

	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	log.WithField("status", jobStatus).Info(("started new local job"))
	return inc.SendAndClose(&v1.StartJobResponse{
		Status: jobStatus,
	})
}

// newTarStreamAdapter creates a reader from an incoming workspace tar stream
func newTarStreamAdapter(inc v1.KeelService_StartLocalJobServer, initial []byte) io.Reader {
	return &tarStreamAdapter{
		inc:       inc,
		remainder: initial,
	}
}

// tarStreamAdapter turns a client-side data stream into an io.Reader
type tarStreamAdapter struct {
	inc       v1.KeelService_StartLocalJobServer
	remainder []byte
}

// Read reads from incoming stream
func (tsa *tarStreamAdapter) Read(p []byte) (n int, err error) {
	if len(tsa.remainder) == 0 {
		var msg *v1.StartLocalJobRequest
		msg, err = tsa.inc.Recv()
		if err != nil {
			return 0, err
		}
		data := msg.GetWorkspaceTar()
		if data == nil {
			log.Debug("tar upload done")
			return 0, io.EOF
		}

		n = copy(p, data)
		tsa.remainder = data[n:]

		return
	}
	n = copy(p, tsa.remainder)
	tsa.remainder = tsa.remainder[n:]

	return n, nil
}

// ListJobs lists jobs
func (srv *Service) ListJobs(ctx context.Context, req *v1.ListJobsRequest) (resp *v1.ListJobsResponse, err error) {
	result, total, err := srv.Jobs.Find(ctx, req.Filter, int(req.Start), int(req.Limit))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	res := make([]*v1.JobStatus, len(result))
	for i := range result {
		res[i] = &result[i]
	}

	return &v1.ListJobsResponse{
		Total:  int32(total),
		Result: res,
	}, nil
}

// Listen listens to logs
func (srv *Service) Listen(req *v1.ListenRequest, ls v1.KeelService_ListenServer) error {

	return status.Error(codes.Unimplemented, "not implemented")
}

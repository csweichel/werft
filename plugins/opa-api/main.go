package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	plugin "github.com/csweichel/werft/pkg/plugin/client"
	"github.com/golang/protobuf/proto"
	"github.com/open-policy-agent/opa/rego"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Config configures this plugin
type Config struct {
	Port   int    `yaml:"port"`
	Policy string `yaml:"policy"`

	Dump bool `yaml:"dumpInput"`
}

func main() {
	plugin.Serve(&Config{},
		plugin.WithIntegrationPlugin(&authenticatedAPIPlugin{}),
	)
	fmt.Fprintln(os.Stderr, "shutting down")
}

type authenticatedAPIPlugin struct{}

func (authenticatedAPIPlugin) Run(ctx context.Context, config interface{}, services *plugin.Services) error {
	cfg := config.(*Config)
	policy, err := rego.New(
		rego.Query("data.werft.allow"),
		rego.Module("policy.rego", cfg.Policy),
	).PrepareForEval(ctx)
	if err != nil {
		return fmt.Errorf("cannot compile policy: %w", err)
	}

	srv := grpc.NewServer()
	v1.RegisterWerftServiceServer(srv, &proxyingService{
		Config:   cfg,
		Policy:   policy,
		Delegate: services.WerftServiceClient,
	})

	errchan := make(chan error, 1)
	go func() {
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
		if err != nil {
			errchan <- err
			return
		}
		errchan <- srv.Serve(l)
	}()
	defer srv.GracefulStop()

	select {
	case err := <-errchan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type proxyingService struct {
	Config   *Config
	Policy   rego.PreparedEvalQuery
	Delegate v1.WerftServiceClient
}

type policyInput struct {
	Method   string        `json:"method"`
	Metadata metadata.MD   `json:"metadata"`
	Message  proto.Message `json:"message"`
}

func (s *proxyingService) allowed(ctx context.Context, method string, msg proto.Message) error {
	md, _ := metadata.FromIncomingContext(ctx)
	input := policyInput{
		Method:   method,
		Metadata: md,
		Message:  msg,
	}

	result, err := s.Policy.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		logrus.WithError(err).Error("cannot evaluate policy")
		return status.Error(codes.Internal, "cannot evaluate policy")
	}
	if len(result) == 0 {
		logrus.WithError(err).Error("policy does not define data.werft.allow query")
		return status.Error(codes.Internal, "invalid policy")
	}
	if s.Config.Dump {
		dmp, _ := json.Marshal(input)
		logrus.WithField("input", string(dmp)).WithField("value", result[0].Expressions[0].Value).Debug("evaluating request")
	}

	if !result.Allowed() {
		return status.Error(codes.Unauthenticated, "not allowed")
	}

	return nil
}

func (s *proxyingService) StartLocalJob(v1.WerftService_StartLocalJobServer) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}

// StartGitHubJob starts a job on a Git context, possibly with a custom job.
func (s *proxyingService) StartGitHubJob(ctx context.Context, in *v1.StartGitHubJobRequest) (*v1.StartJobResponse, error) {
	if err := s.allowed(ctx, "StartGitHubJob", in); err != nil {
		return nil, err
	}
	return s.Delegate.StartGitHubJob(ctx, in)
}

// StartFromPreviousJob starts a new job based on a previous one.
// If the previous job does not have the can-replay condition set this call will result in an error.
func (s *proxyingService) StartFromPreviousJob(ctx context.Context, in *v1.StartFromPreviousJobRequest) (*v1.StartJobResponse, error) {
	if err := s.allowed(ctx, "StartFromPreviousJob", in); err != nil {
		return nil, err
	}
	return s.Delegate.StartFromPreviousJob(ctx, in)
}

// StartJobRequest starts a new job based on its specification.
func (s *proxyingService) StartJob(ctx context.Context, in *v1.StartJobRequest) (*v1.StartJobResponse, error) {
	if err := s.allowed(ctx, "StartJob", in); err != nil {
		return nil, err
	}
	return s.Delegate.StartJob(ctx, in)
}

// StartJob2 starts a new job based on its specification.
func (s *proxyingService) StartJob2(ctx context.Context, in *v1.StartJobRequest2) (*v1.StartJobResponse, error) {
	if err := s.allowed(ctx, "StartJob2", in); err != nil {
		return nil, err
	}
	return s.Delegate.StartJob2(ctx, in)
}

// Searches for jobs known to this instance
func (s *proxyingService) ListJobs(ctx context.Context, in *v1.ListJobsRequest) (*v1.ListJobsResponse, error) {
	if err := s.allowed(ctx, "ListJobs", in); err != nil {
		return nil, err
	}
	return s.Delegate.ListJobs(ctx, in)
}

// Subscribe listens to new jobs/job updates
func (s *proxyingService) Subscribe(in *v1.SubscribeRequest, srv v1.WerftService_SubscribeServer) error {
	if err := s.allowed(srv.Context(), "Listen", in); err != nil {
		return err
	}
	client, err := s.Delegate.Subscribe(srv.Context(), in)
	if err != nil {
		return err
	}

	for {
		msg, err := client.Recv()
		if err != nil {
			return err
		}
		if msg == nil {
			return nil
		}
		err = srv.Send(msg)
		if err != nil {
			return err
		}
	}
}

// GetJob retrieves details of a single job
func (s *proxyingService) GetJob(ctx context.Context, in *v1.GetJobRequest) (*v1.GetJobResponse, error) {
	if err := s.allowed(ctx, "GetJob", in); err != nil {
		return nil, err
	}
	return s.Delegate.GetJob(ctx, in)
}

// Listen listens to job updates and log output of a running job
func (s *proxyingService) Listen(in *v1.ListenRequest, srv v1.WerftService_ListenServer) error {
	if err := s.allowed(srv.Context(), "Listen", in); err != nil {
		return err
	}
	client, err := s.Delegate.Listen(srv.Context(), in)
	if err != nil {
		return err
	}

	for {
		msg, err := client.Recv()
		if err != nil {
			return err
		}
		if msg == nil {
			return nil
		}
		err = srv.Send(msg)
		if err != nil {
			return err
		}
	}
}

// StopJob stops a currently running job
func (s *proxyingService) StopJob(ctx context.Context, in *v1.StopJobRequest) (*v1.StopJobResponse, error) {
	if err := s.allowed(ctx, "StopJob", in); err != nil {
		return nil, err
	}
	return s.Delegate.StopJob(ctx, in)
}

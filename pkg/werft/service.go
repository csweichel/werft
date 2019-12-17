package werft

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/32leaves/werft/pkg/logcutter"
	"github.com/32leaves/werft/pkg/store"
	termtohtml "github.com/buildkite/terminal-to-html"
	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
	"github.com/technosophos/moniker"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StartLocalJob starts a job whoose content is uploaded
func (srv *Service) StartLocalJob(inc v1.WerftService_StartLocalJobServer) error {
	req, err := inc.Recv()
	if err != nil {
		return err
	}
	if req.GetMetadata() == nil {
		return status.Error(codes.InvalidArgument, "first request must contain metadata")
	}
	md := *req.GetMetadata()
	log.WithField("name", md).Debug("StartLocalJob - received metadata")

	dfs, err := ioutil.TempFile(os.TempDir(), "werft-lcp")
	if err != nil {
		return err
	}
	defer dfs.Close()
	defer os.Remove(dfs.Name())

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
			if phase == phaseJobYaml {
				phase = phaseWorkspaceTar
			}
			if phase != phaseWorkspaceTar {
				return status.Error(codes.InvalidArgument, "expected workspace tar")
			}

			data := req.GetWorkspaceTar()
			n, err := dfs.Write(data)
			if err != nil {
				return status.Error(codes.Internal, err.Error())
			}
			if n != len(data) {
				return status.Error(codes.Internal, io.ErrShortWrite.Error())
			}
		}
		if req.GetWorkspaceTarDone() {
			if phase != phaseWorkspaceTar {
				return status.Error(codes.InvalidArgument, "expected prior workspace tar")
			}

			break
		}
	}
	// reset the position in the file - important: otherwise the re-upload to the container fails
	_, err = dfs.Seek(0, 0)

	if len(configYAML) == 0 {
		return status.Error(codes.InvalidArgument, "config YAML must not be empty")
	}
	if len(jobYAML) == 0 {
		return status.Error(codes.InvalidArgument, "job YAML must not be empty")
	}

	cp := &LocalContentProvider{
		TarStream:  dfs,
		Namespace:  srv.Executor.Config.Namespace,
		Kubeconfig: srv.Executor.KubeConfig,
		Clientset:  srv.Executor.Client,
	}

	flatOwner := strings.ReplaceAll(strings.ToLower(md.Owner), " ", "")
	name := fmt.Sprintf("local-%s-%s", flatOwner, moniker.New().NameSep("-"))
	jobStatus, err := srv.RunJob(inc.Context(), name, md, cp, jobYAML)

	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	log.WithField("status", jobStatus).Info(("started new local job"))
	return inc.SendAndClose(&v1.StartJobResponse{
		Status: jobStatus,
	})
}

// StartGitHubJob starts a job on a Git context, possibly with a custom job.
func (srv *Service) StartGitHubJob(ctx context.Context, req *v1.StartGitHubJobRequest) (resp *v1.StartJobResponse, err error) {
	ghclient := srv.GitHub.Client
	if req.Token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: req.Token},
		)
		tc := oauth2.NewClient(ctx, ts)
		ghclient = github.NewClient(tc)
	}

	md := req.Metadata
	if md.Repository.Revision == "" && md.Repository.Ref != "" {
		md.Repository.Revision, _, err = ghclient.Repositories.GetCommitSHA1(ctx, md.Repository.Owner, md.Repository.Repo, md.Repository.Ref, "")
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	var cp = &GitHubContentProvider{
		Owner:    md.Repository.Owner,
		Repo:     md.Repository.Repo,
		Revision: md.Repository.Revision,
		Token:    req.Token,
		Client:   ghclient,
	}

	jobYAML := req.GetJobYaml()
	jobSpecName := "custom"
	if jobYAML == nil {
		jobSpecName = req.GetJobName()
		tplpath := fmt.Sprintf(".werft/%s.yaml", jobSpecName)
		if jobSpecName == "" {
			repoCfg, err := getRepoCfg(ctx, cp)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			tplpath = repoCfg.TemplatePath(req.Metadata.Trigger)
			jobSpecName = strings.TrimSuffix(filepath.Base(tplpath), filepath.Ext(tplpath))
		}
		in, err := cp.Download(ctx, tplpath)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		jobYAML, err = ioutil.ReadAll(in)
		in.Close()
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// acquire job number
	name := strings.ToLower(strings.ReplaceAll(md.Repository.Ref, "/", "-"))
	if name == "" {
		// we did not compute a sensible flatname - use moniker
		name = moniker.New().NameSep("-")
	} else {
		// we have a flatname but must use the number group
		name = fmt.Sprintf("%s-%s-%s", md.Repository.Repo, jobSpecName, name)
		t, err := srv.Groups.Next(name)
		if err != nil {
			srv.OnError(err)
		}

		name = fmt.Sprintf("%s.%d", name, t)
	}

	jobStatus, err := srv.RunJob(ctx, name, *md, cp, jobYAML)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.WithField("status", jobStatus).Info(("started new local job"))
	return &v1.StartJobResponse{
		Status: jobStatus,
	}, nil
}

// newTarStreamAdapter creates a reader from an incoming workspace tar stream
func newTarStreamAdapter(inc v1.WerftService_StartLocalJobServer, initial []byte) io.Reader {
	return &tarStreamAdapter{
		inc:       inc,
		remainder: initial,
	}
}

// tarStreamAdapter turns a client-side data stream into an io.Reader
type tarStreamAdapter struct {
	inc       v1.WerftService_StartLocalJobServer
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
	result, total, err := srv.Jobs.Find(ctx, req.Filter, req.Order, int(req.Start), int(req.Limit))
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

// Subscribe listens to job updates
func (srv *Service) Subscribe(req *v1.SubscribeRequest, resp v1.WerftService_SubscribeServer) (err error) {
	evts := srv.events.On("job")
	for evt := range evts {
		job := evt.Args[0].(*v1.JobStatus)
		if !store.MatchesFilter(job.Metadata, req.Filter) {
			continue
		}

		resp.Send(&v1.SubscribeResponse{
			Result: job,
		})
	}
	return nil
}

// GetJob returns the information about a particular job
func (srv *Service) GetJob(ctx context.Context, req *v1.GetJobRequest) (resp *v1.GetJobResponse, err error) {
	job, err := srv.Jobs.Get(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if job == nil {
		return nil, status.Error(codes.NotFound, "not found")
	}

	return &v1.GetJobResponse{
		Result: job,
	}, nil
}

// Listen listens to logs
func (srv *Service) Listen(req *v1.ListenRequest, ls v1.WerftService_ListenServer) error {
	// TOOD: if one of the listeners fails, all have to fail

	var (
		wg      sync.WaitGroup
		errchan = make(chan error, 2)
	)
	if req.Logs != v1.ListenRequestLogs_LOGS_DISABLED {
		wg.Add(1)

		rd, err := srv.Logs.Read(ls.Context(), req.Name)
		if err != nil {
			if err == store.ErrNotFound {
				return status.Error(codes.NotFound, "not found")
			}

			return status.Error(codes.Internal, err.Error())
		}

		go func() {
			defer wg.Done()

			cutter := logcutter.DefaultCutter
			if req.Logs == v1.ListenRequestLogs_LOGS_UNSLICED {
				cutter = logcutter.NoCutter
			}

			evts, echan := cutter.Slice(rd)
			for {
				select {
				case evt := <-evts:
					if evt == nil {
						log.Debug("logs finished")
						return
					}
					if req.Logs == v1.ListenRequestLogs_LOGS_HTML {
						evt.Payload = string(termtohtml.Render([]byte(evt.Payload)))
					}

					err = ls.Send(&v1.ListenResponse{
						Content: &v1.ListenResponse_Slice{
							Slice: evt,
						},
					})
				case err = <-echan:
					if err == nil {
						return
					}

					errchan <- status.Error(codes.Internal, err.Error())
					return
				case <-ls.Context().Done():
					errchan <- status.Error(codes.Aborted, ls.Context().Err().Error())
					return
				}
			}
		}()
	}

	if req.Updates {
		wg.Add(1)

		go func() {
			defer wg.Done()

			evts := srv.events.On("job")
			for evt := range evts {
				if len(evt.Args) == 0 {
					return
				}
				job, ok := evt.Args[0].(*v1.JobStatus)
				if !ok {
					continue
				}
				if job.Name != req.Name {
					continue
				}

				ls.Send(&v1.ListenResponse{
					Content: &v1.ListenResponse_Update{
						Update: job,
					},
				})
			}
		}()
	}

	select {
	case err := <-errchan:
		return err
	default:
	}
	wg.Wait()
	return nil
}

// StopJob stops a running job
func (srv *Service) StopJob(ctx context.Context, req *v1.StopJobRequest) (*v1.StopJobResponse, error) {
	job, err := srv.Jobs.Get(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if job == nil {
		return nil, status.Error(codes.NotFound, "not found")
	}

	if job.Phase != v1.JobPhase_PHASE_PREPARING && job.Phase != v1.JobPhase_PHASE_STARTING && job.Phase != v1.JobPhase_PHASE_RUNNING {
		return nil, status.Error(codes.FailedPrecondition, "job is unstoppable phase")
	}

	err = srv.Executor.Stop(req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.StopJobResponse{}, nil
}

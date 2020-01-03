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
	"github.com/32leaves/werft/pkg/filterexpr"
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

	if len(configYAML) == 0 && len(jobYAML) == 0 {
		return status.Error(codes.InvalidArgument, "either config or job YAML must not be empty")
	}

	cp := &LocalContentProvider{
		TarStream:  dfs,
		Namespace:  srv.Executor.Config.Namespace,
		Kubeconfig: srv.Executor.KubeConfig,
		Clientset:  srv.Executor.Client,
	}

	// Note: for local jobs we DO NOT store the job yaml as we cannot replay those jobs anyways.
	//       The context upload is a one time thing and hence prevent job replay.

	flatOwner := strings.ReplaceAll(strings.ToLower(md.Owner), " ", "")
	name := fmt.Sprintf("local-%s-%s", flatOwner, moniker.New().NameSep("-"))
	jobStatus, err := srv.RunJob(inc.Context(), name, md, cp, jobYAML, false)

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
	var (
		ghclient = srv.GitHub.Client
		gitauth  = srv.GitHub.Auth
	)
	if req.GithubToken != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: req.GithubToken},
		)
		tc := oauth2.NewClient(ctx, ts)
		ghclient = github.NewClient(tc)
		gitauth = fixedOAuthTokenGitCreds(req.GithubToken)
	}

	md := req.Metadata
	if md.Repository.Revision == "" && md.Repository.Ref != "" {
		md.Repository.Revision, _, err = ghclient.Repositories.GetCommitSHA1(ctx, md.Repository.Owner, md.Repository.Repo, md.Repository.Ref, "")
		if err != nil {
			return nil, translateGitHubToGRPCError(err, md.Repository.Revision, md.Repository.Ref)
		}
	}

	_, _, err = ghclient.Repositories.GetCommit(ctx, md.Repository.Owner, md.Repository.Repo, md.Repository.Revision)
	if err != nil {
		return nil, translateGitHubToGRPCError(err, md.Repository.Revision, md.Repository.Ref)
	}

	var cp = &GitHubContentProvider{
		Owner:    md.Repository.Owner,
		Repo:     md.Repository.Repo,
		Revision: md.Repository.Revision,
		Client:   ghclient,
		Auth:     gitauth,
	}

	var (
		jobYAML = req.JobYaml
		tplpath = req.JobPath
		jobSpecName = "custom"
	)
	if jobYAML == nil {
		if tplpath == "" {
			repoCfg, err := getRepoCfg(ctx, cp)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			tplpath = repoCfg.TemplatePath(req.Metadata)
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
	if tplpath != "" {
		jobSpecName = strings.TrimSuffix(filepath.Base(tplpath), filepath.Ext(tplpath))
	}

	// build job name
	refname := strings.ToLower(strings.ReplaceAll(md.Repository.Ref, "/", "-"))
	if refname == "" {
		// we did not compute a sensible refname - use moniker
		refname = moniker.New().NameSep("-")
	}
	name := fmt.Sprintf("%s-%s-%s", md.Repository.Repo, jobSpecName, refname)
	if refname != "" {
		// we have a valid refname, hence need to acquire job number
		t, err := srv.Groups.Next(name)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		name = fmt.Sprintf("%s.%d", name, t)
	}

	// We do not store the GitHub token of the request and hence can only restart those with default auth
	canReplay := req.GithubToken == ""

	jobStatus, err := srv.RunJob(ctx, name, *md, cp, jobYAML, canReplay)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.WithField("status", jobStatus).Info(("started new GitHub job"))
	return &v1.StartJobResponse{
		Status: jobStatus,
	}, nil
}

func translateGitHubToGRPCError(err error, rev, ref string) error {
	if gherr, ok := err.(*github.ErrorResponse); ok && gherr.Response.StatusCode == 422 {
		msg := fmt.Sprintf("revision %s", rev)
		if ref != "" {
			msg = fmt.Sprintf("ref %s (revision %s)", ref, rev)
		}
		return status.Error(codes.NotFound, fmt.Sprintf("%s not found", msg))
	}

	return status.Error(codes.Internal, err.Error())
}

// StartFromPreviousJob starts a new job based on an old one
func (srv *Service) StartFromPreviousJob(ctx context.Context, req *v1.StartFromPreviousJobRequest) (*v1.StartJobResponse, error) {
	oldJobStatus, err := srv.Jobs.Get(ctx, req.PreviousJob)
	if err == store.ErrNotFound {
		return nil, status.Error(codes.NotFound, "job spec not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	jobYAML, err := srv.Jobs.GetJobSpec(req.PreviousJob)
	if err == store.ErrNotFound {
		return nil, status.Error(codes.NotFound, "job spec not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	name := req.PreviousJob
	if strings.Contains(name, ".") {
		segs := strings.Split(name, ".")
		name = strings.Join(segs[0:len(segs)-1], ".")
	}
	nr, err := srv.Groups.Next(name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	name = fmt.Sprintf("%s.%d", name, nr)

	gitauth := srv.GitHub.Auth
	if req.GithubToken != "" {
		gitauth = fixedOAuthTokenGitCreds(req.GithubToken)
	}

	md := oldJobStatus.Metadata
	cp := &GitHubContentProvider{
		Owner:    md.Repository.Owner,
		Repo:     md.Repository.Repo,
		Revision: md.Repository.Revision,
		Client:   srv.GitHub.Client,
		Auth:     gitauth,
	}

	// We do not store the GitHub token of the request and hence can only restart those with default auth
	canReplay := req.GithubToken == ""

	jobStatus, err := srv.RunJob(ctx, name, *oldJobStatus.Metadata, cp, jobYAML, canReplay)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.WithField("name", req.PreviousJob).WithField("old-name", name).Info(("started new job from an old one"))
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
		if !filterexpr.MatchesFilter(job, req.Filter) {
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

		rd, err := srv.Logs.Read(req.Name)
		if err != nil {
			if err == store.ErrNotFound {
				return status.Error(codes.NotFound, "not found")
			}

			return status.Error(codes.Internal, err.Error())
		}
		defer rd.Close()

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

	err = srv.Executor.Stop(req.Name, "job was stopped manually")
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.StopJobResponse{}, nil
}

func fixedOAuthTokenGitCreds(tkn string) GitCredentialHelper {
	return func(ctx context.Context) (user string, pass string, err error) {
		return tkn, "x-oauth-basic", nil
	}
}

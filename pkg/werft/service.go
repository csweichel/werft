package werft

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	termtohtml "github.com/buildkite/terminal-to-html"
	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/csweichel/werft/pkg/filterexpr"
	"github.com/csweichel/werft/pkg/logcutter"
	"github.com/csweichel/werft/pkg/store"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/go-github/v31/github"
	log "github.com/sirupsen/logrus"
	"github.com/technosophos/moniker"
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

	err = srv.addRemoteAnnotations(inc.Context(), &md)
	if err != nil {
		log.WithError(err).Warn("cannot obtain remote annotations")
	}

	// Note: for local jobs we DO NOT store the job yaml as we cannot replay those jobs anyways.
	//       The context upload is a one time thing and hence prevent job replay.

	flatOwner := strings.ReplaceAll(strings.ToLower(md.Owner), " ", "")
	name := cleanupPodName(fmt.Sprintf("local-%s-%s", flatOwner, moniker.New().NameSep("-")))

	jobStatus, err := srv.RunJob(inc.Context(), name, md, cp, jobYAML, false, time.Time{})

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
	if req.GithubToken != "" {
		return nil, status.Errorf(codes.InvalidArgument, "Per-job GitHub tokens are no longer supported")
	}

	if req.Metadata.Repository.Host == "" {
		req.Metadata.Repository.Host = defaultGitHubHost
	}

	return srv.StartJob(ctx, &v1.StartJobRequest{
		JobPath:   req.JobPath,
		JobYaml:   req.JobYaml,
		Metadata:  req.Metadata,
		Sideload:  req.Sideload,
		WaitUntil: req.WaitUntil,
	})
}

// StartJob starts a new job based on its specification.
func (srv *Service) StartJob(ctx context.Context, req *v1.StartJobRequest) (resp *v1.StartJobResponse, err error) {
	var (
		repoHost = req.Metadata.Repository.Host
		repo, ok = srv.RepoProvider[repoHost]
	)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported repo host: %v", repoHost)
	}

	md := req.Metadata
	err = repo.Resolve(md.Repository)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cannot resolve request: %q", err)
	}

	err = srv.addRemoteAnnotations(ctx, md)
	if err != nil {
		return nil, err
	}

	var cp ContentProvider
	cp, err = repo.ContentProvider(md.Repository)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cannot produce content provider: %q", err)
	}

	if len(req.Sideload) > 0 {
		cp = &SideloadingContentProvider{
			Delegate: cp,

			TarStream:  bytes.NewReader(req.Sideload),
			Namespace:  srv.Executor.Config.Namespace,
			Kubeconfig: srv.Executor.KubeConfig,
			Clientset:  srv.Executor.Client,
		}
	}

	var fp FileProvider
	fp, err = repo.FileProvider(md.Repository)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cannot produce file provider: %q", err)
	}

	var (
		jobYAML     = req.JobYaml
		tplpath     = req.JobPath
		jobSpecName = "custom"
	)
	if jobYAML == nil {
		if tplpath == "" {
			repoCfg, err := getRepoCfg(ctx, fp)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			tplpath = repoCfg.TemplatePath(req.Metadata)
		}

		in, err := fp.Download(ctx, tplpath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "cannot download jobspec from %s: %s", tplpath, err.Error())
		}
		jobYAML, err = ioutil.ReadAll(in)
		in.Close()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "cannot download jobspec from %s: %s", tplpath, err.Error())
		}
	}
	if tplpath != "" {
		jobSpecName = strings.TrimSuffix(filepath.Base(tplpath), filepath.Ext(tplpath))
	}

	// build job name
	refname := md.Repository.Ref
	refname = strings.TrimPrefix(refname, "refs/heads/")
	refname = strings.TrimPrefix(refname, "refs/tags/")
	refname = strings.ReplaceAll(refname, "/", "-")
	refname = strings.ReplaceAll(refname, "_", "-")
	refname = strings.ReplaceAll(refname, "@", "-")
	refname = strings.ToLower(refname)
	if refname == "" {
		// we did not compute a sensible refname - use moniker
		refname = moniker.New().NameSep("-")
	}
	name := cleanupPodName(fmt.Sprintf("%s-%s-%s", md.Repository.Repo, jobSpecName, refname))

	if refname != "" {
		// we have a valid refname, hence need to acquire job number
		t, err := srv.Groups.Next(name)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		name = fmt.Sprintf("%s.%d", name, t)
	}

	canReplay := len(req.Sideload) == 0

	var waitUntil time.Time
	if req.WaitUntil != nil {
		waitUntil, err = ptypes.Timestamp(req.WaitUntil)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "waitUntil is invalid: %v", err)
		}

		if !canReplay {
			return nil, status.Error(codes.InvalidArgument, "cannot delay the execution of non-replayable jobs (i.e. jobs with custom GitHub token or sideload)")
		}
	}

	jobStatus, err := srv.RunJob(ctx, name, *md, cp, jobYAML, canReplay, waitUntil)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.WithField("status", jobStatus).Info(("started new GitHub job"))
	return &v1.StartJobResponse{
		Status: jobStatus,
	}, nil
}

func (srv *Service) addRemoteAnnotations(ctx context.Context, md *v1.JobMetadata) error {
	ghclient := srv.GitHub.Client

	commit, _, err := ghclient.Repositories.GetCommit(ctx, md.Repository.Owner, md.Repository.Repo, md.Repository.Revision)
	if err != nil {
		return translateGitHubToGRPCError(err, md.Repository.Revision, md.Repository.Ref)
	}
	if commit.Commit != nil {
		atns := parseAnnotations(commit.Commit.GetMessage())
		for k, v := range atns {
			md.Annotations = append(md.Annotations, &v1.Annotation{
				Key:   k,
				Value: v,
			})
		}
	}

	prs, _, err := ghclient.PullRequests.ListPullRequestsWithCommit(ctx, md.Repository.Owner, md.Repository.Repo, commit.GetSHA(), &github.PullRequestListOptions{
		State: "open",
		Sort:  "created",
	})
	if err != nil {
		log.WithField("ref", md.Repository.Ref).WithField("rev", md.Repository.Revision).WithError(err).Warn("cannot search for associated PR's")
	} else if len(prs) >= 1 {
		sort.Slice(prs, func(i, j int) bool { return prs[i].GetCreatedAt().Before(prs[j].GetCreatedAt()) })
		pr := prs[0]

		if len(prs) > 1 {
			log.WithField("ref", md.Repository.Ref).WithField("rev", md.Repository.Revision).WithField("pr", pr.GetHTMLURL()).Warn("found multiple open PR's - using oldest one")
		}

		atns := parseAnnotations(pr.GetBody())
		for k, v := range atns {
			md.Annotations = append(md.Annotations, &v1.Annotation{
				Key:   k,
				Value: v,
			})
		}
	}
	return nil
}

// pareseAnnotations parses one annotation per line in the form of "/werft <key>(=<value>)?".
// Any line not matching this format is silently ignored.
func parseAnnotations(message string) (res map[string]string) {
	scanner := bufio.NewScanner(bytes.NewReader([]byte(message)))
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- [x]")
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "/werft ") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "/werft "))

		var name, val string
		if idx := strings.Index(line, "="); idx > -1 {
			name = line[:idx]
			val = strings.TrimPrefix(line[idx:], "=")
		} else {
			name = line
			val = ""
		}
		if res == nil {
			res = make(map[string]string)
		}
		res[name] = val
	}
	return res
}

func cleanupPodName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) == 0 {
		name = "unknown"
	}
	if len(name) > 58 {
		// Kubernetes label values must not be longer than 63 characters according to
		// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
		// We need to leave some space for the build number. Assuming that we won't have more than 9999 builds on
		// a single long named branch, leaving four chars should be enough.
		name = name[:58]
	}

	// attempt to fix the name segment wise
	segs := strings.Split(name, ".")
	for i, n := range segs {
		s := strings.ToLower(n)[0]
		if !(('a' <= s && s <= 'z') || ('0' <= s && s <= '9')) {
			n = "a" + n[1:]
		}
		e := strings.ToLower(n)[len(n)-1]
		if !(('a' <= e && e <= 'z') || ('0' <= e && e <= '9')) {
			n = n[:len(n)-1] + "a"
		}

		segs[i] = n
	}
	name = strings.Join(segs, ".")

	return name
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
	md.Finished = nil
	cp := &GitHubContentProvider{
		Owner:    md.Repository.Owner,
		Repo:     md.Repository.Repo,
		Revision: md.Repository.Revision,
		Client:   srv.GitHub.Client,
		Auth:     gitauth,
	}

	// We do not store the GitHub token of the request and hence can only restart those with default auth
	canReplay := req.GithubToken == ""

	var waitUntil time.Time
	if req.WaitUntil != nil {
		waitUntil, err = ptypes.Timestamp(req.WaitUntil)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "waitUntil is invalid: %v", err)
		}

		if !canReplay {
			return nil, status.Error(codes.InvalidArgument, "cannot delay the execution of non-replayable jobs (i.e. jobs with custom GitHub)")
		}
	}

	jobStatus, err := srv.RunJob(ctx, name, *oldJobStatus.Metadata, cp, jobYAML, canReplay, waitUntil)
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
	job, err := srv.Jobs.Get(ls.Context(), req.Name)
	if err == store.ErrNotFound {
		return status.Errorf(codes.NotFound, "%s not found", req.Name)
	}

	var (
		wg      sync.WaitGroup
		logwg   sync.WaitGroup
		errchan = make(chan error)
	)
	if req.Logs != v1.ListenRequestLogs_LOGS_DISABLED {
		wg.Add(1)
		logwg.Add(1)

		rd, err := srv.Logs.Read(req.Name)
		if err != nil {
			if err == store.ErrNotFound {
				return status.Error(codes.NotFound, "not found")
			}

			return status.Error(codes.Internal, err.Error())
		}

		go func() {
			defer rd.Close()
			defer wg.Done()
			defer logwg.Done()

			cutter := logcutter.DefaultCutter
			if req.Logs == v1.ListenRequestLogs_LOGS_UNSLICED {
				cutter = logcutter.NoCutter
			}

			evts, echan := cutter.Slice(rd)
			for {
				select {
				case evt := <-evts:
					if evt == nil {
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

			if job.Phase == v1.JobPhase_PHASE_DONE {
				// The job we're listening on is already done. To provide the same behaviour as if the job were still running,
				// we first have to dump out all the logs and then send the one final status update.
				logwg.Wait()

				ls.Send(&v1.ListenResponse{
					Content: &v1.ListenResponse_Update{
						Update: job,
					},
				})
				return
			}

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

	go func() {
		wg.Wait()
		errchan <- nil
	}()

	err = <-errchan
	return err
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

	if job.Phase != v1.JobPhase_PHASE_WAITING && job.Phase != v1.JobPhase_PHASE_PREPARING && job.Phase != v1.JobPhase_PHASE_STARTING && job.Phase != v1.JobPhase_PHASE_RUNNING {
		return nil, status.Error(codes.FailedPrecondition, "job is in unstoppable phase")
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

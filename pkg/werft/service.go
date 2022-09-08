package werft

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	termtohtml "github.com/buildkite/terminal-to-html"
	"github.com/csweichel/werft/pkg/api/repoconfig"
	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/csweichel/werft/pkg/filterexpr"
	"github.com/csweichel/werft/pkg/logcutter"
	"github.com/csweichel/werft/pkg/store"
	"github.com/gogo/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"github.com/technosophos/moniker"
	"golang.org/x/xerrors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
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
	name := cleanupPodName(fmt.Sprintf("local-%s-%s", flatOwner, moniker.New().NameSep("-")))

	jobStatus, err := srv.RunJob(inc.Context(), name, md, v1.JobSpec{}, cp, jobYAML, false)

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
		req.Metadata.Repository.Host = "github.com"
	}

	spec := &v1.JobSpec{
		DirectSideload: req.Sideload,
		NameSuffix:     req.NameSuffix,
	}
	if req.JobYaml != nil {
		spec.Source = &v1.JobSpec_JobYaml{JobYaml: req.JobYaml}
	} else {
		spec.Source = &v1.JobSpec_JobPath{JobPath: req.JobPath}
	}

	return srv.StartJob2(ctx, &v1.StartJobRequest2{
		Metadata: req.Metadata,
		Spec:     spec,
	})
}

func (srv *Service) StartJob2(ctx context.Context, req *v1.StartJobRequest2) (resp *v1.StartJobResponse, err error) {
	log.WithField("req", proto.MarshalTextString(req)).Info("StartJob request")

	md := req.Metadata
	if md.Trigger == v1.JobTrigger_TRIGGER_DELETED {
		// Note: attempting to resolve the reference after it's been deleted is
		//       guaranteed to result in an error. Hence we're only doing this
		//       if the trigger isn't DELETED.
	} else {
		err = srv.RepositoryProvider.Resolve(ctx, md.Repository)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "cannot resolve request: %q", err)
		}

		atns, err := srv.RepositoryProvider.RemoteAnnotations(ctx, md.Repository)
		if err != nil {
			return nil, err
		}
		for k, v := range atns {
			md.Annotations = append(md.Annotations, &v1.Annotation{
				Key:   k,
				Value: v,
			})
		}
	}

	cp, err := srv.getContentProvider(ctx, md, req.Spec)
	if err != nil {
		return nil, err
	}

	var (
		jobYAML     []byte
		jobPath     string
		jobRepo     *v1.Repository
		jobSpecName = "custom"
	)
	switch src := req.Spec.Source.(type) {
	case *v1.JobSpec_JobYaml:
		jobYAML = src.JobYaml
	case *v1.JobSpec_Repo:
		jobPath = src.Repo.Path
		jobRepo = src.Repo.Repo
	case *v1.JobSpec_JobPath:
		jobPath = src.JobPath
		jobRepo = req.Metadata.Repository
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown job source type")
	}
	if len(jobYAML) == 0 {
		var fp FileProvider
		fp, err = srv.RepositoryProvider.FileProvider(ctx, jobRepo)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "cannot produce file provider: %q", err)
		}

		if jobPath == "" {
			repoCfg, err := getRepoCfg(ctx, fp)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			jobPath = repoCfg.TemplatePath(req.Metadata)
		}

		in, err := fp.Download(ctx, jobPath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "cannot download jobspec from %s: %s", jobPath, err.Error())
		}
		jobYAML, err = ioutil.ReadAll(in)
		in.Close()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "cannot download jobspec from %s: %s", jobPath, err.Error())
		}

		if jobPath != "" {
			jobSpecName = strings.TrimSpace(strings.TrimSuffix(filepath.Base(jobPath), filepath.Ext(jobPath)))
		}
	}

	md.JobSpecName = jobSpecName

	// build job name
	refname := md.Repository.Ref
	refname = strings.TrimPrefix(refname, "refs/heads/")
	refname = strings.TrimPrefix(refname, "refs/tags/")
	refname = strings.ReplaceAll(refname, "/", "-")
	refname = strings.ReplaceAll(refname, "_", "-")
	refname = strings.ReplaceAll(refname, "@", "-")
	refname = strings.ReplaceAll(refname, "#", "-")
	refname = strings.ToLower(refname)
	if refname == "" {
		// we did not compute a sensible refname - use moniker
		refname = moniker.New().NameSep("-")
	}
	name := cleanupPodName(fmt.Sprintf("%s-%s-%s", md.Repository.Repo, jobSpecName, refname))
	if ns := req.Spec.NameSuffix; ns != "" {
		if len(ns) > 20 {
			return nil, status.Error(codes.InvalidArgument, "name suffix must be less than 20 characters")
		}

		name += "-" + ns
	}

	if refname != "" {
		// we have a valid refname, hence need to acquire job number
		t, err := srv.Groups.Next(name)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		name = fmt.Sprintf("%s.%d", name, t)
	}

	canReplay := true

	jobStatus, err := srv.RunJob(ctx, name, *md, *req.Spec, cp, jobYAML, canReplay)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.WithField("status", jobStatus).Info(("started new job"))
	return &v1.StartJobResponse{
		Status: jobStatus,
	}, nil
}

// getContentProvider produces a content provider for the given job spec
func (srv *Service) getContentProvider(ctx context.Context, md *v1.JobMetadata, spec *v1.JobSpec) (ContentProvider, error) {
	repo := *md.Repository
	if md.Trigger == v1.JobTrigger_TRIGGER_DELETED {
		// the ref/revision are pointless now, because the branch/tag was just deleted.
		// We'll check out the default branch instead.
		repo.Revision = ""
		repo.Ref = md.Repository.DefaultBranch
		if !strings.HasPrefix(repo.Ref, "refs/heads/") {
			repo.Ref = "refs/heads/" + repo.Ref
		}
	}

	var cp CompositeContentProvider
	rcp, err := srv.RepositoryProvider.ContentProvider(ctx, &repo)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cannot produce content provider: %q", err)
	}
	cp = append(cp, rcp)

	if sl := spec.DirectSideload; len(sl) > 0 {
		slc := &SideloadingContentProvider{
			TarStream:  bytes.NewReader(sl),
			Namespace:  srv.Executor.Config.Namespace,
			Kubeconfig: srv.Executor.KubeConfig,
			Clientset:  srv.Executor.Client,
		}
		cp = append(cp, slc)
	}

	if rl := spec.RepoSideload; len(rl) > 0 {
		for i, repo := range rl {
			if repo.Path == "" {
				return nil, status.Errorf(codes.InvalidArgument, "repo sideload %d has no path", i)
			}

			rcp, err := srv.RepositoryProvider.ContentProvider(ctx, repo.Repo, repo.Path)
			if err != nil {
				return nil, status.Errorf(codes.FailedPrecondition, "cannot produce content provider: %q", err)
			}

			cp = append(cp, rcp)
		}
	}

	return cp, nil
}

// StartJob starts a new job based on its specification.
func (srv *Service) StartJob(ctx context.Context, req *v1.StartJobRequest) (resp *v1.StartJobResponse, err error) {
	if req.WaitUntil != nil {
		return nil, status.Errorf(codes.InvalidArgument, "WaitUntil is no longer supported")
	}

	spec := &v1.JobSpec{
		DirectSideload: req.Sideload,
		NameSuffix:     req.NameSuffix,
	}
	if req.JobYaml != nil {
		spec.Source = &v1.JobSpec_JobYaml{JobYaml: req.JobYaml}
	} else {
		spec.Source = &v1.JobSpec_JobPath{JobPath: req.JobPath}
	}

	return srv.StartJob2(ctx, &v1.StartJobRequest2{
		Metadata: req.Metadata,
		Spec:     spec,
	})
}

func getRepoCfg(ctx context.Context, fp FileProvider) (*repoconfig.C, error) {
	// download werft config from branch
	werftYAML, err := fp.Download(ctx, PathWerftConfig)
	if err != nil {
		// TODO handle repos without werft config more gracefully
		return nil, xerrors.Errorf("cannot get repo config: %w", err)
	}
	var repoCfg repoconfig.C
	err = yaml.NewDecoder(werftYAML).Decode(&repoCfg)
	if err != nil {
		return nil, xerrors.Errorf("cannot unmarshal repo config: %w", err)
	}

	return &repoCfg, nil
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

// StartFromPreviousJob starts a new job based on an old one
func (srv *Service) StartFromPreviousJob(ctx context.Context, req *v1.StartFromPreviousJobRequest) (*v1.StartJobResponse, error) {
	oldJobStatus, err := srv.Jobs.Get(ctx, req.PreviousJob)
	if err == store.ErrNotFound {
		return nil, status.Error(codes.NotFound, "job spec not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	jobSpec, jobYAML, err := srv.Jobs.GetJobSpec(req.PreviousJob)
	if err == store.ErrNotFound {
		return nil, status.Error(codes.NotFound, "job spec not found")
	}
	if err != nil {
		log.WithError(err).WithField("req", req).Error("failed to start previous job")
		return nil, status.Error(codes.Internal, err.Error())
	}
	if jobSpec == nil {
		// this can happen for jobs which ran prior to the introduction
		// of job specs.
		return nil, status.Error(codes.FailedPrecondition, "job is too old and cannot be re-run")
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

	md := oldJobStatus.Metadata
	md.Finished = nil
	cp, err := srv.getContentProvider(ctx, md, jobSpec)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// We are replaying this job already - hence this new job is replayable as well
	canReplay := true

	jobStatus, err := srv.RunJob(ctx, name, *oldJobStatus.Metadata, *jobSpec, cp, jobYAML, canReplay)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.WithField("name", req.PreviousJob).WithField("old-name", name).Info(("started new job from an old one"))
	return &v1.StartJobResponse{
		Status: jobStatus,
	}, nil
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

package keel

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"path/filepath"

	v1 "github.com/32leaves/keel/pkg/api/v1"
	"github.com/32leaves/keel/pkg/executor"
	"github.com/32leaves/keel/pkg/logcutter"
	"github.com/32leaves/keel/pkg/store"
	rice "github.com/GeertJohan/go.rice"
	"github.com/Masterminds/sprig"
	"github.com/google/go-github/github"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/olebedev/emitter"
	log "github.com/sirupsen/logrus"
	"github.com/technosophos/moniker"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
)

// Service ties everything together
type Service struct {
	Logs     store.Logs
	Jobs     store.Jobs
	Executor *executor.Executor
	Cutter   logcutter.Cutter
	GitHub   GitHubSetup

	OnError                 func(err error)
	WorkspaceNodePathPrefix string

	events emitter.Emitter
}

// GitHubSetup sets up the access to GitHub
type GitHubSetup struct {
	WebhookSecret []byte
	Client        *github.Client
}

// Start sets up everything to run this keel instance, including executor config, server, etc.
func (srv *Service) Start(addr string) {
	if srv.OnError == nil {
		srv.OnError = func(err error) {
			log.WithError(err).Error("service error")
		}
	}

	// TOOD: on update change status in GitHub
	srv.Executor.OnUpdate = func(s *v1.JobStatus) {
		<-srv.events.Emit(fmt.Sprintf("job.%s", s.Name), s)
	}

	webuiServer := http.FileServer(rice.MustFindBox("../webui/build").HTTPBox())

	grpcServer := grpc.NewServer()
	v1.RegisterKeelServiceServer(grpcServer, srv)
	grpcWebServer := grpcweb.WrapServer(grpcServer)

	mux := http.NewServeMux()
	mux.HandleFunc("/github/app", srv.handleGithubWebhook)
	mux.Handle("/", hstsHandler(
		grpcTrafficSplitter(
			webuiServer,
			grpcWebServer,
		),
	))

	log.WithField("addr", addr).Info("serving keel service")
	err := http.ListenAndServe(addr, mux)
	if err != nil {
		srv.OnError(err)
	}
}

// hstsHandler wraps an http.HandlerFunc such that it sets the HSTS header.
func hstsHandler(fn http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		fn(w, r)
	})
}

func grpcTrafficSplitter(fallback http.Handler, wrappedGrpc *grpcweb.WrappedGrpcServer) http.HandlerFunc {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if wrappedGrpc.IsGrpcWebRequest(req) || wrappedGrpc.IsAcceptableGrpcCorsRequest(req) {
			wrappedGrpc.ServeHTTP(resp, req)
		} else {
			// Fall back to other servers.
			fallback.ServeHTTP(resp, req)
		}
	})
}

func (srv *Service) handleGithubWebhook(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func(err *error) {
		if *err == nil {
			return
		}

		srv.OnError(*err)
		http.Error(w, (*err).Error(), http.StatusInternalServerError)
	}(&err)

	payload, err := github.ValidatePayload(r, srv.GitHub.WebhookSecret)
	if err != nil {
		return
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		return
	}
	switch event := event.(type) {
	case *github.CommitCommentEvent:
		// processCommitCommentEvent(event)
	case *github.CreateEvent:
		// processCreateEvent(event)
	case *github.PushEvent:
		srv.processPushEvent(event)
	default:
		err = xerrors.Errorf("unhandled GitHub event: %+v", event)
	}
}

// RunJob starts a build job from some context
func (srv *Service) RunJob(ctx context.Context, jc JobContext, cp ContentProvider) (name string, err error) {
	// download keel config from branch
	keelYAML, err := cp.Download(ctx, ".keel/config.yaml")
	if err != nil {
		// TODO handle repos without keel config more gracefully
		return "", xerrors.Errorf("cannot handle job for %s: %w", jc.String(), err)
	}
	var repoCfg RepoConfig
	err = yaml.NewDecoder(keelYAML).Decode(&repoCfg)
	if err != nil {
		return "", xerrors.Errorf("cannot handle job for %s: %w", jc.String(), err)
	}

	// check if we need to build/do anything
	if !repoCfg.ShouldRun(JobTriggerPush) {
		return
	}

	// compile job podspec from template
	tplpth := repoCfg.TemplatePath(JobTriggerPush)
	jobTplYAML, err := cp.Download(ctx, tplpth)
	if err != nil {
		return "", xerrors.Errorf("cannot handle job for %s: %w", jc.String(), err)
	}
	jobTplRaw, err := ioutil.ReadAll(jobTplYAML)
	if err != nil {
		return "", xerrors.Errorf("cannot handle job for %s: %w", jc.String(), err)
	}
	jobTpl, err := template.New("job").Funcs(sprig.FuncMap()).Parse(string(jobTplRaw))
	if err != nil {
		return "", xerrors.Errorf("cannot handle job for %s: %w", jc.String(), err)
	}

	buf := bytes.NewBuffer(nil)
	err = jobTpl.Execute(buf, jc)
	if err != nil {
		return "", xerrors.Errorf("cannot handle job for %s: %w", jc.String(), err)
	}

	var podspec corev1.PodSpec
	err = yaml.Unmarshal(buf.Bytes(), &podspec)
	if err != nil {
		return "", xerrors.Errorf("cannot handle job for %s: %w", jc.String(), err)
	}

	name = fmt.Sprintf("keel-%s", moniker.New().NameSep("-"))
	nodePath := filepath.Join(srv.WorkspaceNodePathPrefix, name)
	httype := corev1.HostPathDirectoryOrCreate
	podspec.Volumes = append(podspec.Volumes, corev1.Volume{
		Name: "keel-workspace",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: nodePath,
				Type: &httype,
			},
		},
	})
	cpinit := cp.InitContainer()
	cpinit.Name = "keel-checkout"
	cpinit.ImagePullPolicy = corev1.PullIfNotPresent
	cpinit.VolumeMounts = append(cpinit.VolumeMounts, corev1.VolumeMount{
		Name:      "keel-workspace",
		ReadOnly:  false,
		MountPath: "/workspace",
	})
	podspec.InitContainers = append(podspec.InitContainers, cpinit)
	for i, c := range podspec.Containers {
		podspec.Containers[i].VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
			Name:      "keel-workspace",
			ReadOnly:  false,
			MountPath: "/workspace",
		})
	}

	// schedule/start job
	name, err = srv.Executor.Start(podspec, executor.WithAnnotations(map[string]string{
		"owner": jc.Owner,
		"repo":  jc.Repo,
		"rev":   jc.Revision,
	}), executor.WithName(name))
	if err != nil {
		return "", xerrors.Errorf("cannot handle job for %s: %w", jc.String(), err)
	}

	err = cp.Serve(name)
	if err != nil {
		return "", err
	}

	return name, nil
}

func (srv *Service) processPushEvent(event *github.PushEvent) {
	ctx := context.Background()
	jc := JobContext{
		Owner:    *event.Repo.Owner.Name,
		Repo:     *event.Repo.Name,
		Revision: *event.Ref,
		Trigger:  JobTriggerPush,
	}

	content := &GitHubContentProvider{
		Owner:    jc.Owner,
		Repo:     jc.Repo,
		Revision: jc.Revision,
	}
	_, err := srv.RunJob(ctx, jc, content)
	if err != nil {
		srv.OnError(err)
	}
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

// RepoConfig is the struct we expect to find in the repo root which configures how we build things
type RepoConfig struct {
	DefaultJob string `yaml:"defaultJob"`
}

// TemplatePath returns the path to the job template in the repo
func (rc *RepoConfig) TemplatePath(trigger JobTrigger) string {
	return rc.DefaultJob
}

// ShouldRun determines based on the repo config if the job should run
func (rc *RepoConfig) ShouldRun(trigger JobTrigger) bool {
	return true
}

package keel

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
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
	DebugProxy              string

	events emitter.Emitter
}

// GitHubSetup sets up the access to GitHub
type GitHubSetup struct {
	WebhookSecret []byte
	Client        *github.Client
}

// Start sets up everything to run this keel instance, including executor config
func (srv *Service) Start() {
	if srv.OnError == nil {
		srv.OnError = func(err error) {
			log.WithError(err).Error("service error")
		}
	}

	// TOOD: on update change status in GitHub
	srv.Executor.OnUpdate = func(s *v1.JobStatus) {
		log.WithField("status", s).Info("update")

		if s.Phase == v1.JobPhase_PHASE_CLEANUP {
			return
		}
		srv.Jobs.Store(context.Background(), *s)
		<-srv.events.Emit("job", s)
	}
}

// StartWeb starts the keel web UI service
func (srv *Service) StartWeb(addr string) {
	webuiServer := http.FileServer(rice.MustFindBox("../webui/build").HTTPBox())
	if srv.DebugProxy != "" {
		tgt, err := url.Parse(srv.DebugProxy)
		if err != nil {
			// this is debug only - it's ok to panic
			panic(err)
		}
		webuiServer = httputil.NewSingleHostReverseProxy(tgt)
	}

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

	log.WithField("addr", addr).Info("serving keel web service")
	err := http.ListenAndServe(addr, mux)
	if err != nil {
		srv.OnError(err)
	}
}

// StartGRPC starts the keel GRPC service
func (srv *Service) StartGRPC(addr string) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		srv.OnError(err)
	}

	grpcServer := grpc.NewServer()
	v1.RegisterKeelServiceServer(grpcServer, srv)

	log.WithField("addr", addr).Info("serving keel GRPC service")
	err = grpcServer.Serve(lis)
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
func (srv *Service) RunJob(ctx context.Context, metadata v1.JobMetadata, cp ContentProvider) (status *v1.JobStatus, err error) {
	name := fmt.Sprintf("keel-%s", moniker.New().NameSep("-"))

	// download keel config from branch
	keelYAML, err := cp.Download(ctx, PathKeelConfig)
	if err != nil {
		// TODO handle repos without keel config more gracefully
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}
	var repoCfg RepoConfig
	err = yaml.NewDecoder(keelYAML).Decode(&repoCfg)
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}

	// check if we need to build/do anything
	if !repoCfg.ShouldRun(metadata.Trigger) {
		return
	}

	// compile job podspec from template
	tplpth := repoCfg.TemplatePath(metadata.Trigger)
	jobTplYAML, err := cp.Download(ctx, tplpth)
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}
	jobTplRaw, err := ioutil.ReadAll(jobTplYAML)
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}
	jobTpl, err := template.New("job").Funcs(sprig.FuncMap()).Parse(string(jobTplRaw))
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}

	buf := bytes.NewBuffer(nil)
	err = jobTpl.Execute(buf, metadata)
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}

	var podspec corev1.PodSpec
	err = yaml.Unmarshal(buf.Bytes(), &podspec)
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}

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
	status, err = srv.Executor.Start(podspec, metadata, executor.WithName(name))
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}
	name = status.Name

	err = cp.Serve(name)
	if err != nil {
		return nil, err
	}

	return status, nil
}

func (srv *Service) processPushEvent(event *github.PushEvent) {
	ctx := context.Background()
	metadata := v1.JobMetadata{
		Owner: *event.Pusher.Name,
		Repository: &v1.Repository{
			Owner: *event.Repo.Owner.Name,
			Repo:  *event.Repo.Name,
			Ref:   *event.Ref,
		},
		Trigger: v1.JobTrigger_TRIGGER_PUSH,
	}

	content := &GitHubContentProvider{
		Owner:    metadata.Repository.Owner,
		Repo:     metadata.Repository.Repo,
		Revision: metadata.Repository.Ref,
	}
	_, err := srv.RunJob(ctx, metadata, content)
	if err != nil {
		srv.OnError(err)
	}
}

// RepoConfig is the struct we expect to find in the repo root which configures how we build things
type RepoConfig struct {
	DefaultJob string `yaml:"defaultJob"`
}

// TemplatePath returns the path to the job template in the repo
func (rc *RepoConfig) TemplatePath(trigger v1.JobTrigger) string {
	return rc.DefaultJob
}

// ShouldRun determines based on the repo config if the job should run
func (rc *RepoConfig) ShouldRun(trigger v1.JobTrigger) bool {
	return true
}

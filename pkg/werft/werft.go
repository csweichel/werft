package werft

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/32leaves/werft/pkg/api/repoconfig"
	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/32leaves/werft/pkg/executor"
	"github.com/32leaves/werft/pkg/logcutter"
	"github.com/32leaves/werft/pkg/store"
	"github.com/Masterminds/sprig"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/go-github/github"
	"github.com/olebedev/emitter"
	"github.com/segmentio/textio"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	k8syaml "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
)

// Config configures the behaviour of the service
type Config struct {
	// BaseURL is the URL this service is available on (e.g. https://werft.some-domain.com)
	BaseURL string `json:"baseURL,omitempty"`

	// WorkspaceNodePathPrefix is the location on the node where we place the builds
	WorkspaceNodePathPrefix string `json:"workspaceNodePathPrefix,omitempty"`

	// Enables the webui debug proxy pointing to this address
	DebugProxy string
}

type jobLog struct {
	CancelExecutorListener context.CancelFunc
	LogStore               io.Closer
}

// Service ties everything together
type Service struct {
	Logs     store.Logs
	Jobs     store.Jobs
	Groups   store.NumberGroup
	Executor *executor.Executor
	Cutter   logcutter.Cutter
	GitHub   GitHubSetup
	OnError  func(err error)

	Config Config

	mu          sync.RWMutex
	logListener map[string]*jobLog

	events emitter.Emitter
}

// GitHubSetup sets up the access to GitHub
type GitHubSetup struct {
	WebhookSecret []byte
	Client        *github.Client
}

// Start sets up everything to run this werft instance, including executor config
func (srv *Service) Start() {
	if srv.OnError == nil {
		srv.OnError = func(err error) {
			log.WithError(err).Error("service error")
		}
	}
	if srv.logListener == nil {
		srv.logListener = make(map[string]*jobLog)
	}

	srv.Executor.OnUpdate = func(pod *corev1.Pod, s *v1.JobStatus) {
		log.WithField("status", s).Info("update")

		// ensure we have logging, e.g. reestablish joblog for unknown jobs (i.e. after restart)
		srv.ensureLogging(s)

		out, err := srv.Logs.Write(s.Name)
		if err == nil {
			pw := textio.NewPrefixWriter(out, "[werft:kubernetes] ")
			k8syaml.NewSerializer(k8syaml.DefaultMetaFactory, scheme.Scheme, nil, false).Encode(pod, pw)
			pw.Flush()

			jsonStatus, _ := json.Marshal(s)
			fmt.Fprintf(out, "[werft:status] %s\n", jsonStatus)
		}

		// TODO make sure this runs only once, e.g. by improving the status computation s.t. we pass through starting
		// if s.Phase == v1.JobPhase_PHASE_RUNNING {
		// out, err := srv.Logs.Place(context.TODO(), s.Name)
		// if err == nil {
		// 	fmt.Fprintln(out, "[running|PHASE] job running")
		// }
		// }

		if s.Phase == v1.JobPhase_PHASE_CLEANUP {
			srv.mu.Lock()
			if jl, ok := srv.logListener[s.Name]; ok {
				if jl.CancelExecutorListener != nil {
					jl.CancelExecutorListener()
				}
				if jl.LogStore != nil {
					jl.LogStore.Close()
				}

				delete(srv.logListener, s.Name)
			}
			srv.mu.Unlock()

			return
		}
		err = srv.Jobs.Store(context.Background(), *s)
		if err != nil {
			srv.OnError(xerrors.Errorf("cannot store job %s: %v", s.Name, err))
		}

		err = srv.updateGitHubStatus(s)
		if err != nil {
			srv.OnError(xerrors.Errorf("cannot update GitHub status for %s: %v", s.Name, err))
		}

		// tell our Listen subscribers about this change
		<-srv.events.Emit("job", s)
	}
}

func (srv *Service) ensureLogging(s *v1.JobStatus) {
	if s.Phase > v1.JobPhase_PHASE_DONE {
		return
	}

	allOk := func() bool {
		jl, ok := srv.logListener[s.Name]
		if !ok {
			return false
		}
		if jl.CancelExecutorListener == nil {
			return false
		}

		return true
	}

	srv.mu.RLock()
	if allOk() {
		srv.mu.RUnlock()
		return
	}
	srv.mu.RUnlock()

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if allOk() {
		return
	}

	jl, ok := srv.logListener[s.Name]

	// make sure we have logging in place in general
	if !ok {
		logs, err := srv.Logs.Open(s.Name)
		if err != nil {
			log.WithError(err).WithField("name", s.Name).Error("cannot (re-)establish logs for this job")
			return
		}

		jl = &jobLog{LogStore: logs}
		srv.logListener[s.Name] = jl
	}

	// if we should be listening to the executor log, make sure we are
	if jl.CancelExecutorListener == nil {
		ctx, cancel := context.WithCancel(context.Background())
		jl.CancelExecutorListener = cancel
		go func() {
			err := srv.listenToLogs(ctx, s.Name, srv.Executor.Logs(s.Name))
			if err != nil && err != context.Canceled {
				srv.OnError(err)
				jl.CancelExecutorListener = nil
			}
		}()
	}
}

func (srv *Service) listenToLogs(ctx context.Context, name string, inc io.Reader) error {
	out, err := srv.Logs.Write(name)
	if err != nil {
		return err
	}

	// we pipe the content to the log cutter to find results
	pr, pw := io.Pipe()
	tr := io.TeeReader(inc, pw)
	evtchan, cerrchan := srv.Cutter.Slice(pr)

	// then forward the logs we read from the executor to the log store
	errchan := make(chan error, 1)
	go func() {
		_, err := io.Copy(out, tr)
		if err != nil && err != io.EOF {
			errchan <- err
		}
		close(errchan)
	}()

	for {
		select {
		case err := <-cerrchan:
			log.WithError(err).WithField("name", name).Warn("listening for build results failed")
			continue
		case evt := <-evtchan:
			if evt.Type != v1.LogSliceType_SLICE_RESULT {
				continue
			}

			segs := strings.Fields(evt.Payload)
			payload, desc := segs[0], strings.Join(segs[1:], " ")
			res := &v1.JobResult{
				Type:        strings.TrimSpace(evt.Name),
				Payload:     payload,
				Description: desc,
			}
			err := srv.Executor.RegisterResult(name, res)
			if err != nil {
				log.WithError(err).WithField("name", name).WithField("res", res).Warn("cannot record job result")
			}
		case err := <-errchan:
			if err != nil {
				srv.Executor.Stop(name, fmt.Sprintf("log infrastructure failure: %s", err.Error()))
				return xerrors.Errorf("writing logs for %s: %v", name, err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// RunJob starts a build job from some context
func (srv *Service) RunJob(ctx context.Context, name string, metadata v1.JobMetadata, cp ContentProvider, jobYAML []byte) (status *v1.JobStatus, err error) {
	var logs io.WriteCloser
	defer func(perr *error) {
		if *perr == nil {
			return
		}

		// make sure we tell the world about this failed job startup attempt
		var s v1.JobStatus
		if status != nil {
			s = *status
		}
		s.Name = name
		s.Phase = v1.JobPhase_PHASE_DONE
		s.Conditions = &v1.JobConditions{Success: false, FailureCount: 1}
		s.Metadata = &metadata
		if s.Metadata.Created == nil {
			s.Metadata.Created = ptypes.TimestampNow()
		}
		s.Details = (*perr).Error()
		logs.Write([]byte("\n[werft] FAILURE " + s.Details))

		srv.Jobs.Store(context.Background(), s)
		<-srv.events.Emit("job", &s)
	}(&err)

	logs, err = srv.Logs.Open(name)
	if err != nil {
		return nil, xerrors.Errorf("cannot start logging for %s: %w", name, err)
	}
	srv.mu.Lock()
	srv.logListener[name] = &jobLog{LogStore: logs}
	srv.mu.Unlock()

	fmt.Fprintln(logs, "[preparing|PHASE] job preparation")

	jobTpl, err := template.New("job").Funcs(sprig.FuncMap()).Parse(string(jobYAML))
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}

	buf := bytes.NewBuffer(nil)
	err = jobTpl.Execute(buf, newTemplateObj(name, &metadata))
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}

	var jobspec repoconfig.JobSpec
	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader(buf.Bytes()), 4096).Decode(&jobspec)
	// err = yaml.Unmarshal(buf.Bytes(), &jobspec)
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}

	// we have to use the Kubernetes YAML decoder to decode the podspec

	podspec := jobspec.Pod
	if podspec == nil {
		return nil, xerrors.Errorf("cannot handle job for %s: no podspec present", name)
	}

	nodePath := filepath.Join(srv.Config.WorkspaceNodePathPrefix, name)
	httype := corev1.HostPathDirectoryOrCreate
	podspec.Volumes = append(podspec.Volumes, corev1.Volume{
		Name: "werft-workspace",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: nodePath,
				Type: &httype,
			},
		},
	})
	cpinit := cp.InitContainer()
	cpinit.Name = "werft-checkout"
	cpinit.ImagePullPolicy = corev1.PullIfNotPresent
	cpinit.VolumeMounts = append(cpinit.VolumeMounts, corev1.VolumeMount{
		Name:      "werft-workspace",
		ReadOnly:  false,
		MountPath: "/workspace",
	})
	podspec.InitContainers = append(podspec.InitContainers, cpinit)
	for i, c := range podspec.Containers {
		podspec.Containers[i].VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
			Name:      "werft-workspace",
			ReadOnly:  false,
			MountPath: "/workspace",
		})
	}

	// dump podspec into logs
	pw := textio.NewPrefixWriter(logs, "[werft:template] ")
	k8syaml.NewYAMLSerializer(k8syaml.DefaultMetaFactory, nil, nil).Encode(&corev1.Pod{Spec: *podspec}, pw)
	pw.Flush()

	// schedule/start job
	status, err = srv.Executor.Start(*podspec, metadata, executor.WithName(name))
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

type templateObj struct {
	Name        string
	Owner       string
	Repository  v1.Repository
	Trigger     string
	Annotations map[string]string
}

func newTemplateObj(name string, md *v1.JobMetadata) templateObj {
	annotations := make(map[string]string)
	for _, a := range md.Annotations {
		annotations[a.Key] = a.Value
	}

	return templateObj{
		Name:        name,
		Owner:       md.Owner,
		Repository:  *md.Repository,
		Trigger:     strings.ToLower(strings.TrimPrefix(md.Trigger.String(), "TRIGGER_")),
		Annotations: annotations,
	}
}

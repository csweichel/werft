package werft

import (
	"bytes"
	"context"
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
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	k8syaml "k8s.io/apimachinery/pkg/runtime/serializer/json"
)

// Config configures the behaviour of the service
type Config struct {
	// BaseURL is the URL this service is available on (e.g. https://werft.some-domain.com)
	BaseURL string `json:"baseURL,omitempty"`

	// WorkspaceNodePathPrefix is the location on the node where we place the builds
	WorkspaceNodePathPrefix string

	// Enables the webui debug proxy pointing to this address
	DebugProxy string
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

	mu          sync.Mutex
	logListener map[string]context.CancelFunc

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
		srv.logListener = make(map[string]context.CancelFunc)
	}

	srv.Executor.OnUpdate = func(s *v1.JobStatus) {
		log.WithField("status", s).Info("update")

		if s.Phase == v1.JobPhase_PHASE_PREPARING {
			srv.mu.Lock()
			if _, alreadyListening := srv.logListener[s.Name]; !alreadyListening {
				ctx, cancel := context.WithCancel(context.Background())
				srv.logListener[s.Name] = cancel
				go func() {
					err := srv.listenToLogs(ctx, s.Name, srv.Executor.Logs(s.Name))
					if err != nil && err != context.Canceled {
						srv.OnError(err)
					}
				}()
			}
			srv.mu.Unlock()
		}

		// TODO make sure this runs only once, e.g. by improving the status computation s.t. we pass through starting
		// if s.Phase == v1.JobPhase_PHASE_RUNNING {
		// 	out, err := srv.Logs.Place(context.TODO(), s.Name)
		// 	if err == nil {
		// 		fmt.Fprintln(out, "[running|PHASE] job running")
		// 	}
		// }

		if s.Phase == v1.JobPhase_PHASE_CLEANUP {
			srv.mu.Lock()
			if srv.logListener == nil {
				srv.logListener = make(map[string]context.CancelFunc)
			}
			if stopListening, ok := srv.logListener[s.Name]; ok {
				stopListening()
			}
			srv.mu.Unlock()

			return
		}
		err := srv.Jobs.Store(context.Background(), *s)
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

func (srv *Service) listenToLogs(ctx context.Context, name string, inc io.Reader) error {
	out, err := srv.Logs.Place(ctx, name)
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

		if logs != nil {
			logs.Close()
		}
	}(&err)

	logs, err = srv.Logs.Place(context.TODO(), name)
	if err != nil {
		return nil, xerrors.Errorf("cannot start logging for %s: %w", name, err)
	}
	fmt.Fprintln(logs, "[preparing|PHASE] job preparation")

	jobTpl, err := template.New("job").Funcs(sprig.FuncMap()).Parse(string(jobYAML))
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}

	buf := bytes.NewBuffer(nil)
	err = jobTpl.Execute(buf, metadata)
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}

	var jobspec repoconfig.JobSpec
	err = yaml.Unmarshal(buf.Bytes(), &jobspec)
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}
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
	pw := textio.NewPrefixWriter(logs, "[werft] ")
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

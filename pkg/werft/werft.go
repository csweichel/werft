package werft

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"text/template"
	"time"

	sprig "github.com/Masterminds/sprig/v3"
	"github.com/csweichel/werft/pkg/api/repoconfig"
	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/csweichel/werft/pkg/executor"
	"github.com/csweichel/werft/pkg/logcutter"
	"github.com/csweichel/werft/pkg/store"
	"github.com/golang/protobuf/ptypes"
	"github.com/olebedev/emitter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/segmentio/textio"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	k8sjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
)

var (
	// annotationCleanupJob is set on jobs which cleanup after an actual user-started job.
	// These kind of jobs are not stored in the database and do not propagate through the system.
	annotationCleanupJob = "cleanupJob"
)

// Config configures the behaviour of the service
type Config struct {
	// BaseURL is the URL this service is available on (e.g. https://werft.some-domain.com)
	BaseURL string `yaml:"baseURL,omitempty"`

	// WorkspaceNodePathPrefix is the location on the node where we place the builds
	WorkspaceNodePathPrefix string `yaml:"workspaceNodePathPrefix,omitempty"`

	// CleanupJobSpec is a podspec YAML which forms the basis for cleanup jobs.
	// Can be empty, in which clean up jobs will use a default.
	CleanupJobSpec *configPodSpec `yaml:"cleanupJobSpec,omitempty"`

	// Enables the webui debug proxy pointing to this address
	DebugProxy string
}

type configPodSpec corev1.PodSpec

func (spec *configPodSpec) UnmarshalYAML(value *yaml.Node) error {
	raw, err := yaml.Marshal(value)
	if err != nil {
		return err
	}

	err = k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(raw), 4096).Decode(spec)
	if err != nil {
		return err
	}

	return nil
}

type jobLog struct {
	CancelExecutorListener context.CancelFunc
	LogStore               io.Closer
}

// Service ties everything together
type Service struct {
	Logs               store.Logs
	Jobs               store.Jobs
	Groups             store.NumberGroup
	Executor           *executor.Executor
	Cutter             logcutter.Cutter
	RepositoryProvider RepositoryProvider

	Config Config

	mu          sync.RWMutex
	logListener map[string]*jobLog

	events  emitter.Emitter
	metrics struct {
		GithubJobPreparationSeconds   prometheus.Histogram
		ExecutorJobPreperationSeconds prometheus.Histogram
	}
}

// Start sets up everything to run this werft instance, including executor config
func (srv *Service) Start() error {
	if srv.logListener == nil {
		srv.logListener = make(map[string]*jobLog)
	}
	srv.Executor.OnUpdate = srv.handleJobUpdate

	// set up prometheus gauges
	srv.metrics.GithubJobPreparationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "github_job_preparation_seconds",
		Help:    "Time it took to retrieve all required data from GitHub prior to starting a job",
		Buckets: []float64{0, 0.25, 0.5, 0.75, 1},
	})
	srv.metrics.ExecutorJobPreperationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "executor_job_preparation_seconds",
		Help:    "Time it took to start executing the job",
		Buckets: []float64{0, 0.25, 0.5, 0.75, 1},
	})

	// we might still have waiting jobs which we must load back into the executor
	waitingJobs, _, err := srv.Jobs.Find(context.Background(), []*v1.FilterExpression{
		{Terms: []*v1.FilterTerm{
			{
				Field:     "phase",
				Value:     "waiting",
				Operation: v1.FilterOp_OP_EQUALS,
			},
		}},
	}, []*v1.OrderExpression{}, 0, 0)
	if err != nil {
		return xerrors.Errorf("cannot restore waiting jobs: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	for _, j := range waitingJobs {
		cancelJob := func(err error) {
			log.WithError(err).Errorf("cannot restore waiting job %s", j.Name)
			j.Phase = v1.JobPhase_PHASE_DONE
			j.Details = fmt.Sprintf("cannot restore execution context upon werft restart: %v", err)
			j.Conditions.Success = false
			srv.handleJobUpdate(nil, &j)
		}

		jobYAML, err := srv.Jobs.GetJobSpec(j.Name)
		if err != nil {
			cancelJob(err)
			continue
		}
		waitUntil, err := ptypes.Timestamp(j.Conditions.WaitUntil)
		if err != nil {
			cancelJob(err)
			continue
		}

		md := j.Metadata
		cp, err := srv.RepositoryProvider.ContentProvider(ctx, md.Repository)
		if err != nil {
			cancelJob(err)
			continue
		}
		_, err = srv.RunJob(context.Background(), j.Name, *md, cp, jobYAML, true, waitUntil)
		if err != nil {
			cancelJob(err)
			continue
		}
	}

	go srv.doHousekeeping()

	return nil
}

// RegisterPrometheusMetrics registers the service metrics on the registerer with MustRegister
func (srv *Service) RegisterPrometheusMetrics(reg prometheus.Registerer) {
	reg.MustRegister(srv.metrics.GithubJobPreparationSeconds)
	reg.MustRegister(srv.metrics.ExecutorJobPreperationSeconds)
}

func (srv *Service) doHousekeeping() {
	tick := time.NewTicker(5 * time.Minute)
	for {
		log.Debug("performing werft service housekeeping")

		ctx := context.Background()
		expectedJobs, _, err := srv.Jobs.Find(ctx, []*v1.FilterExpression{{Terms: []*v1.FilterTerm{{Field: "phase", Value: "done", Operation: v1.FilterOp_OP_EQUALS, Negate: true}}}}, []*v1.OrderExpression{}, 0, 0)
		if err != nil {
			log.WithError(err).Warn("cannot perform housekeeping")
			<-tick.C
			continue
		}

		knownJobs, err := srv.Executor.GetKnownJobs()
		if err != nil {
			log.WithError(err).Warn("cannot perform housekeeping")
			<-tick.C
			continue
		}

		knownJobsIdx := make(map[string]v1.JobStatus)
		for _, s := range knownJobs {
			knownJobsIdx[s.Name] = s
		}

		for _, job := range expectedJobs {
			knownStatus, exists := knownJobsIdx[job.Name]
			if !exists {
				log.WithField("name", job.Name).Warn("executor does not know about this job - we have missed an event. Marking as failed.")
				job.Phase = v1.JobPhase_PHASE_DONE
				job.Conditions.Success = false
				job.Details = "Werft missed updates for this job and the job is no longer running."
				srv.handleJobUpdate(nil, &job)
				continue
			}

			if !reflect.DeepEqual(knownStatus, job) {
				log.WithField("name", job.Name).Warn("executor had a different status than what we had last seen - we have missed an event. Updating job.")
				srv.handleJobUpdate(nil, &job)
			}
		}

		<-tick.C
	}
}

func redactContainerEnv(c corev1.Container) corev1.Container {
	for j, e := range c.Env {
		nme := strings.ToLower(e.Name)
		if !strings.Contains(nme, "secret") && !strings.Contains(nme, "password") && !strings.Contains(nme, "token") {
			continue
		}

		e.Value = "[redacted]"
		c.Env[j] = e
	}
	return c
}

func (srv *Service) handleJobUpdate(pod *corev1.Pod, s *v1.JobStatus) {
	var isCleanupJob bool
	for _, annotation := range s.Metadata.Annotations {
		if annotation.Key == annotationCleanupJob {
			isCleanupJob = true
			break
		}
	}
	// We ignore all status updates from cleanup jobs - they are not user triggered and we do not want them polluting the system.
	if isCleanupJob {
		return
	}

	// ensure we have logging, e.g. reestablish joblog for unknown jobs (i.e. after restart)
	srv.ensureLogging(s)

	out, err := srv.Logs.Write(s.Name)
	if err == nil && pod != nil {
		for i, c := range pod.Spec.Containers {
			pod.Spec.Containers[i] = redactContainerEnv(c)
		}
		for i, c := range pod.Spec.InitContainers {
			pod.Spec.InitContainers[i] = redactContainerEnv(c)
		}

		pw := textio.NewPrefixWriter(out, "[werft:kubernetes] ")
		k8sjson.NewSerializer(k8sjson.DefaultMetaFactory, scheme.Scheme, nil, false).Encode(pod, pw)
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
			srv.cleanupJobWorkspace(s)

			delete(srv.logListener, s.Name)
		}
		srv.mu.Unlock()

		return
	}
	err = srv.Jobs.Store(context.Background(), *s)
	if err != nil {
		log.WithError(err).WithField("name", s.Name).Warn("cannot store job")
	}

	// tell our Listen subscribers about this change
	<-srv.events.Emit("job", s)
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
				log.WithError(err).WithField("name", s.Name).Error("cannot listen to job logs")
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

			var res *v1.JobResult
			var body struct {
				P string   `json:"payload"`
				C []string `json:"channels"`
				D string   `json:"description"`
			}
			if err := json.Unmarshal([]byte(evt.Payload), &body); err == nil {
				res = &v1.JobResult{
					Type:        strings.TrimSpace(evt.Name),
					Payload:     body.P,
					Description: body.D,
					Channels:    body.C,
				}
			} else {
				segs := strings.Fields(evt.Payload)
				payload, desc := segs[0], strings.Join(segs[1:], " ")
				res = &v1.JobResult{
					Type:        strings.TrimSpace(evt.Name),
					Payload:     payload,
					Description: desc,
				}
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
func (srv *Service) RunJob(ctx context.Context, name string, metadata v1.JobMetadata, cp ContentProvider, jobYAML []byte, canReplay bool, waitUntil time.Time) (status *v1.JobStatus, err error) {
	var logs io.WriteCloser
	defer func(perr *error) {
		if *perr != nil {
			// make sure we tell the world about this failed job startup attempt
			if status == nil {
				status = &v1.JobStatus{}
			}
			status.Name = name
			status.Phase = v1.JobPhase_PHASE_DONE
			status.Conditions = &v1.JobConditions{Success: false, FailureCount: 1}
			status.Metadata = &metadata
			if status.Metadata.Created == nil {
				status.Metadata.Created = ptypes.TimestampNow()
			}
			status.Details = (*perr).Error()
			if logs != nil {
				logs.Write([]byte("\n[werft] FAILURE " + status.Details))
			}
		}

		// either way, at the end of this function we must save the job
		serr := srv.Jobs.Store(context.Background(), *status)
		if serr != nil {
			log.WithError(serr).WithField("name", name).Warn("cannot save job - this will break things")
		}
		<-srv.events.Emit("job", status)
	}(&err)

	if canReplay {
		// save job yaml
		err = srv.Jobs.StoreJobSpec(name, jobYAML)
		if err != nil {
			log.WithError(err).Warn("cannot store job YAML - job will not be replayable")
		}
	}

	jobTpl, err := template.New("job").Funcs(sprig.TxtFuncMap()).Parse(string(jobYAML))
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}

	buf := bytes.NewBuffer(nil)
	err = jobTpl.Execute(buf, newTemplateObj(name, &metadata))
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}

	// we have to use the Kubernetes YAML decoder to decode the podspec
	var jobspec repoconfig.JobSpec
	err = k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(buf.Bytes()), 4096).Decode(&jobspec)
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}

	podspec := jobspec.Pod
	if podspec == nil {
		return nil, xerrors.Errorf("cannot handle job for %s: no podspec present", name)
	}

	for _, s := range jobspec.Sidecars {
		var found bool
		for _, p := range podspec.Containers {
			if p.Name == s {
				found = true
				break
			}
		}

		if !found {
			return nil, xerrors.Errorf("pod has no container \"%s\", but the job lists it as sidecar", s)
		}
	}

	wsVolume := "werft-workspace"
	if srv.Config.WorkspaceNodePathPrefix != "" {
		nodePath := filepath.Join(srv.Config.WorkspaceNodePathPrefix, name)
		httype := corev1.HostPathDirectoryOrCreate
		podspec.Volumes = append(podspec.Volumes, corev1.Volume{
			Name: wsVolume,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: nodePath,
					Type: &httype,
				},
			},
		})
	} else {
		podspec.Volumes = append(podspec.Volumes, corev1.Volume{
			Name: wsVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	ics, err := cp.InitContainer()
	if err != nil {
		return nil, xerrors.Errorf("cannot produce init container: %w", err)
	}
	for i, ic := range ics {
		ics[i].VolumeMounts = append(ic.VolumeMounts, corev1.VolumeMount{
			Name:      wsVolume,
			ReadOnly:  false,
			MountPath: "/workspace",
		})
	}
	podspec.InitContainers = append(podspec.InitContainers, ics...)
	for i, c := range podspec.Containers {
		podspec.Containers[i].VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
			Name:      wsVolume,
			ReadOnly:  false,
			MountPath: "/workspace",
		})
	}

	logs, err = srv.Logs.Open(name)
	if err != nil {
		return nil, xerrors.Errorf("cannot start logging for %s: %w", name, err)
	}
	srv.mu.Lock()
	srv.logListener[name] = &jobLog{LogStore: logs}
	srv.mu.Unlock()
	fmt.Fprintln(logs, "[preparing|PHASE] job preparation")

	// dump podspec into logs
	pw := textio.NewPrefixWriter(logs, "[werft:template] ")
	redactedSpec := podspec.DeepCopy()
	for ci, c := range redactedSpec.InitContainers {
		for ei, e := range c.Env {
			log.WithField("conts", strings.Contains(strings.ToLower(e.Name), "secret")).WithField("name", e.Name).Debug("redacting")
			if !strings.Contains(strings.ToLower(e.Name), "secret") {
				continue
			}

			e.Value = "[redacted]"
			c.Env[ei] = e
			redactedSpec.InitContainers[ci] = c
		}
	}
	k8sjson.NewYAMLSerializer(k8sjson.DefaultMetaFactory, nil, nil).Encode(&corev1.Pod{Spec: *redactedSpec}, pw)
	pw.Flush()

	// schedule/start job
	tExecutorPrepStart := time.Now()
	status, err = srv.Executor.Start(*podspec, metadata,
		executor.WithName(name),
		executor.WithCanReplay(canReplay),
		executor.WithWaitUntil(waitUntil),
		executor.WithMutex(jobspec.Mutex),
		executor.WithSidecars(jobspec.Sidecars),
	)
	if err != nil {
		return nil, xerrors.Errorf("cannot handle job for %s: %w", name, err)
	}
	name = status.Name
	srv.metrics.ExecutorJobPreperationSeconds.Observe(time.Since(tExecutorPrepStart).Seconds())

	err = cp.Serve(name)
	if err != nil {
		return nil, err
	}

	return status, nil
}

// cleanupWorkspace starts a cleanup job for a previously run job
func (srv *Service) cleanupJobWorkspace(s *v1.JobStatus) {
	if srv.Config.WorkspaceNodePathPrefix == "" {
		// we don't have a workspace node path prefix, hence used an emptydir volume,
		// hence don't have to clean up after ourselves.
		return
	}

	name := s.Name
	md := v1.JobMetadata{
		Owner:      s.Metadata.Owner,
		Repository: s.Metadata.Repository,
		Trigger:    v1.JobTrigger_TRIGGER_UNKNOWN,
		Created:    ptypes.TimestampNow(),
		Annotations: []*v1.Annotation{
			{
				Key:   annotationCleanupJob,
				Value: "true",
			},
		},
	}

	var podspec corev1.PodSpec
	if srv.Config.CleanupJobSpec != nil {
		// We have a cleanup job spec we ought to respect.
		podspec = corev1.PodSpec(*srv.Config.CleanupJobSpec)
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
	podspec.Containers = append(podspec.Containers, corev1.Container{
		Name:       "cleanup",
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "rm -rf *"},
		WorkingDir: "/workspace",
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "werft-workspace",
				MountPath: "/workspace",
			},
		},
	})
	podspec.RestartPolicy = corev1.RestartPolicyOnFailure
	_, err := srv.Executor.Start(podspec, md, executor.WithCanReplay(false), executor.WithBackoff(3), executor.WithName(fmt.Sprintf("cleanup-%s", name)))
	if err != nil {
		log.WithError(err).WithField("name", name).Error("cannot start cleanup job")
	}
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

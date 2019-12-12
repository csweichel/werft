package werft

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/32leaves/werft/pkg/api/repoconfig"
	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var (
	werftGithubContext = "continunous-integration/werft"

	// annotationStatusUpdate is set on jobs whoose status needs to be updated on GitHub.
	// This is set only on jobs created through GitHub events.
	annotationStatusUpdate = "updateGitHubStatus"
)

func (srv *Service) updateGitHubStatus(job *v1.JobStatus) error {
	var needsStatusUpdate bool
	for _, a := range job.Metadata.Annotations {
		if a.Key == annotationStatusUpdate {
			needsStatusUpdate = true
			break
		}
	}
	if !needsStatusUpdate {
		return nil
	}

	var (
		state string
		desc  string
	)

	switch job.Phase {
	case v1.JobPhase_PHASE_PREPARING, v1.JobPhase_PHASE_STARTING, v1.JobPhase_PHASE_RUNNING:
		state = "pending"
		desc = "build is " + strings.TrimPrefix(strings.ToLower(job.Phase.String()), "phase_")
	default:
		if job.Conditions.Success {
			state = "success"
			desc = "The build succeeded!"
		} else {
			state = "failure"
			desc = "The build failed!"
		}
	}
	url := fmt.Sprintf("%s/job/%s", srv.Config.BaseURL, job.Name)
	ghstatus := &github.RepoStatus{
		State:       &state,
		Description: &desc,
		Context:     &werftGithubContext,
		TargetURL:   &url,
	}
	log.WithField("status", ghstatus).Debugf("updating GitHub status for %s", job.Name)
	ctx := context.Background()
	_, _, err := srv.GitHub.Client.Repositories.CreateStatus(ctx, job.Metadata.Repository.Owner, job.Metadata.Repository.Repo, job.Metadata.Repository.Revision, ghstatus)
	if err != nil {
		return err
	}

	return nil
}

// HandleGithubWebhook handles incoming Github events
func (srv *Service) HandleGithubWebhook(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func(err *error) {
		if *err == nil {
			return
		}

		srv.OnError(*err)
		http.Error(w, (*err).Error(), http.StatusInternalServerError)
	}(&err)

	if r.Method == "GET" {
		http.Redirect(w, r, "/github?"+r.URL.Query().Encode(), 301)
		return
	}

	payload, err := github.ValidatePayload(r, srv.GitHub.WebhookSecret)
	if err != nil && strings.Contains(err.Error(), "unknown X-Github-Event") {
		err = nil
		return
	}
	if err != nil {
		return
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		return
	}
	switch event := event.(type) {
	case *github.PushEvent:
		srv.processPushEvent(event)
	case *github.InstallationEvent:
		srv.processInstallationEvent(event)
	default:
		log.WithField("event", event).Debug("unhandled GitHub event")
		http.Error(w, "unhandled event", http.StatusInternalServerError)
	}
}

func (srv *Service) processPushEvent(event *github.PushEvent) {
	ctx := context.Background()
	rev := *event.After

	ref := strings.TrimPrefix(*event.Ref, "refs/heads/")
	flatname := strings.ToLower(strings.ReplaceAll(ref, "/", "-"))

	metadata := v1.JobMetadata{
		Owner: *event.Pusher.Name,
		Repository: &v1.Repository{
			Owner:    *event.Repo.Owner.Name,
			Repo:     *event.Repo.Name,
			Ref:      ref,
			Revision: rev,
		},
		Trigger: v1.JobTrigger_TRIGGER_PUSH,
		Annotations: []*v1.Annotation{
			&v1.Annotation{
				Key:   annotationStatusUpdate,
				Value: "true",
			},
		},
	}

	cp := &GitHubContentProvider{
		Client:   srv.GitHub.Client,
		Owner:    metadata.Repository.Owner,
		Repo:     metadata.Repository.Repo,
		Revision: rev,
	}
	repoCfg, err := getRepoCfg(ctx, cp)
	if err != nil {
		log.WithError(err).WithField("name", flatname).Error("cannot start job")
		return
	}

	// check if we need to build/do anything
	if !repoCfg.ShouldRun(metadata.Trigger) {
		return
	}

	// download job podspec from GitHub
	tplpth := repoCfg.TemplatePath(metadata.Trigger)
	jobTplYAML, err := cp.Download(ctx, tplpth)
	if err != nil {
		log.WithError(err).WithField("name", flatname).Error("cannot start job")
		return
	}
	jobTplRaw, err := ioutil.ReadAll(jobTplYAML)
	if err != nil {
		log.WithError(err).WithField("name", flatname).Error("cannot start job")
		return
	}

	// acquire job number
	t, err := srv.Groups.Next(flatname)
	if err != nil {
		srv.OnError(err)
	}
	jobName := filepath.Base(tplpth)
	jobName = strings.TrimSuffix(jobName, filepath.Ext(jobName))
	name := fmt.Sprintf("%s-%s.%d", jobName, flatname, t)

	_, err = srv.RunJob(ctx, name, metadata, cp, jobTplRaw)
	if err != nil {
		srv.OnError(err)
	}
}

func getRepoCfg(ctx context.Context, fp FileProvider) (*repoconfig.C, error) {
	// download werft config from branch
	werftYAML, err := fp.Download(ctx, PathWerftConfig)
	if err != nil {
		// TODO handle repos without werft config more gracefully
		return nil, err
	}
	var repoCfg repoconfig.C
	err = yaml.NewDecoder(werftYAML).Decode(&repoCfg)
	if err != nil {
		return nil, err
	}

	return &repoCfg, nil
}

func (srv *Service) processInstallationEvent(event *github.InstallationEvent) {
	if *event.Action != "created" {
		return
	}

	log.WithFields(log.Fields{
		"action":         *event.Action,
		"sender":         event.Sender.Name,
		"installationID": *event.Installation.ID,
		"appID":          *event.Installation.AppID,
	}).Info("someone just installed a GitHub app for this webhook")
}

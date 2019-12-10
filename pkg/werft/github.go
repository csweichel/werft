package werft

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
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
	log.WithField("status", ghstatus).Debug("updating GitHub status for %s", job.Name)
	ctx := context.Background()
	_, _, err := srv.GitHub.Client.Repositories.CreateStatus(ctx, job.Metadata.Repository.Owner, job.Metadata.Repository.Repo, job.Metadata.Repository.Revision, ghstatus)
	if err != nil {
		return err
	}

	return nil
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
	flatname := *event.Repo.Name + "-" + strings.ToLower(strings.ReplaceAll(ref, "/", "-"))
	t, err := srv.Groups.Next(flatname)
	if err != nil {
		srv.OnError(err)
	}
	name := fmt.Sprintf("%s.%d", flatname, t)

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

	content := &GitHubContentProvider{
		Client:   srv.GitHub.Client,
		Owner:    metadata.Repository.Owner,
		Repo:     metadata.Repository.Repo,
		Revision: rev,
	}
	_, err = srv.RunJob(ctx, name, metadata, content)
	if err != nil {
		srv.OnError(err)
	}
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

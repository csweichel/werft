package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"

	"github.com/bradleyfalzon/ghinstallation"
	v1 "github.com/csweichel/werft/pkg/api/v1"
	plugin "github.com/csweichel/werft/pkg/plugin/client"
	"github.com/google/go-github/v31/github"
	log "github.com/sirupsen/logrus"
)

var (
	werftGithubContext       = "continuous-integration/werft"
	werftResultGithubContext = "continuous-integration/werft/result"

	// annotationStatusUpdate is set on jobs whoose status needs to be updated on GitHub.
	// This is set only on jobs created through GitHub events.
	annotationStatusUpdate = "updateGitHubStatus"

	defaultGitHubHost = "github.com"
)

// Config configures this plugin
type Config struct {
	BaseURL string `yaml:"baseURL"`

	WebhookSecret  string `yaml:"webhookSecret"`
	PrivateKeyPath string `yaml:"privateKeyPath"`
	InstallationID int64  `yaml:"installationID,omitempty"`
	AppID          int64  `yaml:"appID"`

	PRComments struct {
		Enabled bool `yaml:"enabled"`

		// If this is a non-empty list, the commenting user needs to be in at least one
		// of the organisations listed here for the build to start.
		RequiresOrganisation []string `yaml:"requiresOrg"`

		// If true, we'll update the comment to give feedback about what werft understood.
		UpdateComment bool `yaml:"updateComment"`
	} `yaml:"pullRequestComments"`
}

func main() {
	plg := &githubTriggerPlugin{}
	plugin.Serve(&Config{},
		plugin.WithIntegrationPlugin(plg),
		plugin.WithProxyPass(plg),
	)
}

type githubTriggerPlugin struct {
	Config *Config
	Werft  v1.WerftServiceClient
	Github *github.Client
}

func (p *githubTriggerPlugin) Run(ctx context.Context, config interface{}, srv v1.WerftServiceClient) error {
	cfg, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("config has wrong type %s", reflect.TypeOf(config))
	}

	ghtr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, cfg.AppID, cfg.InstallationID, cfg.PrivateKeyPath)
	if err != nil {
		return err
	}
	ghClient := github.NewClient(&http.Client{Transport: ghtr})

	p.Config = cfg
	p.Werft = srv
	p.Github = ghClient

	errchan := make(chan error)
	sub, err := srv.Subscribe(ctx, &v1.SubscribeRequest{})
	if err != nil {
		return fmt.Errorf("cannot subscribe for notification: %w", err)
	}
	log.Infof("status updates for GitHub set up")
	go func() {
		for {
			inc, err := sub.Recv()
			if err != nil {
				errchan <- err
				return
			}

			err = p.updateGitHubStatus(inc.Result)
			if err != nil {
				log.WithError(err).Error("cannot update GitHub status")
			}
		}
	}()

	select {
	case err := <-errchan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *githubTriggerPlugin) updateGitHubStatus(job *v1.JobStatus) error {
	var wantsUpdate bool
	for _, a := range job.Metadata.Annotations {
		if a.Key == annotationStatusUpdate {
			wantsUpdate = true
			break
		}
	}
	if !wantsUpdate {
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
	url := fmt.Sprintf("%s/job/%s", p.Config.BaseURL, job.Name)
	ghstatus := &github.RepoStatus{
		State:       &state,
		Description: &desc,
		Context:     &werftGithubContext,
		TargetURL:   &url,
	}
	log.WithField("status", ghstatus).Debugf("updating GitHub status for %s", job.Name)
	ctx := context.Background()
	_, _, err := p.Github.Repositories.CreateStatus(ctx, job.Metadata.Repository.Owner, job.Metadata.Repository.Repo, job.Metadata.Repository.Revision, ghstatus)
	if err != nil {
		return err
	}

	// update all result statuses
	var idx int
	for _, r := range job.Results {
		var ok bool
		for _, c := range r.Channels {
			if c == "github" {
				ok = true
				break
			}
		}
		if !ok {
			continue
		}

		resultURL := url
		if r.Type == "url" {
			resultURL = r.Payload
		}
		success := "success"
		ghcontext := fmt.Sprintf("%s-%03d", werftResultGithubContext, idx)
		_, _, err := p.Github.Repositories.CreateStatus(ctx,
			job.Metadata.Repository.Owner,
			job.Metadata.Repository.Repo,
			job.Metadata.Repository.Revision,
			&github.RepoStatus{
				State:       &success,
				TargetURL:   &resultURL,
				Description: &r.Description,
				Context:     &ghcontext,
			},
		)
		if err != nil {
			log.WithError(err).WithField("job", job.Name).Warn("cannot update result status")
		}

	}

	return nil
}

func (p *githubTriggerPlugin) Serve(ctx context.Context, l net.Listener) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", http.HandlerFunc(p.HandleGithubWebhook))
	http.Serve(l, mux)
	<-ctx.Done()

	return nil
}

// HandleGithubWebhook handles incoming Github events
func (p *githubTriggerPlugin) HandleGithubWebhook(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func(err *error) {
		if *err == nil {
			return
		}

		log.WithError(*err).Warn("GitHub webhook error")
		http.Error(w, (*err).Error(), http.StatusInternalServerError)
	}(&err)

	if r.Method == "GET" {
		http.Redirect(w, r, "/github?"+r.URL.Query().Encode(), 301)
		return
	}

	payload, err := github.ValidatePayload(r, []byte(p.Config.WebhookSecret))
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
		p.processPushEvent(event)
	case *github.InstallationEvent:
		p.processInstallationEvent(event)
	case *github.IssueCommentEvent:
		p.processIssueCommentEvent(r.Context(), event)
	default:
		log.WithField("event", event).Debug("unhandled GitHub event")
		http.Error(w, "unhandled event", http.StatusInternalServerError)
	}
}

func (p *githubTriggerPlugin) processPushEvent(event *github.PushEvent) {
	ctx := context.Background()
	rev := *event.After

	trigger := v1.JobTrigger_TRIGGER_PUSH
	if event.Deleted != nil && *event.Deleted {
		trigger = v1.JobTrigger_TRIGGER_DELETED
	}

	metadata := v1.JobMetadata{
		Owner: *event.Pusher.Name,
		Repository: &v1.Repository{
			Host:     defaultGitHubHost,
			Owner:    *event.Repo.Owner.Name,
			Repo:     *event.Repo.Name,
			Ref:      *event.Ref,
			Revision: rev,
		},
		Trigger: trigger,
		Annotations: []*v1.Annotation{
			&v1.Annotation{
				Key:   annotationStatusUpdate,
				Value: "true",
			},
		},
	}

	_, err := p.Werft.StartGitHubJob(ctx, &v1.StartGitHubJobRequest{
		Metadata: &metadata,
	})
	if err != nil {
		log.WithError(err).Warn("GitHub webhook error")
	}
}

func (p *githubTriggerPlugin) processInstallationEvent(event *github.InstallationEvent) {
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

func (p *githubTriggerPlugin) processIssueCommentEvent(ctx context.Context, event *github.IssueCommentEvent) {
	if !p.Config.PRComments.Enabled {
		return
	}
	if event.GetAction() != "created" {
		return
	}
	if !event.GetIssue().IsPullRequest() {
		return
	}

	var (
		segs  = strings.Split(event.GetRepo().GetFullName(), "/")
		prRepoOwner = segs[0]
		prRepoRepo  = segs[1]
	)

	var feedback struct {
		Success bool
		Message string
	}
	defer func() {
		if !p.Config.PRComments.UpdateComment {
			return
		}

		icon := ":white_check_mark:"
		if !feedback.Success {
			icon = ":x:"
		}

		comment := event.GetComment()
		lines := strings.Split(comment.GetBody(), "\n")
		newlines := make([]string, 0, len(lines)+2)
		for _, l := range lines {
			newlines = append(newlines, l)
			if strings.HasPrefix(strings.TrimSpace(l), "/werft ") {
				newlines = append(newlines, "", fmt.Sprintf("%s   %s", icon, feedback.Message))
			}
			body := strings.Join(newlines, "\n")
			comment.Body = &body
		}

		p.Github.Issues.EditComment(ctx, prRepoOwner, prRepoRepo, event.GetComment().GetID(), comment)
	}()

	sender := event.GetSender().GetLogin()
	if len(p.Config.PRComments.RequiresOrganisation) > 0 {
		var allowed bool
		for _, org := range p.Config.PRComments.RequiresOrganisation {
			ok, _, err := p.Github.Organizations.IsMember(ctx, org, sender)
			if err != nil {
				log.WithError(err).WithField("org", org).WithField("user", sender).Warn("cannot check organisation membership")
			}
			if ok {
				allowed = true
				break
			}
		}

		if !allowed {
			feedback.Success = false
			feedback.Message = "not authorized"
			return
		}
	}

	pr, _, err := p.Github.PullRequests.Get(ctx, prRepoOwner, prRepoRepo, event.GetIssue().GetNumber())
	if err != nil {
		log.WithError(err).Warn("GitHub webhook error")
		feedback.Success = false
		feedback.Message = "cannot find corresponding PR"
		return
	}

	var run bool
	lines := strings.Split(event.GetComment().GetBody(), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "/werft") {
			continue
		}
		l = strings.TrimPrefix(l, "/werft")
		l = strings.TrimSpace(l)

		fmt.Println(l)
		if l == "run" {
			run = true
			break
		} else {
			feedback.Success = false
			feedback.Message = fmt.Sprintf("unknown command `%s` - only `run` is supported", l)
		}
	}
	if !run {
		return
	}

	segs = strings.Split(pr.GetHead().GetRepo().GetFullName(), "/")
	var (
		owner = segs[0]
		repo = segs[1]
	)
	metadata := v1.JobMetadata{
		Owner: event.GetSender().GetLogin(),
		Repository: &v1.Repository{
			Host:     defaultGitHubHost,
			Owner:    owner,
			Repo:     repo,
			Ref:      pr.GetHead().GetRef(),
			Revision: pr.GetHead().GetSHA(),
		},
		Trigger: v1.JobTrigger_TRIGGER_MANUAL,
		Annotations: []*v1.Annotation{
			&v1.Annotation{
				Key:   annotationStatusUpdate,
				Value: "true",
			},
		},
	}
	resp, err := p.Werft.StartGitHubJob(ctx, &v1.StartGitHubJobRequest{
		Metadata: &metadata,
	})
	if err != nil {
		log.WithError(err).Warn("GitHub webhook error")
		feedback.Success = false
		feedback.Message = "cannot start job - please talk to whoever's in charge of your Werft installation"
		return
	}

	feedback.Success = true
	feedback.Message = fmt.Sprintf("started the job as [%s](%s/job/%s)", resp.Status.Name, p.Config.BaseURL, resp.Status.Name)
}

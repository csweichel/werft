package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"reflect"
	"strings"

	"github.com/bradleyfalzon/ghinstallation"
	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/csweichel/werft/pkg/plugin/client"
	plugin "github.com/csweichel/werft/pkg/plugin/client"
	"github.com/google/go-github/v35/github"
	log "github.com/sirupsen/logrus"
)

var (
	werftGithubContextPrefix = "ci/werft"
	werftResultChannelPrefix = "github-check-"

	// annotationStatusUpdate is set on jobs whoose status needs to be updated on GitHub.
	// This is set only on jobs created through GitHub events.
	annotationStatusUpdate = "updateGitHubStatus"

	defaultGitHubHost = "github.com"

	commandHelp = `You can interact with werft using: ` + "`" + `/werft command <args>` + "`" + `.
Available commands are:
 - ` + "`" + `/werft run [annotation=value]` + "`" + ` which starts a new werft job from this context.
    You can optionally pass multiple whitespace-separated annotations.
 - ` + "`" + `/werft help` + "`" + ` displays this help
`
)

// Config configures this plugin
type Config struct {
	BaseURL string `yaml:"baseURL"`

	WebhookSecret  string             `yaml:"webhookSecret"`
	PrivateKeyPath string             `yaml:"privateKeyPath"`
	InstallationID int64              `yaml:"installationID,omitempty"`
	AppID          int64              `yaml:"appID"`
	JobProtection  JobProtectionLevel `yaml:"jobProtection"`

	PRComments struct {
		Enabled bool `yaml:"enabled"`

		// If this is a non-empty list, the commenting user needs to be in at least one
		// of the organisations listed here for the build to start.
		RequiresOrganisation []string `yaml:"requiresOrg"`

		// If true, we'll update the comment to give feedback about what werft understood.
		UpdateComment bool `yaml:"updateComment"`
	} `yaml:"pullRequestComments"`
}

type JobProtectionLevel string

const (
	JobProtectionOff           JobProtectionLevel = ""
	JobProtectionDefaultBranch JobProtectionLevel = "default-branch"
)

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

func (p *githubTriggerPlugin) Run(ctx context.Context, config interface{}, srv *client.Services) error {
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
	var (
		wantsUpdate   bool
		statusDstRepo string
	)
	for _, a := range job.Metadata.Annotations {
		if a.Key == annotationStatusUpdate {
			wantsUpdate = true
			statusDstRepo = a.Value
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
	jobGHctx := werftGithubContextPrefix + "/" + job.Metadata.JobSpecName
	ghstatus := &github.RepoStatus{
		State:       &state,
		Description: &desc,
		Context:     &jobGHctx,
		TargetURL:   &url,
	}

	var (
		segs  = strings.Split(statusDstRepo, "/")
		owner string
		repo  string
	)
	if len(segs) == 2 {
		owner, repo = segs[0], segs[1]
	} else {
		owner, repo = job.Metadata.Owner, job.Metadata.Repository.Repo
	}

	log.WithField("status", ghstatus).Debugf("updating GitHub status for %s", job.Name)
	ctx := context.Background()
	_, _, err := p.Github.Repositories.CreateStatus(ctx, owner, repo, job.Metadata.Repository.Revision, ghstatus)
	if err != nil {
		return err
	}

	// update all result statuses
	var idx int
	for _, r := range job.Results {
		var (
			ok    bool
			ghctx string
		)
		for _, c := range r.Channels {
			if c == "github" {
				ok = true
				ghctx = fmt.Sprintf("%s/results/%03d", jobGHctx, idx)
				idx++
				break
			}
			if strings.HasPrefix(c, werftResultChannelPrefix) {
				ok = true
				ghctx = fmt.Sprintf("%s/results/%s", jobGHctx, strings.TrimPrefix(c, werftResultChannelPrefix))
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
		if r.Type == "conclusion" {
			success = r.Payload
		}
		_, _, err := p.Github.Repositories.CreateStatus(ctx,
			owner,
			repo,
			job.Metadata.Repository.Revision,
			&github.RepoStatus{
				State:       &success,
				TargetURL:   &resultURL,
				Description: &r.Description,
				Context:     &ghctx,
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
	case *github.DeleteEvent:
		// handled by the push event already
	default:
		log.WithField("event", event).Debug("unhandled GitHub event")
		http.Error(w, "unhandled event", http.StatusInternalServerError)
	}
}

func (p *githubTriggerPlugin) processPushEvent(event *github.PushEvent) {
	ctx := context.Background()
	rev := event.GetAfter()

	trigger := v1.JobTrigger_TRIGGER_PUSH
	if event.Deleted != nil && *event.Deleted {
		trigger = v1.JobTrigger_TRIGGER_DELETED

		// rev after deletion makes no sense
		rev = ""
	}

	req := p.prepareStartJobRequest(event.Pusher, event.Repo.Owner, event.Repo.Owner, event.Repo, event.Repo, event.GetRef(), rev, trigger)
	_, err := p.Werft.StartJob2(ctx, req)

	if err != nil {
		log.WithError(err).Warn("GitHub webhook error")
	}
}

type githubRepo interface {
	GetName() string
	GetFullName() string
	GetDefaultBranch() string
}

func (p *githubTriggerPlugin) prepareStartJobRequest(pusher, srcOwner, dstOwner *github.User, src, dst githubRepo, ref, rev string, trigger v1.JobTrigger) *v1.StartJobRequest2 {
	metadata := v1.JobMetadata{
		Owner: pusher.GetLogin(),
		Repository: &v1.Repository{
			Host:          defaultGitHubHost,
			Owner:         srcOwner.GetLogin(),
			Repo:          src.GetName(),
			Ref:           ref,
			Revision:      rev,
			DefaultBranch: src.GetDefaultBranch(),
		},
		Trigger: trigger,
	}
	if trigger != v1.JobTrigger_TRIGGER_DELETED {
		metadata.Annotations = []*v1.Annotation{
			{
				Key:   annotationStatusUpdate,
				Value: dstOwner.GetLogin() + "/" + dst.GetName(),
			},
		}
	}

	var spec v1.JobSpec
	if dstOwner.GetID() != srcOwner.GetID() {
		spec.NameSuffix = "fork"
	}

	if p.Config.JobProtection == JobProtectionDefaultBranch ||
		trigger == v1.JobTrigger_TRIGGER_DELETED {
		defaultBranch := &v1.Repository{
			Host:          defaultGitHubHost,
			Owner:         dstOwner.GetLogin(),
			Repo:          dst.GetName(),
			Ref:           "refs/heads/" + dst.GetDefaultBranch(),
			DefaultBranch: dst.GetDefaultBranch(),
		}
		spec.Source = &v1.JobSpec_Repo{
			Repo: &v1.JobSpec_FromRepo{
				Repo: defaultBranch,
			},
		}
		spec.RepoSideload = append(spec.RepoSideload, &v1.JobSpec_FromRepo{
			Repo: defaultBranch,
			Path: ".werft",
		})
	} else {
		// let werft decide where to get the job from
		spec.Source = &v1.JobSpec_JobPath{}
	}

	return &v1.StartJobRequest2{
		Metadata: &metadata,
		Spec:     &spec,
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
		segs       = strings.Split(event.GetRepo().GetFullName(), "/")
		prDstOwner = segs[0]
		prDstRepo  = segs[1]
	)

	var feedback struct {
		Success bool
		Message string
	}
	defer func() {
		if !p.Config.PRComments.UpdateComment {
			return
		}

		icon := ":+1:"
		if !feedback.Success {
			icon = ":-1:"
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

		p.Github.Issues.EditComment(ctx, prDstOwner, prDstRepo, event.GetComment().GetID(), comment)
	}()

	pr, _, err := p.Github.PullRequests.Get(ctx, prDstOwner, prDstRepo, event.GetIssue().GetNumber())
	if err != nil {
		log.WithError(err).Warn("GitHub webhook error")
		feedback.Success = false
		feedback.Message = "cannot find corresponding PR"
		return
	}

	var (
		sender  = event.GetSender().GetLogin()
		allowed = true
	)
	if len(p.Config.PRComments.RequiresOrganisation) > 0 {
		allowed = false
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
	}
	permissions, _, err := p.Github.Repositories.GetPermissionLevel(ctx, prDstOwner, prDstRepo, sender)
	if err != nil {
		log.WithError(err).WithField("repo", fmt.Sprintf("%s/%s", prDstOwner, prDstRepo)).WithField("user", sender).Warn("cannot get permission level")
	}
	switch permissions.GetPermission() {
	case "admin", "write":
		// leave allowed as it stands
	default:
		allowed = false
	}
	if !allowed {
		feedback.Success = false
		feedback.Message = "not authorized"
		return
	}

	lines := strings.Split(event.GetComment().GetBody(), "\n")
	for _, l := range lines {
		cmd, args, err := parseCommand(l)
		if err != nil {
			feedback.Success = false
			feedback.Message = fmt.Sprintf("cannot parse %s: %v", l, err)
			continue
		}
		if cmd == "" {
			continue
		}

		var resp string
		switch cmd {
		case "run":
			resp, err = p.handleCommandRun(ctx, event, pr, args)
		case "help":
			resp = commandHelp
		default:
			err = fmt.Errorf("unknown command: %s\nUse `/werft help` to list the available commands", cmd)
		}
		if err != nil {
			log.WithError(err).Warn("GitHub webhook error")
			feedback.Success = false
			feedback.Message = err.Error()
			return
		}

		feedback.Success = true
		feedback.Message = resp
	}
}

func (p *githubTriggerPlugin) handleCommandRun(ctx context.Context, event *github.IssueCommentEvent, pr *github.PullRequest, args []string) (msg string, err error) {
	argm := make(map[string]string)
	for _, arg := range args {
		var key, value string
		if segs := strings.Split(arg, "="); len(segs) == 1 {
			key = arg
		} else {
			key, value = segs[0], strings.Join(segs[1:], "=")
		}
		argm[key] = value
	}

	annotations := make([]*v1.Annotation, 0, len(argm))
	for k, v := range argm {
		annotations = append(annotations, &v1.Annotation{
			Key:   k,
			Value: v,
		})
	}

	ref := pr.GetHead().GetRef()
	if !strings.HasPrefix(ref, "refs/") {
		// we assume this is a branch
		ref = "refs/heads/" + ref
	}

	fc, _ := json.Marshal(pr)
	ioutil.WriteFile("/tmp/pr.json", fc, 0644)

	src := pr.GetHead().GetRepo()
	dst := event.GetRepo()
	req := p.prepareStartJobRequest(event.Comment.User, src.Owner, dst.Owner, src, dst, ref, pr.GetHead().GetSHA(), v1.JobTrigger_TRIGGER_MANUAL)
	resp, err := p.Werft.StartJob2(ctx, req)
	if err != nil {
		log.WithError(err).Warn("GitHub webhook error")
		return "", fmt.Errorf("cannot start job - please talk to whoever's in charge of your Werft installation")
	}

	msg = fmt.Sprintf("started the job as [%s](%s/job/%s)", resp.Status.Name, p.Config.BaseURL, resp.Status.Name)
	switch p.Config.JobProtection {
	case JobProtectionDefaultBranch:
		msg += fmt.Sprintf("\n(with `.werft/` from `%s`)", dst.GetDefaultBranch())
	}

	return msg, nil
}

func parseCommand(l string) (cmd string, args []string, err error) {
	l = strings.TrimSpace(l)
	if !strings.HasPrefix(l, "/werft") {
		return
	}
	l = strings.TrimPrefix(l, "/werft")
	l = strings.TrimSpace(l)

	segs := strings.Fields(l)
	if len(segs) < 1 {
		return "", nil, fmt.Errorf("missing command")
	}

	return segs[0], segs[1:], nil
}

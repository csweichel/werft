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
	"github.com/csweichel/werft/pkg/plugin/client"
	plugin "github.com/csweichel/werft/pkg/plugin/client"
	repo "github.com/csweichel/werft/plugins/github-repo/pkg/provider"
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
	// JobProtectionOff           JobProtectionLevel = ""
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

	testMode bool
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
	case *github.PullRequestEvent:
		p.processPullRequestEditedEvent(r.Context(), event)
	case *github.DeleteEvent:
		// handled by the push event already
	default:
		log.WithField("event", event).Debug("unhandled GitHub event")
		http.Error(w, "unhandled event", http.StatusInternalServerError)
	}
}

func (p *githubTriggerPlugin) processPullRequestEditedEvent(ctx context.Context, event *github.PullRequestEvent) {
	pr := event.PullRequest
	// Potentially (depending on the webhook configuration) we'll get multiple PR events - when pushing (every push generates a "synchronize" pull_request event if there's a PR open),
	// adding/removing a label,reviewer, assigning/unassigning, etc. The only time annotations are relevant are when a PRs description is edited (opening a PR is handled by a different flow)
	// So this is the only one we process at this time, the rest we discard.
	if event.GetAction() != "edited" {
		return
	}

	// We don't care to process unless the body actually changed
	if event.GetChanges() == nil || event.GetChanges().Body == nil {
		return
	}

	oldAnnotations := repo.ParseAnnotations(*event.GetChanges().Body.From)
	prAnnotations := repo.ParseAnnotations(pr.GetBody())
	// the annotations didn't change between the PR edits - there's nothing to do
	if mapEQ(oldAnnotations, prAnnotations) {
		return
	}

	hasStatusUpdateAnnotation := len(prAnnotations) > 0 && prAnnotations[annotationStatusUpdate] != ""

	ref := pr.GetHead().GetRef()
	if !strings.HasPrefix(ref, "refs/") {
		// we assume this is a branch
		ref = "refs/heads/" + ref
	}
	rev := pr.GetHead().GetSHA()

	lastJobs, err := p.Werft.ListJobs(ctx, &v1.ListJobsRequest{
		Filter: []*v1.FilterExpression{
			{
				Terms: []*v1.FilterTerm{
					{Field: "repo.ref", Value: ref, Operation: v1.FilterOp_OP_EQUALS},
				},
			},
		},
		Order: []*v1.OrderExpression{{Field: "created", Ascending: false}},
		Limit: 10,
	})
	if err != nil {
		log.WithError(err).Warn("cannot list last jobs when handling PR body update")
		return
	}

	// We only care about the last job that run for the same commit as that of the PR Event
	for _, lastJob := range lastJobs.Result {
		if lastJob.Metadata.Repository.Revision != rev {
			continue
		}

		jobAnnotations := make(map[string]string, len(lastJob.Metadata.Annotations))
		for _, a := range lastJob.Metadata.Annotations {
			// Potentially all previous jobs would have been started with the updateGithubStatus annotation, as that annotation is added on a push event
			// The PR Edit event we process here will NOT have that annotation at this stage, unless it's been explicitly added in the body of the PR
			// Which would lead to a job being triggered every time we process an event - by always having a mismatch
			// Therefore we exclude it in our comparison if it doesn't exist in the PR Annotations AND in the last job
			if !hasStatusUpdateAnnotation && a.Key == annotationStatusUpdate {
				continue
			}
			jobAnnotations[a.Key] = a.Value
		}

		// If the annotations didn't change between the last job and the event we're currently processing we have nothing to do
		if mapEQ(prAnnotations, jobAnnotations) {
			return
		}

		// If we got here, it means that the annotations changed, so we should continue and launch a job with the new annotations
		// Also we don't care for the rest of the "previous" jobs, so we discard by breaking out of the loop
		break
	}

	var (
		msg        string
		segs       = strings.Split(event.GetRepo().GetFullName(), "/")
		prDstOwner = segs[0]
		prDstRepo  = segs[1]
	)
	if p.userIsAllowedToStartJob(ctx, prDstOwner, prDstRepo, event.GetSender().GetLogin()) {
		req := p.prepareStartJobRequest(event.Sender, event.Repo.Owner, event.Repo.Owner, event.Repo, event.Repo, ref, rev, v1.JobTrigger_TRIGGER_MANUAL)
		for k, v := range prAnnotations {
			req.Metadata.Annotations = append(req.Metadata.Annotations, &v1.Annotation{
				Key:   k,
				Value: v,
			})
		}
		resp, err := p.Werft.StartJob2(ctx, req)
		if err == nil {
			msg = fmt.Sprintf("started the job as [%s](%s/job/%s) because the annotations in the pull request description changed", resp.Status.Name, p.Config.BaseURL, resp.Status.Name)
			switch p.Config.JobProtection {
			case JobProtectionDefaultBranch:
				msg += fmt.Sprintf("\n(with `.werft/` from `%s`)", event.Repo.GetDefaultBranch())
			}
		} else {
			log.WithError(err).Warn("GitHub webhook error")
			msg = "cannot start job - please talk to whoever's in charge of your Werft installation"
		}
	} else {
		msg = "annotations in the pull request changed, but user is not allowed to start a job"
	}

	if p.testMode {
		return
	}
	_, _, err = p.Github.Issues.CreateComment(ctx, prDstOwner, prDstRepo, pr.GetNumber(), &github.IssueComment{
		Body: &msg,
	})
	if err != nil {
		log.WithError(err).Error("cannot create comment after handling PR body update")
	}
}

func mapEQ(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		bv, ok := b[k]
		if !ok || v != bv {
			return false
		}
	}
	return true
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

	req := p.prepareStartJobRequest(event.Sender, event.Repo.Owner, event.Repo.Owner, event.Repo, event.Repo, event.GetRef(), rev, trigger)
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

func (p *githubTriggerPlugin) userIsAllowedToStartJob(ctx context.Context, owner, repo, user string) bool {
	if p.testMode {
		return true
	}

	permissions, _, err := p.Github.Repositories.GetPermissionLevel(ctx, owner, repo, user)
	if err != nil {
		log.WithError(err).WithField("repo", fmt.Sprintf("%s/%s", owner, repo)).WithField("user", user).Warn("cannot get permission level")
		return false
	}
	switch permissions.GetPermission() {
	case "admin", "write":
		return true
	default:
		return false
	}
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
	if !p.userIsAllowedToStartJob(ctx, prDstOwner, prDstRepo, sender) {
		allowed = false
	}
	if !allowed {
		feedback.Success = false
		feedback.Message = "not authorized"
		return
	}

	cmd, args, err := parseCommand(event.GetComment().GetBody())
	if err != nil {
		feedback.Success = false
		feedback.Message = err.Error()
		return
	}
	if cmd == "" {
		return
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

	ref := pr.GetHead().GetRef()
	if !strings.HasPrefix(ref, "refs/") {
		// we assume this is a branch
		ref = "refs/heads/" + ref
	}

	src := pr.GetHead().GetRepo()
	dst := event.GetRepo()

	req := p.prepareStartJobRequest(event.Comment.User, src.Owner, dst.Owner, src, dst, ref, pr.GetHead().GetSHA(), v1.JobTrigger_TRIGGER_MANUAL)
	for _, e := range req.Metadata.Annotations {
		delete(argm, e.Key)
	}
	for k, v := range argm {
		req.Metadata.Annotations = append(req.Metadata.Annotations, &v1.Annotation{
			Key:   k,
			Value: v,
		})
	}

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

func parseCommand(msg string) (cmd string, args []string, err error) {
	for _, l := range strings.Split(msg, "\n") {
		l = strings.TrimSpace(l)
		if !strings.HasPrefix(l, "/werft") {
			continue
		}
		l = strings.TrimPrefix(l, "/werft")
		l = strings.TrimSpace(l)

		segs := strings.Fields(l)
		if len(segs) < 1 {
			return "", nil, fmt.Errorf("cannot parse %s: missing command", l)
		}

		return segs[0], segs[1:], nil
	}

	return "", nil, nil
}

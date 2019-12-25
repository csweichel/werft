package werft

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/32leaves/werft/pkg/api/repoconfig"
	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/32leaves/werft/pkg/reporef"
	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// UIService implements api/v1/WerftUIServer
type UIService struct {
	Github *github.Client
	Repos  []string

	cache []*v1.ListJobSpecsResponse
	mu    sync.RWMutex
}

// NewUIService produces a new UI service and initializes its repo list
func NewUIService(gh *github.Client, repos []string) (*UIService, error) {
	r := &UIService{
		Github: gh,
		Repos:  repos,
	}
	err := r.updateJobSpecs()
	if err != nil {
		return nil, err
	}
	return r, nil
}

// updateJobSpecs updates the cached job spec responses by looking into the configured repositories
func (uis *UIService) updateJobSpecs() error {
	uis.mu.Lock()
	defer uis.mu.Unlock()
	uis.cache = nil

	for _, r := range uis.Repos {
		repo, err := reporef.Parse(r)
		if err != nil {
			log.WithError(err).WithField("repo", r).Warn("unable to download job spec while updating UI")
			continue
		}
		if repo.Ref != "" && repo.Revision == "" {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			repo.Revision, _, err = uis.Github.Repositories.GetCommitSHA1(ctx, repo.Owner, repo.Repo, repo.Ref, "")
			cancel()

			if err != nil {
				log.WithError(err).WithField("repo", r).Warn("cannot resolve ref to revision")
				continue
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		_, dc, _, err := uis.Github.Repositories.GetContents(ctx, repo.Owner, repo.Repo, ".werft", &github.RepositoryContentGetOptions{
			Ref: repo.Revision,
		})
		cancel()
		if err != nil {
			log.WithError(err).WithField("repo", repo).Warn("unable to download job spec while updating UI")
			continue
		}

		for _, f := range dc {
			if f.GetType() != "file" {
				continue
			}

			fn := f.GetName()
			if !strings.HasSuffix(fn, "yaml") || f.GetPath() == PathWerftConfig {
				continue
			}
			jobName := strings.TrimSuffix(fn, filepath.Ext(fn))

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			fc, err := uis.Github.Repositories.DownloadContents(ctx, repo.Owner, repo.Repo, f.GetPath(), &github.RepositoryContentGetOptions{
				Ref: repo.Revision,
			})
			cancel()
			if err != nil {
				log.WithError(err).WithField("repo", repo).WithField("path", f.GetPath()).Warn("unable to download job spec while updating UI")
				continue
			}

			var jobspec repoconfig.JobSpec
			err = yaml.NewDecoder(fc).Decode(&jobspec)
			fc.Close()
			if err != nil {
				log.WithError(err).WithField("repo", repo).WithField("path", f.GetPath()).Warn("unable to unmarshal job spec while updating UI")
				continue
			}

			var args []*v1.DesiredAnnotation
			for _, arg := range jobspec.Args {
				args = append(args, &v1.DesiredAnnotation{
					Name:        arg.Name,
					Required:    arg.Req,
					Description: arg.Desc,
				})
			}

			res := &v1.ListJobSpecsResponse{
				Repo: &v1.Repository{
					Host:     "github.com",
					Owner:    repo.Owner,
					Repo:     repo.Repo,
					Ref:      repo.Ref,
					Revision: repo.Revision,
				},
				Name:        jobName,
				Description: jobspec.Desc,
				Arguments:   args,
			}
			uis.cache = append(uis.cache, res)
		}
	}

	return nil
}

// ListJobSpecs returns a list of jobs that can be started through the UI.
func (uis *UIService) ListJobSpecs(req *v1.ListJobSpecsRequest, srv v1.WerftUI_ListJobSpecsServer) error {
	uis.mu.RLock()
	defer uis.mu.RUnlock()

	for _, r := range uis.cache {
		err := srv.Send(r)
		if err != nil {
			return err
		}
	}

	return nil
}

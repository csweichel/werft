package werft

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/csweichel/werft/pkg/api/repoconfig"
	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/csweichel/werft/pkg/reporef"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// UIService implements api/v1/WerftUIServer
type UIService struct {
	RepositoryProvider RepositoryProvider
	Repos    []string
	Readonly bool

	cache []*v1.ListJobSpecsResponse
	mu    sync.RWMutex
}

// NewUIService produces a new UI service and initializes its repo list
func NewUIService(repoprov RepositoryProvider, repos []string, readonly bool) (*UIService, error) {
	r := &UIService{
		RepositoryProvider:   repoprov,
		Repos:    repos,
		Readonly: readonly,
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

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		err = uis.RepositoryProvider.Resolve(ctx, repo)
		if err != nil {
			cancel()
			log.WithError(err).WithField("repo", r).Warn("unable to download job spec while updating UI")
			continue
		}

		fp, err := uis.RepositoryProvider.FileProvider(ctx, repo)
		if err != nil {
			cancel()
			log.WithError(err).WithField("repo", r).Warn("unable to download job spec while updating UI")
			continue
		}
		paths, err := fp.ListFiles(ctx, ".werft")
		if err != nil {
			cancel()
			log.WithError(err).WithField("repo", r).Warn("unable to download job spec while updating UI")
			continue
		}
		cancel()

		for _, fn := range paths {
			if !strings.HasSuffix(fn, "yaml") || fn == PathWerftConfig {
				continue
			}
			jobName := strings.TrimSuffix(fn, filepath.Ext(fn))

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			fc, err := fp.Download(ctx, fn)
			cancel()
			if err != nil {
				log.WithError(err).WithField("repo", repo).WithField("path", fn).Warn("unable to download job spec while updating UI")
				continue
			}

			var jobspec repoconfig.JobSpec
			err = yaml.NewDecoder(fc).Decode(&jobspec)
			fc.Close()
			if err != nil {
				log.WithError(err).WithField("repo", repo).WithField("path", fn).Warn("unable to unmarshal job spec while updating UI")
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
				Path:        fn,
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

// IsReadOnly returns true if the UI is readonly.
func (uis *UIService) IsReadOnly(context.Context, *v1.IsReadOnlyRequest) (*v1.IsReadOnlyResponse, error) {
	return &v1.IsReadOnlyResponse{
		Readonly: uis.Readonly,
	}, nil
}

package host

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"io"
	"io/ioutil"
	"sync"
	"time"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/csweichel/werft/pkg/plugin/common"
	"github.com/csweichel/werft/pkg/werft"

	corev1 "k8s.io/api/core/v1"
)

type compoundRepositoryProvider struct {
	hosts map[string]werft.RepositoryProvider
	mu    sync.RWMutex
}

var errNoRepoProvider = errors.New("no host provider")

func (c *compoundRepositoryProvider) getProvider(host string) (werft.RepositoryProvider, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	res, ok := c.hosts[host]
	if !ok {
		return nil, errNoRepoProvider
	}
	return res, nil
}

func (c *compoundRepositoryProvider) registerProvider(host string, prov werft.RepositoryProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.hosts == nil {
		c.hosts = make(map[string]werft.RepositoryProvider)
	}

	c.hosts[host] = prov
}

// Resolve resolves the repo's revision based on its ref(erence).
// If the revision is already set, this operation does nothing.
func (c *compoundRepositoryProvider) Resolve(ctx context.Context, repo *v1.Repository) error {
	prov, err := c.getProvider(repo.Host)
	if err != nil {
		return err
	}
	return prov.Resolve(ctx, repo)
}

// RemoteAnnotations extracts werft annotations form information associated
// with a particular commit, e.g. the commit message, PRs or merge requests.
// Implementors can expect the revision of the repo object to be set.
func (c *compoundRepositoryProvider) RemoteAnnotations(ctx context.Context, repo *v1.Repository) (annotations map[string]string, err error) {
	prov, err := c.getProvider(repo.Host)
	if err != nil {
		return nil, err
	}
	return prov.RemoteAnnotations(ctx, repo)
}

// ContentProvider produces a content provider for a particular repo
func (c *compoundRepositoryProvider) ContentProvider(ctx context.Context, repo *v1.Repository) (werft.ContentProvider, error) {
	prov, err := c.getProvider(repo.Host)
	if err != nil {
		return nil, err
	}
	return prov.ContentProvider(ctx, repo)
}

// FileProvider provides direct access to repository content
func (c *compoundRepositoryProvider) FileProvider(ctx context.Context, repo *v1.Repository) (werft.FileProvider, error) {
	prov, err := c.getProvider(repo.Host)
	if err != nil {
		return nil, err
	}
	return prov.FileProvider(ctx, repo)
}

type pluginHostProvider struct {
	C common.RepositoryPluginClient
}

var _ werft.RepositoryProvider = &pluginHostProvider{}

// Resolve resolves the repo's revision based on its ref(erence).
// If the revision is already set, this operation does nothing.
func (p *pluginHostProvider) Resolve(ctx context.Context, repo *v1.Repository) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := p.C.Resolve(ctx, &common.ResolveRequest{
		Repository: repo,
	})
	if err != nil {
		return err
	}
	*repo = *resp.Repository
	return nil
}

// RemoteAnnotations extracts werft annotations form information associated
// with a particular commit, e.g. the commit message, PRs or merge requests.
func (p *pluginHostProvider) RemoteAnnotations(ctx context.Context, repo *v1.Repository) (annotations map[string]string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := p.C.GetRemoteAnnotations(ctx, &common.GetRemoteAnnotationsRequest{Repository: repo})
	if err != nil {
		return nil, err
	}

	return resp.Annotations, nil
}

// ContentProvider produces a content provider for a particular repo
func (p *pluginHostProvider) ContentProvider(ctx context.Context, repo *v1.Repository) (werft.ContentProvider, error) {
	return &pluginContentProvider{
		Repo: repo,
		C:    p.C,
	}, nil
}

// FileProvider provides direct access to repository content
func (p *pluginHostProvider) FileProvider(ctx context.Context, repo *v1.Repository) (werft.FileProvider, error) {
	return &pluginContentProvider{
		Repo: repo,
		C:    p.C,
	}, nil
}

type pluginContentProvider struct {
	Repo *v1.Repository
	C    common.RepositoryPluginClient
}

func (c *pluginContentProvider) InitContainer() (res []corev1.Container, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.C.ContentInitContainer(ctx, &common.ContentInitContainerRequest{
		Repository: c.Repo,
	})
	if err != nil {
		return nil, err
	}

	err = gob.NewDecoder(bytes.NewReader(resp.Container)).Decode(&res)
	return
}

func (c *pluginContentProvider) Serve(jobName string) error {
	return nil
}

func (c *pluginContentProvider) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.C.Download(ctx, &common.DownloadRequest{
		Repository: c.Repo,
		Path:       path,
	})
	if err != nil {
		return nil, err
	}

	return ioutil.NopCloser(bytes.NewReader(resp.Content)), nil
}

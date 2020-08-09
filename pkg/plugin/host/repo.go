package host

import (
	"bytes"
	"io/ioutil"
	"context"
	"encoding/gob"
	"io"
	"time"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/csweichel/werft/pkg/plugin/common"
	"github.com/csweichel/werft/pkg/werft"

	corev1 "k8s.io/api/core/v1"
)

type pluginHostProvider struct {
	C common.RepositoryPluginClient
}

var _ werft.RepositoryProvider = &pluginHostProvider{}

// Resolve resolves the repo's revision based on its ref(erence).
// If the revision is already set, this operation does nothing.
func (p *pluginHostProvider) Resolve(repo *v1.Repository) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

// ContentProvider produces a content provider for a particular repo
func (p *pluginHostProvider) ContentProvider(repo *v1.Repository) (werft.ContentProvider, error) {
	return &pluginContentProvider{
		Repo: repo,
		C: p.C,
	}, nil
}

// FileProvider provides direct access to repository content
func (p *pluginHostProvider) FileProvider(repo *v1.Repository) (werft.FileProvider, error) {
	return &pluginContentProvider{
		Repo: repo,
		C: p.C,
	}, nil
}

type pluginContentProvider struct {
	Repo *v1.Repository
	C common.RepositoryPluginClient
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
		Path: path,
	})
	if err != nil {
		return nil, err
	}

	return ioutil.NopCloser(bytes.NewReader(resp.Content)), nil
}
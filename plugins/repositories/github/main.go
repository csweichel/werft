package main

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	plugin "github.com/csweichel/werft/pkg/plugin/client"
	"github.com/csweichel/werft/pkg/plugin/common"
	"github.com/csweichel/werft/repos/github/pkg/provider"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v31/github"
)

// Config configures this plugin
type Config struct {
	PrivateKeyPath string `yaml:"privateKeyPath"`
	InstallationID int64  `yaml:"installationID,omitempty"`
	AppID          int64  `yaml:"appID"`
}

func main() {
	plugin.Serve(&Config{},
		plugin.WithRepositoryPlugin(&githubRepoPlugin{}),
	)
}

type githubRepoPlugin struct{}

func (*githubRepoPlugin) Run(ctx context.Context, config interface{}) (common.RepositoryPluginServer, error) {
	cfg, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("config has wrong type %s", reflect.TypeOf(config))
	}

	ghtr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, cfg.AppID, cfg.InstallationID, cfg.PrivateKeyPath)
	if err != nil {
		return nil, err
	}
	ghClient := github.NewClient(&http.Client{Transport: ghtr})

	return &provider.GithubRepoServer{
		Client: ghClient,
		Auth: func(ctx context.Context) (user string, pass string, err error) {
			tkn, err := ghtr.Token(ctx)
			if err != nil {
				return
			}
			user = "x-access-token"
			pass = tkn
			return
		},
	}, nil
}

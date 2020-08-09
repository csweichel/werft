package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"

	plugin "github.com/csweichel/werft/pkg/plugin/client"
	"github.com/csweichel/werft/pkg/plugin/common"
	"github.com/csweichel/werft/pkg/werft"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v31/github"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
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

	return &githubRepoServer{
		Config: cfg,
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

type githubRepoServer struct {
	Config *Config
	Client *github.Client
	Auth   werft.GitCredentialHelper
}

func (s *githubRepoServer) RepoHost(context.Context, *common.RepoHostRequest) (*common.RepoHostResponse, error) {
	return &common.RepoHostResponse{
		Host: "github.com",
	}, nil
}

// Resolve resolves the repo's revision based on its ref(erence)
func (s *githubRepoServer) Resolve(ctx context.Context, req *common.ResolveRequest) (*common.ResolveResponse, error) {
	repo := req.Repository
	if repo.Revision != "" {
		return &common.ResolveResponse{Repository: repo}, nil
	}
	if repo.Ref == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ref is empty")
	}

	branch, _, err := s.Client.Repositories.GetBranch(ctx, repo.Owner, repo.Repo, repo.Ref)
	if err != nil {
		return nil, translateGitHubToGRPCError(err, repo.Revision, repo.Ref)
	}
	if branch == nil {
		return nil, status.Error(codes.NotFound, "did not find ref")
	}
	if branch.Commit == nil || branch.Commit.SHA == nil {
		return nil, status.Error(codes.NotFound, "ref did not point to a commit")
	}
	repo.Revision = *branch.Commit.SHA
	log.WithField("ref", repo.Ref).WithField("rev", repo.Revision).Debug("resolved reference to revision")

	return &common.ResolveResponse{
		Repository: repo,
	}, nil
}

func (s *githubRepoServer) ContentInitContainer(ctx context.Context, req *common.ContentInitContainerRequest) (*common.ContentInitContainerResponse, error) {
	var (
		repo = req.Repository
		user string
		pass string
	)
	if s.Auth != nil {
		var err error
		user, pass, err = s.Auth(context.Background())
		if err != nil {
			return nil, err
		}
	}

	cloneCmd := "git clone"
	if user != "" || pass != "" {
		cloneCmd = fmt.Sprintf("git clone -c \"credential.helper=/bin/sh -c 'echo username=$GHUSER_SECRET; echo password=$GHPASS_SECRET'\"")
	}
	cloneCmd = fmt.Sprintf("%s https://github.com/%s/%s.git .; git checkout %s", cloneCmd, repo.Owner, repo.Repo, repo.Revision)

	c := []corev1.Container{
		{
			Name:  "github-checkout",
			Image: "alpine/git:latest",
			Command: []string{
				"sh", "-c",
				cloneCmd,
			},
			Env: []corev1.EnvVar{
				corev1.EnvVar{
					Name:  "GHUSER_SECRET",
					Value: user,
				},
				corev1.EnvVar{
					Name:  "GHPASS_SECRET",
					Value: pass,
				},
			},
			WorkingDir: "/workspace",
		},
	}
	buf := bytes.NewBuffer(nil)
	err := gob.NewEncoder(buf).Encode(c)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cannot serialize init container: %q", err)
	}

	return &common.ContentInitContainerResponse{
		Container: buf.Bytes(),
	}, nil
}

func (s *githubRepoServer) Download(ctx context.Context, req *common.DownloadRequest) (*common.DownloadResponse, error) {
	dl, err := s.Client.Repositories.DownloadContents(ctx, req.Repository.Owner, req.Repository.Repo, req.Path, &github.RepositoryContentGetOptions{
		Ref: req.Repository.Revision,
	})
	if err != nil {
		return nil, translateGitHubToGRPCError(err, req.Repository.Revision, req.Repository.Ref)
	}
	defer dl.Close()

	res, err := ioutil.ReadAll(dl)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cannot download file: %q", err)
	}
	return &common.DownloadResponse{
		Content: res,
	}, nil
}

func translateGitHubToGRPCError(err error, rev, ref string) error {
	if gherr, ok := err.(*github.ErrorResponse); ok && gherr.Response.StatusCode == 422 {
		msg := fmt.Sprintf("revision %s", rev)
		if ref != "" {
			msg = fmt.Sprintf("ref %s (revision %s)", ref, rev)
		}
		return status.Error(codes.NotFound, fmt.Sprintf("%s not found", msg))
	}

	return status.Error(codes.Internal, err.Error())
}

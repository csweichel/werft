package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strconv"

	plugin "github.com/csweichel/werft/pkg/plugin/client"
	"github.com/csweichel/werft/pkg/plugin/common"
	"golang.org/x/oauth2"

	"github.com/google/go-github/v43/github"
)

// Config configures this plugin
type Config struct {
	EnableTeams  bool `yaml:"enableTeams,omitempty"`
	EnableEmails bool `yaml:"enableEmails,omitempty"`
}

func main() {
	plugin.Serve(&Config{},
		plugin.WithAuthenticationPlugin(&githubAuthPlugin{}),
	)
	fmt.Fprintln(os.Stderr, "shutting down")
}

type githubAuthPlugin struct{}

func (*githubAuthPlugin) Run(ctx context.Context, config interface{}) (common.AuthenticationPluginServer, error) {
	cfg, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("config has wrong type %s", reflect.TypeOf(config))
	}

	return &authServer{Config: cfg}, nil
}

type authServer struct {
	Config *Config
}

func (as *authServer) Authenticate(ctx context.Context, req *common.AuthenticateRequest) (*common.AuthenticateResponse, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: req.Token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	ghreq, err := client.NewRequest("GET", "user", nil)
	if err != nil {
		return nil, err
	}

	user := new(github.User)
	resp, err := client.Do(ctx, ghreq, user)
	if resp.StatusCode == http.StatusUnauthorized {
		return &common.AuthenticateResponse{Known: false}, nil
	}
	if err != nil {
		return nil, err
	}

	var emails []string
	if as.Config.EnableEmails {
		rawEmails, _, _ := client.Users.ListEmails(ctx, nil)
		for _, e := range rawEmails {
			if !e.GetVerified() {
				continue
			}

			emails = append(emails, e.GetEmail())
		}
	}

	var teams []string
	if as.Config.EnableTeams {
		rawTeams, _, _ := client.Teams.ListUserTeams(ctx, nil)
		for _, t := range rawTeams {
			teams = append(teams, t.GetURL())
		}
	}

	return &common.AuthenticateResponse{
		Known:    true,
		Username: user.GetLogin(),
		Metadata: map[string]string{
			"two-factor-authentication": strconv.FormatBool(user.GetTwoFactorAuthentication()),
			"name":                      user.GetName(),
		},
		Emails: emails,
		Teams:  teams,
	}, nil
}

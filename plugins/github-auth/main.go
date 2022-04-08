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
type Config struct{}

func main() {
	plugin.Serve(&Config{},
		plugin.WithAuthenticationPlugin(&githubAuthPlugin{}),
	)
	fmt.Fprintln(os.Stderr, "shutting down")
}

type githubAuthPlugin struct{}

func (*githubAuthPlugin) Run(ctx context.Context, config interface{}) (common.AuthenticationPluginServer, error) {
	_, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("config has wrong type %s", reflect.TypeOf(config))
	}

	return &authServer{}, nil
}

type authServer struct{}

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

	return &common.AuthenticateResponse{
		Known:    true,
		Username: user.GetLogin(),
		Metadata: map[string]string{
			"two-factor-authentication": strconv.FormatBool(user.GetTwoFactorAuthentication()),
			"email":                     user.GetEmail(),
		},
	}, nil
}

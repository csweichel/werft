package main

import (
	"context"
	"fmt"
	"reflect"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	plugin "github.com/csweichel/werft/pkg/plugin/client"
	log "github.com/sirupsen/logrus"
)

// Config configures this plugin
type Config struct {
	Emoji string `yaml:"emoji"`
}

func main() {
	plugin.Serve(&Config{},
		plugin.WithIntegrationPlugin(&integrationPlugin{}),
	)
}

type integrationPlugin struct{}

func (*integrationPlugin) Run(ctx context.Context, config interface{}, srv v1.WerftServiceClient) error {
	cfg, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("config has wrong type %s", reflect.TypeOf(config))
	}

	sub, err := srv.Subscribe(ctx, &v1.SubscribeRequest{})
	if err != nil {
		return err
	}

	log.Infof("hello world %s", cfg.Emoji)
	for {
		resp, err := sub.Recv()
		if err != nil {
			return err
		}

		fmt.Printf("%s %v\n", cfg.Emoji, resp)
	}
}

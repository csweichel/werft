package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	plugin "github.com/csweichel/werft/pkg/plugin/client"
	"github.com/csweichel/werft/pkg/reporef"
	cron "github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
)

// Config configures this plugin
type Config struct {
	Tasks []struct {
		Spec        string            `yaml:"spec"`
		Repo        string            `yaml:"repo"`
		JobPath     string            `yaml:"jobPath,omitempty"`
		Trigger     string            `yaml:"trigger,omitempty"`
		Annotations map[string]string `yaml:"annotations,omitempty"`
	} `yaml:"tasks"`
}

func main() {
	plugin.Serve(&Config{},
		plugin.WithIntegrationPlugin(&cronPlugin{}),
	)
}

type cronPlugin struct{}

func (*cronPlugin) Run(ctx context.Context, config interface{}, srv v1.WerftServiceClient) error {
	cfg, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("config has wrong type %s", reflect.TypeOf(config))
	}

	c := cron.New()
	for idx, task := range cfg.Tasks {
		repo, err := reporef.Parse(task.Repo)
		if err != nil {
			return err
		}

		var trigger v1.JobTrigger
		if trg, ok := v1.JobTrigger_value[fmt.Sprintf("TRIGGER_%s", strings.ToUpper(task.Trigger))]; ok {
			trigger = v1.JobTrigger(trg)
		} else if task.Trigger != "" {
			return fmt.Errorf("unknown job trigger %s", task.Trigger)
		}

		var annotations []*v1.Annotation
		for k, v := range task.Annotations {
			annotations = append(annotations, &v1.Annotation{
				Key:   k,
				Value: v,
			})
		}

		_, err = c.AddFunc(task.Spec, func() {
			_, err := srv.StartGitHubJob(ctx, &v1.StartGitHubJobRequest{
				Metadata: &v1.JobMetadata{
					Owner:       "cron",
					Annotations: annotations,
					Trigger:     trigger,
					Repository:  repo,
				},
				JobPath: task.JobPath,
			})
			if err != nil {
				log.WithError(err).WithField("idx", idx).WithField("spec", task.Spec).Error("cannot start job")
			}
		})
		if err != nil {
			return err
		}

		log.WithField("spec", task.Spec).Info("scheduled job")
	}
	c.Run()

	return nil
}

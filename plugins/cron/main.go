package main

import (
	"context"
	"fmt"
	"io"
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

func (*cronPlugin) Run(ctx context.Context, config interface{}, srv *plugin.Services) error {
	cfg, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("config has wrong type %s", reflect.TypeOf(config))
	}

	c := cron.New()

	remoteJobs := make(map[string]cron.EntryID)
	_, err := c.AddFunc("* * * * *", func() {
		defer recover()

		err := func() error {
			log.Info("refreshing job specs")

			foundJobs := make(map[string]struct{})
			specs, err := srv.ListJobSpecs(context.Background(), &v1.ListJobSpecsRequest{})
			if err != nil {
				return err
			}
			for {
				spec, err := specs.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				if spec == nil {
					break
				}
				id := strings.Join([]string{spec.Repo.Host, spec.Repo.Owner, spec.Repo.Repo, spec.Repo.Ref, spec.Name}, "/")
				foundJobs[id] = struct{}{}
				if _, exists := remoteJobs[id]; exists {
					continue
				}

				log := log.WithField("repo", spec.Repo).WithField("id", id)
				expr, ok := spec.Plugins["cron"]
				if !ok {
					log.Debug("has no cron expression - ignoring")
					continue
				}
				cronSpec := strings.TrimPrefix(strings.TrimSuffix(expr, `"`), `"`)
				log = log.WithField("cron", cronSpec)

				request := &v1.StartGitHubJobRequest{
					Metadata: &v1.JobMetadata{
						Owner:      "cron",
						Trigger:    v1.JobTrigger_TRIGGER_MANUAL,
						Repository: spec.Repo,
					},
					JobPath: spec.Path,
				}
				request.Metadata.Repository.Revision = ""
				entryID, err := c.AddFunc(cronSpec, func() {
					_, err := srv.StartGitHubJob(ctx, request)
					if err != nil {
						log.WithError(err).Error("cannot start job")
					}
				})
				if err != nil {
					log.WithError(err).Error("cannot schedule cron job")
					continue
				}
				remoteJobs[id] = entryID
				log.Info("scheduled new job")
			}
			for k, v := range remoteJobs {
				if _, ok := foundJobs[k]; ok {
					continue
				}

				c.Remove(v)
				log.WithField("id", k).Info("stopping job")
			}
			return nil
		}()
		if err != nil {
			log.WithError(err).Error("error while updating cron jobs")
		}
	})
	if err != nil {
		return err
	}

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

		request := &v1.StartGitHubJobRequest{
			Metadata: &v1.JobMetadata{
				Owner:       "cron",
				Annotations: annotations,
				Trigger:     trigger,
				Repository:  repo,
			},
			JobPath: task.JobPath,
		}
		request.Metadata.Repository.Revision = ""
		_, err = c.AddFunc(task.Spec, func() {
			_, err := srv.StartGitHubJob(ctx, request)
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

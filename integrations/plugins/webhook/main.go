package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"reflect"
	"sync"
	"time"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/csweichel/werft/pkg/filterexpr"
	plugin "github.com/csweichel/werft/pkg/plugin/client"
	log "github.com/sirupsen/logrus"
)

// Config configures this plugin
type Config struct {
	Notifications []struct {
		WebhookURL  string   `yaml:"url"`
		Filter      []string `yaml:"filter"`
		Template    string   `yaml:"template"`
		ContentType string   `yaml:"contentType"`
	} `yaml:"notifications"`
}

func main() {
	plugin.Serve(&Config{},
		plugin.WithIntegrationPlugin(&webhookPlugin{}),
	)
}

type webhookPlugin struct{}

func (*webhookPlugin) Run(ctx context.Context, config interface{}, srv v1.WerftServiceClient) error {
	cfg, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("config has wrong type %s", reflect.TypeOf(config))
	}

	var wg sync.WaitGroup
	for idx, nf := range cfg.Notifications {
		filter, err := filterexpr.Parse(nf.Filter)
		if err != nil {
			log.WithError(err).Errorf("cannot parse filter for notification %d", idx)
		}

		tpl, err := template.New("tpl").Parse(nf.Template)
		if err != nil {
			log.WithError(err).Errorf("cannot parse template for notification %d", idx)
		}

		wg.Add(1)
		go func(idx int, url string, contentType string, tpl *template.Template) {
			defer wg.Done()

			sub, err := srv.Subscribe(ctx, &v1.SubscribeRequest{
				Filter: []*v1.FilterExpression{&v1.FilterExpression{Terms: filter}},
			})
			if err != nil {
				log.WithError(err).Errorf("cannot subscribe for notification %d", idx)
				return
			}
			log.Infof("notifications for %s set up", url)

			for {
				resp, err := sub.Recv()
				if err != nil {
					log.WithError(err).Errorf("subscription error with notification %d", idx)
					return
				}

				buf := bytes.NewBuffer(nil)
				err = tpl.Execute(buf, resp.Result)
				if err != nil {
					log.WithError(err).Warnf("template error with notification %d", idx)
					continue
				}

				err = sendNotification(url, contentType, buf)
				if err != nil {
					log.WithError(err).Warnf("send error with notification %d", idx)
					continue
				}
			}
		}(idx, nf.WebhookURL, nf.ContentType, tpl)
	}

	wg.Wait()
	return nil
}

// sendNotification will post text to a URL
func sendNotification(webhookURL string, contentType string, body io.Reader) error {
	log.WithField("url", webhookURL).Info("sending message")

	req, err := http.NewRequest(http.MethodPost, webhookURL, body)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", contentType)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	if buf.String() != "ok" {
		return fmt.Errorf("non-ok response")
	}
	return nil
}

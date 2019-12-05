package cmd

// Copyright © 2019 Christian Weichel

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/32leaves/keel/pkg/executor"
	"github.com/32leaves/keel/pkg/keel"
	"github.com/32leaves/keel/pkg/logcutter"
	"github.com/32leaves/keel/pkg/store"
	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run <config.json>",
	Short: "Starts the keel server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fc, err := ioutil.ReadFile(args[0])
		if err != nil {
			return err
		}

		var cfg Config
		err = json.Unmarshal(fc, &cfg)
		if err != nil {
			return err
		}

		var kubeConfig *rest.Config
		if cfg.Kubernetes.Kubeconfig == "" {
			kubeConfig, err = rest.InClusterConfig()
			if err != nil {
				return err
			}
		} else {
			kubeConfig, err = clientcmd.BuildConfigFromFlags("", cfg.Kubernetes.Kubeconfig)
			if err != nil {
				return err
			}
		}

		ghtr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, cfg.GitHub.AppID, cfg.GitHub.InstallationID, cfg.GitHub.PrivateKeyPath)
		if err != nil {
			return err
		}
		ghClient := github.NewClient(&http.Client{Transport: ghtr})

		execCfg := executor.Config{
			Namespace: cfg.Kubernetes.Namespace,
		}
		if execCfg.Namespace == "" {
			execCfg.Namespace = "default"
		}

		jobStore := store.NewInMemoryJobStore()
		exec, err := executor.NewExecutor(execCfg, kubeConfig)
		if err != nil {
			return err
		}
		exec.Run()
		service := &keel.Service{
			Logs:     store.NewInMemoryLogStore(),
			Jobs:     jobStore,
			Executor: exec,
			Cutter:   logcutter.DefaultCutter,
			GitHub: keel.GitHubSetup{
				WebhookSecret: []byte(cfg.GitHub.WebhookSecret),
				Client:        ghClient,
			},
		}
		if val, _ := cmd.Flags().GetString("debug-webui-proxy"); val != "" {
			service.DebugProxy = val
		}
		service.Start()
		go service.StartWeb(fmt.Sprintf(":%d", cfg.Service.WebPort))
		go service.StartGRPC(fmt.Sprintf(":%d", cfg.Service.GRPCPort))

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		log.Info("keel is up and running. Stop with SIGINT or CTRL+C")
		<-sigChan
		log.Info("Received SIGINT - shutting down")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().String("debug-webui-proxy", "", "proxies the web UI to this address")
}

// Config configures the keel server
type Config struct {
	Service struct {
		WebPort  int `json:"webPort"`
		GRPCPort int `json:"grpcPort"`
	}
	Kubernetes struct {
		Kubeconfig string `json:"kubeconfig,omitempty"`
		Namespace  string `json:"namespace,omitempty"`
	} `json:"kubernetes,omitempty"`
	GitHub struct {
		WebhookSecret  string `json:"webhookSecret"`
		PrivateKeyPath string `json:"privateKeyPath"`
		InstallationID int64  `json:"installationID,omitempty"`
		AppID          int64  `json:"appID"`
	} `json:"github"`
}

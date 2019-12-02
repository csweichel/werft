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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	v1 "github.com/32leaves/keel/pkg/api/v1"
	"github.com/32leaves/keel/pkg/executor"
	"github.com/32leaves/keel/pkg/keel"
	"github.com/32leaves/keel/pkg/store"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start runs a keel job in the working directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
		if kubeconfig == "" {
			return fmt.Errorf("--kubeconfig is mandatory")
		}
		res, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return err
		}
		kubeClient, err := kubernetes.NewForConfig(res)
		if err != nil {
			return err
		}

		ns, _ := cmd.Flags().GetString("namespace")
		wd, _ := cmd.Flags().GetString("cwd")
		trigger, _ := cmd.Flags().GetString("trigger")
		jc, err := getLocalJobContext(wd, keel.JobTrigger(trigger))
		if err != nil {
			return err
		}

		content := &keel.LocalContentProvider{
			BasePath:   wd,
			Namespace:  ns,
			Kubeconfig: res,
			Clientset:  kubeClient,
		}

		executor := executor.NewExecutor(executor.Config{
			Namespace:     ns,
			EventTraceLog: "/tmp/evts.json",
		}, kubeClient)
		upchan, errchan := make(chan *v1.JobStatus), make(chan error)
		executor.OnUpdate = func(update *v1.JobStatus) {
			upchan <- update
		}
		executor.OnError = func(err error) {
			errchan <- err
		}
		executor.Run()

		srv := keel.Service{
			Logs:     store.NewInMemoryLogStore(),
			Jobs:     store.NewInMemoryJobStore(),
			Executor: executor,
		}

		name, err := srv.RunJob(context.Background(), *jc, content)
		if err != nil {
			return err
		}

		go func() {
			logs := executor.Logs(name)
			for l := range logs {
				fmt.Println(l)
			}
		}()

	recv:
		for {
			select {
			case update := <-upchan:
				if name != update.Name {
					continue
				}

				up, _ := json.Marshal(update)
				log.WithField("update", string(up)).Info("job update")
				if update.Phase == v1.JobPhase_PHASE_CLEANUP {
					break recv
				}
			case err := <-errchan:
				return err
			}
		}

		return nil
	},
}

func getLocalJobContext(wd string, trigger keel.JobTrigger) (*keel.JobContext, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = wd
	rev, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return &keel.JobContext{
		Owner:    "local",
		Repo:     filepath.Base(wd),
		Revision: string(rev),
		Trigger:  trigger,
	}, nil
}

func init() {
	rootCmd.AddCommand(startCmd)

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		homedir, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(homedir, ".kube", "config")
		if _, err := os.Stat(kubeconfig); err != nil {
			kubeconfig = ""
		}
	}

	wd, _ := os.Getwd()

	startCmd.Flags().String("kubeconfig", kubeconfig, "kubeconfig file")
	startCmd.Flags().String("namespace", "default", "kubernetes namespace to operate in")
	startCmd.Flags().String("trigger", "manual", "job trigger. One of push, pr, manual")
	startCmd.Flags().String("cwd", wd, "working directory")
}

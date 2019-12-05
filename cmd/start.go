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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/32leaves/werft/pkg/executor"
	"github.com/32leaves/werft/pkg/werft"
	"github.com/32leaves/werft/pkg/store"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start runs a werft job in the working directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		kubeconfigPath, _ := cmd.Flags().GetString("kubeconfig")
		if kubeconfigPath == "" {
			return fmt.Errorf("--kubeconfig is mandatory")
		}
		kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return err
		}
		kubeClient, err := kubernetes.NewForConfig(kubeConfig)
		if err != nil {
			return err
		}

		ns, _ := cmd.Flags().GetString("namespace")
		wd, _ := cmd.Flags().GetString("cwd")
		triggerName, _ := cmd.Flags().GetString("trigger")

		tv, ok := v1.JobTrigger_value[fmt.Sprintf("TRIGGER_%s", strings.ToUpper(triggerName))]
		if !ok {
			var vs []string
			for k := range v1.JobTrigger_value {
				vs = append(vs, strings.ToLower(strings.TrimPrefix("TRIGGER_", k)))
			}

			return xerrors.Errorf("Invalid value for --trigger. Valid choices are %s", strings.Join(vs, "\n"))
		}
		jc, err := getLocalJobContext(wd, v1.JobTrigger(tv))
		if err != nil {
			return err
		}

		tarcmd := exec.Command("tar", "cz", ".")
		tarcmd.Dir = wd
		tarStream, err := tarcmd.StdoutPipe()
		if err != nil {
			return err
		}
		err = tarcmd.Start()
		if err != nil {
			return err
		}

		content := &werft.LocalContentProvider{
			FileProvider: func(p string) (io.ReadCloser, error) { return os.OpenFile(filepath.Join(wd, p), os.O_RDONLY, 0644) },
			TarStream:    tarStream,
			Namespace:    ns,
			Kubeconfig:   kubeConfig,
			Clientset:    kubeClient,
		}

		executor, err := executor.NewExecutor(executor.Config{
			Namespace:     ns,
			EventTraceLog: "/tmp/evts.json",
		}, kubeConfig)
		if err != nil {
			return err
		}
		upchan, errchan := make(chan *v1.JobStatus), make(chan error)
		executor.OnUpdate = func(update *v1.JobStatus) {
			upchan <- update
		}
		executor.OnError = func(err error) {
			errchan <- err
		}
		executor.Run()

		srv := werft.Service{
			Logs:     store.NewInMemoryLogStore(),
			Jobs:     store.NewInMemoryJobStore(),
			Executor: executor,
		}

		jobStatus, err := srv.RunJob(context.Background(), *jc, content)
		if err != nil {
			return err
		}
		name := jobStatus.Name

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

func getLocalJobContext(wd string, trigger v1.JobTrigger) (*v1.JobMetadata, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = wd
	rev, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	cmd = exec.Command("git", "config", "--global", "user.name")
	user, err := cmd.Output()
	if err != nil {
		return nil, xerrors.Errorf("cannot get gloval git user: %w", err)
	}

	return &v1.JobMetadata{
		Owner: string(user),
		Repository: &v1.Repository{
			Owner: "local",
			Repo:  filepath.Base(wd),
			Ref:   string(rev),
		},
		Trigger: trigger,
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

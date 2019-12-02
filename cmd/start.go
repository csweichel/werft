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
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/32leaves/keel/pkg/executor"
	"github.com/32leaves/keel/pkg/keel"
	"github.com/32leaves/keel/pkg/store"
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

		wd, _ := cmd.Flags().GetString("cwd")
		jc, err := getLocalJobContext(wd)
		if err != nil {
			return err
		}

		fp := func(path string) (io.ReadCloser, error) {
			return os.OpenFile(path, os.O_RDONLY, 0644)
		}

		ns, _ := cmd.Flags().GetString("namespace")
		srv := keel.Service{
			Logs: store.NewInMemoryLogStore(),
			Jobs: store.NewInMemoryJobStore(),
			Executor: executor.NewExecutor(executor.Config{
				Namespace: ns,
			}, kubeClient),
		}

		trigger, _ := cmd.Flags().GetString("trigger")
		srv.RunJob(context.Background(), *jc, keel.JobTrigger(trigger), fp)

		return nil
	},
}

func getLocalJobContext(wd string) (*keel.JobContext, error) {

	return nil, nil
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

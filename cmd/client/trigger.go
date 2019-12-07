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
	"os/exec"
	"path/filepath"

	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

// triggerCmd represents the trigger command
var triggerCmd = &cobra.Command{
	Use:   "trigger",
	Short: "Triggers the execution of a job",
	Args:  cobra.MinimumNArgs(1),
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

func followJob(client v1.WerftServiceClient, name string) error {
	ctx := context.Background()
	logs, err := client.Listen(ctx, &v1.ListenRequest{
		Name: name,
		Logs: v1.ListenRequestLogs_LOGS_RAW,
	})
	if err != nil {
		return err
	}

	for {
		msg, err := logs.Recv()
		if err != nil {
			return err
		}

		fmt.Println(string(msg.GetSlice().Payload))
	}
}

func init() {
	rootCmd.AddCommand(triggerCmd)

	triggerCmd.PersistentFlags().String("host", "localhost:7777", "werft host to talk to")
	triggerCmd.PersistentFlags().String("job-file", "", "location of the job file (defaults to the default job in the werft config)")
	triggerCmd.PersistentFlags().String("config-file", "$CWD/.werft/config.yaml", "location of the werft config file")
	triggerCmd.PersistentFlags().String("trigger", "manual", "job trigger. One of push, manual")
	triggerCmd.PersistentFlags().BoolP("follow", "f", false, "follow the log output once the job is running")
}

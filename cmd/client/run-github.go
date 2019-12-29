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
	"io/ioutil"
	"strings"

	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/32leaves/werft/pkg/reporef"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

// runGithubCmd represents the triggerRemote command
var runGithubCmd = &cobra.Command{
	Use:   "github [<owner>/<repo>(:ref | @revision)]",
	Short: "starts a job from a remore repository",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Parent().PersistentFlags()
		cwd, _ := flags.GetString("cwd")

		var (
			md  *v1.JobMetadata
			err error
		)
		if len(args) == 0 {
			md, err = getLocalJobContext(cwd, v1.JobTrigger_TRIGGER_MANUAL)
		} else {
			repo, err := reporef.Parse(args[0])
			if err != nil {
				return err
			}
			md = &v1.JobMetadata{
				Owner:      repo.Owner,
				Repository: repo,
			}
		}

		triggerName, _ := flags.GetString("trigger")
		trigger, ok := v1.JobTrigger_value[fmt.Sprintf("TRIGGER_%s", strings.ToUpper(triggerName))]
		if !ok {
			var vs []string
			for k := range v1.JobTrigger_value {
				vs = append(vs, strings.ToLower(strings.TrimPrefix("TRIGGER_", k)))
			}

			return xerrors.Errorf("Invalid value for --trigger. Valid choices are %s", strings.Join(vs, "\n"))
		}
		md.Trigger = v1.JobTrigger(trigger)

		token, _ := cmd.Flags().GetString("token")
		req := &v1.StartGitHubJobRequest{
			Metadata:    md,
			GithubToken: token,
		}

		jobname, _ := cmd.Flags().GetString("job-name")
		jobPath, _ := flags.GetString("job-file")
		if jobname != "" && jobPath != "" {
			return xerrors.Errorf("cannot specify both: job name and job path")
		} else if jobname != "" {
			req.Job = &v1.StartGitHubJobRequest_JobName{
				JobName: jobname,
			}
		} else if jobPath != "" {
			fc, err := ioutil.ReadFile(jobPath)
			if err != nil {
				return err
			}

			req.Job = &v1.StartGitHubJobRequest_JobYaml{
				JobYaml: fc,
			}
		}

		conn := dial()
		defer conn.Close()
		client := v1.NewWerftServiceClient(conn)

		ctx := context.Background()
		resp, err := client.StartGitHubJob(ctx, req)
		if err != nil {
			return err
		}
		fmt.Println(resp.Status.Name)

		follow, _ := flags.GetBool("follow")
		if follow {
			err = followJob(client, resp.Status.Name)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	runCmd.AddCommand(runGithubCmd)

	runGithubCmd.Flags().String("token", "", "Token to use for authorization against GitHub")
	runGithubCmd.Flags().StringP("job-name", "j", "", "start a particular job (defaults to the default job of the repo)")
}

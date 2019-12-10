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
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
)

// triggerRemoteCmd represents the triggerRemote command
var triggerRemoteCmd = &cobra.Command{
	Use:   "github <owner>/<repo>(@revision)",
	Short: "starts a job from a remore repository",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		segs := strings.Split(args[0], "/")
		if len(segs) < 2 {
			return xerrors.Errorf("%s is not in the format owner/repo(@revision)")
		}
		owner, repo, rev := segs[0], segs[1], ""
		if strings.Contains(repo, "@") {
			segs = strings.Split(repo, "@")
			repo, rev = segs[0], segs[1]
		}

		flags := cmd.Parent().PersistentFlags()
		host, _ := flags.GetString("host")
		triggerName, _ := flags.GetString("trigger")
		trigger, ok := v1.JobTrigger_value[fmt.Sprintf("TRIGGER_%s", strings.ToUpper(triggerName))]
		if !ok {
			var vs []string
			for k := range v1.JobTrigger_value {
				vs = append(vs, strings.ToLower(strings.TrimPrefix("TRIGGER_", k)))
			}

			return xerrors.Errorf("Invalid value for --trigger. Valid choices are %s", strings.Join(vs, "\n"))
		}

		username, _ := flags.GetString("username")
		password, _ := flags.GetString("password")
		req := &v1.StartGitHubJobRequest{
			Job: &v1.JobMetadata{
				Owner: owner,
				Repository: &v1.Repository{
					Host:     "github.com",
					Owner:    owner,
					Repo:     repo,
					Revision: rev,
				},
				Trigger: v1.JobTrigger(trigger),
			},
			Username: username,
			Password: password,
		}

		jobPath, _ := flags.GetString("job-file")
		if jobPath != "" {
			fc, err := ioutil.ReadFile(jobPath)
			if err != nil {
				return err
			}

			req.JobYaml = fc
		}

		ctx := context.Background()
		conn, err := grpc.Dial(host, grpc.WithInsecure())
		if err != nil {
			return err
		}

		defer conn.Close()
		client := v1.NewWerftServiceClient(conn)
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
	triggerCmd.AddCommand(triggerRemoteCmd)

	triggerRemoteCmd.Flags().String("username", "", "username to use as authorization")
	triggerRemoteCmd.Flags().String("password", "", "password to use as authorization")
}

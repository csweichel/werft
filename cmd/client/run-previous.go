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

	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/golang/protobuf/ptypes"
	"github.com/spf13/cobra"
)

// runPreviousJobCmd represents the triggerRemote command
var runPreviousJobCmd = &cobra.Command{
	Use:   "previous [old-job-name]",
	Short: "starts a job from a previous one",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Parent().PersistentFlags()

		annotations, _ := flags.GetStringToString("annotations")
		if len(annotations) > 0 {
			return fmt.Errorf("--annotation is not supported when replaying a previous job")
		}

		conn := dial()
		defer conn.Close()
		client := v1.NewWerftServiceClient(conn)
		ctx := context.Background()

		var (
			name string
			err  error
		)
		if len(args) == 0 {
			name, err = findJobByLocalContext(ctx, client)
			if err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("no job found - please specify job name")
			}

			fmt.Printf("re-running \033[34m\033[1m%s\t\033\033[0m\n", name)
		} else {
			name = args[0]
		}

		token, _ := cmd.Flags().GetString("token")
		req := &v1.StartFromPreviousJobRequest{
			PreviousJob: name,
			GithubToken: token,
		}

		waitUntil, err := getWaitUntil()
		if err != nil {
			return err
		}
		if waitUntil != nil {
			req.WaitUntil, err = ptypes.TimestampProto(*waitUntil)
			if err != nil {
				return err
			}
		}

		resp, err := client.StartFromPreviousJob(ctx, req)
		if err != nil {
			return err
		}
		fmt.Println(resp.Status.Name)

		follow, _ := flags.GetBool("follow")
		withPrefix, _ := flags.GetString("follow-with-prefix")
		if follow || withPrefix != "" {
			err = followJob(client, resp.Status.Name, withPrefix)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	runCmd.AddCommand(runPreviousJobCmd)

	runPreviousJobCmd.Flags().String("token", "", "Token to use for authorization against GitHub")
}

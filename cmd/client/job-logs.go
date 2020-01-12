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
	"os"

	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

// jobLogsCmd represents the list command
var jobLogsCmd = &cobra.Command{
	Use:   "logs [name]",
	Short: "Listens to the log output of a job",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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
				return xerrors.Errorf("no job found - please specify job name")
			}

			fmt.Printf("showing logs of \033[34m\033[1m%s\t\033\033[0m\n", name)
		} else {
			name = args[0]
		}

		resp, err := client.Listen(ctx, &v1.ListenRequest{
			Name:    name,
			Logs:    v1.ListenRequestLogs_LOGS_RAW,
			Updates: true,
		})
		if err != nil {
			return err
		}

		for {
			msg, err := resp.Recv()
			if err != nil {
				return err
			}
			if msg == nil {
				return nil
			}

			update := msg.GetUpdate()
			if update != nil && update.Phase == v1.JobPhase_PHASE_DONE {
				if !update.Conditions.Success {
					os.Exit(-1)
				}

				return nil
			}
			if update != nil {
				continue
			}

			pringLogSlice(msg.GetSlice())
		}
	},
}

func pringLogSlice(slice *v1.LogSliceEvent) {
	if slice.Name == "werft:kubernetes" || slice.Name == "werft:status" {
		return
	}

	var tpl string
	switch slice.Type {
	case v1.LogSliceType_SLICE_PHASE:
		tpl = "\033[33m\033[1m{{ .Name }}\t\033[39m{{ .Payload }}\033[0m\n"
	case v1.LogSliceType_SLICE_CONTENT:
		tpl = "\033[2m[{{ .Name }}]\033[0m {{ .Payload }}\n"
	}
	if tpl == "" {
		return
	}
	prettyPrint(slice, tpl)
}

func init() {
	jobCmd.AddCommand(jobLogsCmd)
}

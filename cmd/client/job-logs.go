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

	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/spf13/cobra"
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

		name, localJobContext, err := getLocalJobName(client, args)
		if err != nil {
			return err
		}
		ctx, cancel, err := getRequestContext(localJobContext)
		if err != nil {
			return err
		}
		defer cancel()

		fmt.Printf("showing logs of \033[34m\033[1m%s\t\033\033[0m\n", name)

		return followJob(ctx, client, name, "")
	},
}

func followJob(ctx context.Context, client v1.WerftServiceClient, name, prefix string) error {
	logs, err := client.Listen(ctx, &v1.ListenRequest{
		Name:    name,
		Logs:    v1.ListenRequestLogs_LOGS_RAW,
		Updates: true,
	})
	if err != nil {
		return err
	}

	for {
		msg, err := logs.Recv()
		if err != nil {
			return err
		}

		if update := msg.GetUpdate(); update != nil {
			if update.Phase == v1.JobPhase_PHASE_DONE {
				prettyPrint(update, jobGetTpl)

				if update.Conditions.Success {
					os.Exit(0)
				} else {
					os.Exit(1)
				}
			}
		}
		if data := msg.GetSlice(); data != nil {
			if prefix == "" {
				pringLogSlice(data)
			} else {
				printLogSliceWithPrefix(prefix, data)
			}
		}
	}
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

func printLogSliceWithPrefix(prefix string, slice *v1.LogSliceEvent) {
	if slice.Name == "werft:kubernetes" || slice.Name == "werft:status" {
		return
	}

	switch slice.Type {
	case v1.LogSliceType_SLICE_PHASE:
		fmt.Printf("[%s%s|PHASE] %s\n", prefix, slice.Name, slice.Payload)
	case v1.LogSliceType_SLICE_CONTENT:
		fmt.Printf("[%s%s] %s\n", prefix, slice.Name, slice.Payload)
	case v1.LogSliceType_SLICE_DONE:
		fmt.Printf("[%s%s|DONE] %s\n", prefix, slice.Name, slice.Payload)
	case v1.LogSliceType_SLICE_FAIL:
		fmt.Printf("[%s%s|FAIL] %s\n", prefix, slice.Name, slice.Payload)
	case v1.LogSliceType_SLICE_RESULT:
		fmt.Printf("[%s|RESULT] %s\n", slice.Name, slice.Payload)
	}
}

func init() {
	jobCmd.AddCommand(jobLogsCmd)
}

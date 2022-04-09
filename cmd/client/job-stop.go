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
	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/spf13/cobra"
)

// jobStopCmd represents the list command
var jobStopCmd = &cobra.Command{
	Use:   "stop [name]",
	Short: "Stop a job",
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

		_, err = client.StopJob(ctx, &v1.StopJobRequest{
			Name: name,
		})
		if err != nil {
			return err
		}

		return nil
	},
}

func init() {
	jobCmd.AddCommand(jobStopCmd)
}

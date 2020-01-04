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
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var logResultCmd = &cobra.Command{
	Use:   "result <type> <payload>",
	Short: "logs a job result",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		tpe, payload := args[0], args[1]
		desc, _ := cmd.Flags().GetString("description")
		channels, _ := cmd.Flags().GetStringArray("channels")

		if desc != "" || len(channels) > 0 {
			var body struct {
				P string   `json:"payload"`
				C []string `json:"channels,omitempty"`
				D string   `json:"description,omitempty"`
			}
			body.P = payload
			body.C = channels
			body.D = desc

			msg, _ := json.Marshal(body)
			fmt.Printf("[%s|RESULT] %s\n", tpe, string(msg))
			return
		}

		fmt.Printf("[%s|RESULT] %s\n", tpe, payload)
	},
}

func init() {
	logCmd.AddCommand(logResultCmd)

	logResultCmd.Flags().StringP("description", "d", "", "result description")
	logResultCmd.Flags().StringArrayP("channels", "c", []string{}, "result channels (e.g. github or slack)")
}

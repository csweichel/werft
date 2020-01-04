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
	"fmt"
	"io"
	"os"

	"github.com/segmentio/textio"
	"github.com/spf13/cobra"
)

var logSliceCmd = &cobra.Command{
	Use:   "slice <name>",
	Short: "logs a job slice",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		if fail, _ := cmd.Flags().GetString("fail"); fail != "" {
			fmt.Printf("[%s|FAIL] %s\n", name, fail)
			return
		}
		if done, _ := cmd.Flags().GetBool("done"); done {
			fmt.Printf("[%s|DONE]\n", name)
			return
		}

		pw := textio.NewPrefixWriter(os.Stdout, fmt.Sprintf("[%s] ", name))
		defer pw.Flush()

		io.Copy(pw, os.Stdin)
	},
}

func init() {
	logCmd.AddCommand(logSliceCmd)
	logSliceCmd.Flags().String("fail", "", "fails the slice")
	logSliceCmd.Flags().Bool("done", false, "marks the slice done")
}

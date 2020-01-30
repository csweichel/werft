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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initJobCmd = &cobra.Command{
	Use:   "job <name>",
	Short: "creates a job YAML file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		fn, _ := cmd.Flags().GetString("output")
		if fn == "" {
			fn = filepath.Join(".werft", fmt.Sprintf("%s.yaml", name))
		}

		if _, err := os.Stat(filepath.Dir(fn)); err != nil {
			err := os.MkdirAll(filepath.Dir(fn), 0755)
			if err != nil {
				return err
			}
		}

		return ioutil.WriteFile(fn, []byte(`pod:
  containers:
  - name: `+name+`
    image: alpine:latest
    workingDir: /workspace
    imagePullPolicy: IfNotPresent
    command:
    - sh 
    - -c
    - |
      echo Hello World`), 0644)
	},
}

func init() {
	initCmd.AddCommand(initJobCmd)

	initJobCmd.Flags().StringP("output", "o", "", "output filename (defaults to .werft/jobname.yaml)")
}

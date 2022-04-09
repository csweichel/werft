package cmd

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
	"io/ioutil"
	"log"

	"github.com/spf13/cobra"
	servercmd "github.com/csweichel/werft/cmd/server"
)

// validateConfigCmd validates a Werft config file.
var validateConfigCmd = &cobra.Command{
	Use:   "validate-config",
	Short: "Validates a YAML config file used to start a Werft server.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fc, err := ioutil.ReadFile(args[0])
		if err != nil {
			log.Fatalf("Could not read %v", args[0])
		}

		_, err = servercmd.LoadConfig(fc)
		if err != nil {
			log.Fatalf(err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(validateConfigCmd)
}
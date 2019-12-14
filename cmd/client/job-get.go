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

	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/spf13/cobra"
)

var jobGetTpl = `Name:	{{ .Name }}
Phase:	{{ .Phase }}
Success:	{{ .Conditions.Success }}
Metadata:
  Owner:	{{ .Metadata.Owner }}
  Trigger:	{{ .Metadata.Trigger }}
  Started:	{{ .Metadata.Created | toRFC3339 }}
  Finished:	{{ .Metadata.Finished | toRFC3339 }}
Repository:
  Host:	{{ .Metadata.Repository.Host }}
  Owner:	{{ .Metadata.Repository.Owner }}
  Repo:	{{ .Metadata.Repository.Repo }}
  Ref:	{{ .Metadata.Repository.Ref }}
  Revision:	{{ .Metadata.Repository.Revision }}
{{- if .Results }}
Results:
{{- range .Results }}
  {{ .Type }}:	{{ .Payload }}
	{{ .Description -}}
{{ end -}}
{{- end }}
`

// jobGetCmd represents the list command
var jobGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Retrieves details of a job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn := dial()
		defer conn.Close()
		client := v1.NewWerftServiceClient(conn)

		ctx := context.Background()
		resp, err := client.GetJob(ctx, &v1.GetJobRequest{
			Name: args[0],
		})
		if err != nil {
			return err
		}

		return prettyPrint(resp.Result, jobGetTpl)
	},
}

func init() {
	jobCmd.AddCommand(jobGetCmd)
}

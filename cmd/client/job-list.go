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
	"strings"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/csweichel/werft/pkg/filterexpr"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

// jobListCmd represents the list command
var jobListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists and searches for jobs",
	Long: `Lists and searches for jobs using search expressions in the form of "<key><op><value>":
Available keys are:
  name        name of the job
  trigger     one of push, manual, unkown
  owner       owner/originator of the job
  phase       one of unknown, preparing, starting, running, done
  repo.owner  owner of the source repository
  repo.repo   name of the source repository
  repo.host   host of the source repository (e.g. github.com)
  repo.ref    source reference, i.e. branch name
  success     one of true, false
  created     time the job started as RFC3339 date

Available operators are:
  ==          checks for equality
  ~=          value must be contained in
  |=		  starts with
  =|          ends with

Operators can be negated by prefixing them with !.

For example:
  phase==running             finds all running jobs
  owner!==webui              finds all jobs NOT owned by webui
  repo.repo|=werft           finds all jobs on repositories whose names begin with werft
  phase==done success==true  finds all successfully finished jobs
		`,
	RunE: func(cmd *cobra.Command, args []string) error {
		filterterms, err := filterexpr.Parse(args)
		if err != nil {
			return err
		}
		filter := []*v1.FilterExpression{
			{Terms: filterterms},
		}

		useLocalContext, _ := cmd.Flags().GetBool("local")
		if useLocalContext {
			lf, err := getLocalContextJobFilter()
			if err != nil {
				return xerrors.Errorf("--local requires the current working directory to be a Git repo: %w", err)
			}

			filter = append(filter, lf...)
		}

		orderExprs, _ := cmd.Flags().GetStringArray("order")
		order, err := parseOrder(orderExprs)
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetUint("limit")
		offset, _ := cmd.Flags().GetUint("offset")
		req := v1.ListJobsRequest{
			Filter: filter,
			Order:  order,
			Limit:  int32(limit),
			Start:  int32(offset),
		}

		conn := dial()
		defer conn.Close()
		client := v1.NewWerftServiceClient(conn)

		ctx := context.Background()
		resp, err := client.ListJobs(ctx, &req)
		if err != nil {
			return err
		}

		return prettyPrint(resp, `NAME	OWNER	REPO	PHASE	SUCCESS
{{- range .Result }}
{{ .Name }}	{{ .Metadata.Owner }}	{{ .Metadata.Repository.Owner }}/{{ .Metadata.Repository.Repo }}	{{ .Phase }}	{{ .Conditions.Success -}}
{{ end }}
`)
	},
}

func parseOrder(exprs []string) ([]*v1.OrderExpression, error) {
	res := make([]*v1.OrderExpression, len(exprs))
	for i, expr := range exprs {
		segs := strings.Split(expr, ":")
		if len(segs) != 2 {
			return nil, xerrors.Errorf("invalid order expression: %s", expr)
		}

		res[i] = &v1.OrderExpression{
			Field:     segs[0],
			Ascending: segs[1] == "asc",
		}
	}
	return res, nil
}

func init() {
	jobCmd.AddCommand(jobListCmd)

	jobListCmd.Flags().Uint("limit", 50, "limit the number of results")
	jobListCmd.Flags().Uint("offset", 0, "return results starting later than zero")
	jobListCmd.Flags().StringArray("order", []string{"name:desc"}, "order the result list by fields")
	jobListCmd.Flags().BoolP("local", "l", false, "finds jobs matching the local Git context")
}

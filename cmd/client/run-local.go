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
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/paulbellamy/ratecounter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

// runLocalCmd represents the triggerLocal command
var runLocalCmd = &cobra.Command{
	Use:   "local",
	Short: "starts a job from a local directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		if wu, _ := getWaitUntil(); wu != nil {
			return xerrors.Errorf("--wait-until is not supported for local jobs")
		}

		flags := cmd.Parent().PersistentFlags()
		workingdir, _ := cmd.Flags().GetString("cwd")
		triggerName, _ := flags.GetString("trigger")
		trigger, ok := v1.JobTrigger_value[fmt.Sprintf("TRIGGER_%s", strings.ToUpper(triggerName))]
		if !ok {
			var vs []string
			for k := range v1.JobTrigger_value {
				vs = append(vs, strings.ToLower(strings.TrimPrefix("TRIGGER_", k)))
			}

			return xerrors.Errorf("Invalid value for --trigger. Valid choices are %s", strings.Join(vs, "\n"))
		}

		md, err := getLocalJobContext(workingdir, v1.JobTrigger(trigger))
		if err != nil {
			log.WithError(err).Warn("cannot extract local job context - continuing with default")
			md = &v1.JobMetadata{
				Owner: "local",
				Repository: &v1.Repository{
					Host:  "unknown",
					Owner: "none",
					Repo:  "none",
				},
			}
		}
		addUserAnnotations(md)

		var configYAML []byte
		jobPath, _ := cmd.Flags().GetString("job-file")
		if jobPath == "" {
			return xerrors.Errorf("missing job file")
		}
		jobYAML, err := ioutil.ReadFile(jobPath)
		if err != nil {
			return xerrors.Errorf("cannot read job file: %w", err)
		}

		conn := dial()
		defer conn.Close()
		client := v1.NewWerftServiceClient(conn)

		ctx, cancel, err := getRequestContext(md)
		if err != nil {
			return err
		}
		defer cancel()
		srv, err := client.StartLocalJob(ctx)
		if err != nil {
			return xerrors.Errorf("cannot start job: %w", err)
		}

		err = srv.Send(&v1.StartLocalJobRequest{
			Content: &v1.StartLocalJobRequest_Metadata{
				Metadata: md,
			},
		})
		if err != nil {
			return xerrors.Errorf("cannot send metadata: %w", err)
		}
		err = srv.Send(&v1.StartLocalJobRequest{
			Content: &v1.StartLocalJobRequest_ConfigYaml{
				ConfigYaml: configYAML,
			},
		})
		if err != nil {
			return xerrors.Errorf("cannot send config yaml: %w", err)
		}
		err = srv.Send(&v1.StartLocalJobRequest{
			Content: &v1.StartLocalJobRequest_JobYaml{
				JobYaml: jobYAML,
			},
		})
		if err != nil {
			return xerrors.Errorf("cannot send job yaml: %w", err)
		}

		tarcmd := exec.Command("tar", "cz", ".")
		tarcmd.Dir = workingdir
		tarStream, err := tarcmd.StdoutPipe()
		tarcmd.Stderr = os.Stderr
		if err != nil {
			return xerrors.Errorf("cannot setup tar: %w", err)
		}
		err = tarcmd.Start()
		if err != nil {
			return xerrors.Errorf("cannot start tar process: %w", err)
		}

		buf := make([]byte, 32768)
		total := 0
		counter := ratecounter.NewRateCounter(1 * time.Second)
		const mib = 1024 * 1024
		for {
			n, err := tarStream.Read(buf)
			if err != nil && err != io.EOF {
				return xerrors.Errorf("cannot read tar data: %w", err)
			}

			if n > 0 {
				total += n
				counter.Incr(int64(n))
				if total%mib == 0 {
					log.WithField("total [mb]", float32(total)/mib).WithField("rate [mb/s]", float32(counter.Rate())/mib).Debug("uploading tar data")
				}

				err = srv.Send(&v1.StartLocalJobRequest{
					Content: &v1.StartLocalJobRequest_WorkspaceTar{
						WorkspaceTar: buf[:n],
					},
				})
				if err != nil {
					return xerrors.Errorf("cannot forward tar data: %w", err)
				}
			}
			if err == io.EOF {
				// we're done here
				log.Debug("done uploading workspace content")
				err = srv.Send(&v1.StartLocalJobRequest{
					Content: &v1.StartLocalJobRequest_WorkspaceTarDone{
						WorkspaceTarDone: true,
					},
				})
				if err != nil {
					return xerrors.Errorf("cannot signal tar data end: %w", err)
				}

				break
			}
		}

		resp, err := srv.CloseAndRecv()
		if err != nil {
			return xerrors.Errorf("cannot complete job startup: %w", err)
		}
		fmt.Println(resp.Status.Name)

		follow, _ := flags.GetBool("follow")
		withPrefix, _ := flags.GetString("follow-with-prefix")
		if follow || withPrefix != "" {
			err = followJob(ctx, client, resp.Status.Name, withPrefix)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	runCmd.AddCommand(runLocalCmd)

	wd, _ := os.Getwd()
	runLocalCmd.Flags().String("cwd", wd, "working directory")
	runLocalCmd.Flags().StringP("job-file", "j", "", "start a particular job (defaults to the default job of the repo)")
}

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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/32leaves/werft/pkg/reporef"
	"github.com/golang/protobuf/ptypes"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxSideloadSizeBytes = 3 * 1024 * 1024

// runGithubCmd represents the triggerRemote command
var runGithubCmd = &cobra.Command{
	Use:   "github [<owner>/<repo>(:ref | @revision)]",
	Short: "starts a job from a remote repository",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Parent().PersistentFlags()
		cwd, _ := flags.GetString("cwd")

		var (
			md  *v1.JobMetadata
			err error
		)
		if len(args) == 0 {
			md, err = getLocalJobContext(cwd, v1.JobTrigger_TRIGGER_MANUAL)
		} else {
			repo, err := reporef.Parse(args[0])
			if err != nil {
				return err
			}
			md = &v1.JobMetadata{
				Owner:      repo.Owner,
				Repository: repo,
			}
		}
		if err != nil {
			return err
		}
		addUserAnnotations(md)

		triggerName, _ := flags.GetString("trigger")
		trigger, ok := v1.JobTrigger_value[fmt.Sprintf("TRIGGER_%s", strings.ToUpper(triggerName))]
		if !ok {
			var vs []string
			for k := range v1.JobTrigger_value {
				vs = append(vs, strings.ToLower(strings.TrimPrefix("TRIGGER_", k)))
			}

			return xerrors.Errorf("Invalid value for --trigger. Valid choices are %s", strings.Join(vs, "\n"))
		}
		md.Trigger = v1.JobTrigger(trigger)

		token, _ := cmd.Flags().GetString("token")
		req := &v1.StartGitHubJobRequest{
			Metadata:    md,
			GithubToken: token,
		}

		req.JobPath, _ = cmd.Flags().GetString("remote-job-path")
		if fn, _ := flags.GetString("job-file"); fn != "" {
			fc, err := ioutil.ReadFile(fn)
			if err != nil {
				return err
			}

			req.JobYaml = fc
			if req.JobPath == "" {
				req.JobPath = fn
			}
		}

		sideload, _ := cmd.Flags().GetStringArray("sideload")
		if len(sideload) > 0 {
			sd, err := compileSideload(sideload)
			if err != nil {
				return err
			}
			req.Sideload = sd
		}

		waitUntil, err := getWaitUntil()
		if err != nil {
			return err
		}
		if waitUntil != nil {
			req.WaitUntil, err = ptypes.TimestampProto(*waitUntil)
			if err != nil {
				return err
			}
		}

		conn := dial()
		defer conn.Close()
		client := v1.NewWerftServiceClient(conn)

		ctx := context.Background()
		resp, err := client.StartGitHubJob(ctx, req)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return xerrors.Errorf("%s. Are all changes pushed to origin?", err.Error())
			}

			return err
		}
		fmt.Println(resp.Status.Name)

		follow, _ := flags.GetBool("follow")
		withPrefix, _ := flags.GetString("follow-with-prefix")
		if follow || withPrefix != "" {
			err = followJob(client, resp.Status.Name, withPrefix)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func compileSideload(files []string) ([]byte, error) {
	res := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(res)
	tw := tar.NewWriter(gw)

	for _, fn := range files {
		src, tgt := fn, fn
		if strings.Contains(src, ":") {
			segs := strings.Split(src, ":")
			src, tgt = segs[0], segs[1]
		}

		if filepath.IsAbs(tgt) {
			return nil, xerrors.Errorf("cannot sideload using absolute filepaths")
		}
		if strings.Contains(tgt, "../") {
			return nil, xerrors.Errorf("files must be relative current working directory (i.e. no ../ in path)")
		}

		f, err := os.OpenFile(src, os.O_RDONLY, 0600)
		if err != nil {
			return nil, xerrors.Errorf("cannot sideload %s: %w", src, err)
		}
		defer f.Close()
		stat, err := f.Stat()
		if err != nil {
			return nil, xerrors.Errorf("cannot sideload %s: %w", src, err)
		}
		if int64(res.Len())+stat.Size() > maxSideloadSizeBytes {
			return nil, xerrors.Errorf("maximum sideload size (%d bytes) exceeded at file \"%s\"", maxSideloadSizeBytes, src)
		}

		err = tw.WriteHeader(&tar.Header{
			Name:    tgt,
			Size:    stat.Size(),
			Mode:    int64(stat.Mode()),
			ModTime: stat.ModTime(),
		})
		if err != nil {
			return nil, xerrors.Errorf("cannot sideload %s: %w", src, err)
		}
		_, err = io.Copy(tw, f)
		if err != nil {
			return nil, xerrors.Errorf("cannot sideload %s: %w", src, err)
		}
	}
	err := tw.Close()
	if err != nil {
		return nil, err
	}
	err = gw.Close()
	if err != nil {
		return nil, err
	}

	return res.Bytes(), nil
}

func init() {
	runCmd.AddCommand(runGithubCmd)

	runGithubCmd.Flags().String("token", "", "Token to use for authorization against GitHub")
	runGithubCmd.Flags().String("remote-job-path", "", "start the job at that path in the repo (defaults to the default job of the repo)")
	runGithubCmd.Flags().StringArrayP("sideload", "s", []string{}, "sideload files overwriting/adding to the Git working copy")
}

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
	"bytes"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	v1 "github.com/32leaves/werft/pkg/api/v1"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Starts the execution of a job",
	Args:  cobra.MinimumNArgs(1),
}

func getLocalJobContext(wd string, trigger v1.JobTrigger) (*v1.JobMetadata, error) {
	var repo v1.Repository

	stderr := bytes.NewBuffer(nil)
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = wd
	cmd.Stderr = stderr
	revision, err := cmd.Output()
	if eerr, ok := err.(*exec.ExitError); ok && eerr.ExitCode() == 128 {
		return nil, xerrors.Errorf(stderr.String())
	}
	if err != nil {
		return nil, err
	}
	repo.Revision = strings.TrimSpace(string(revision))

	cmd = exec.Command("git", "rev-parse", "--symbolic-full-name", "HEAD")
	cmd.Dir = wd
	ref, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	repo.Ref = strings.TrimSpace(string(ref))

	cmd = exec.Command("git", "config", "--global", "user.name")
	user, err := cmd.Output()
	if err != nil {
		return nil, xerrors.Errorf("cannot get global git user: %w", err)
	}

	cmd = exec.Command("git", "config", "--get", "remote.origin.url")
	origin, err := cmd.Output()
	if err == nil {
		err = configureRepoFromOrigin(&repo, strings.TrimSpace(string(origin)))
		if err != nil {
			log.WithError(err).Debug("cannot parse local context")
		}
	}
	if repo.Owner == "" {
		repo.Host = "local"
		repo.Owner = "local"
		repo.Repo = filepath.Base(wd)
	}

	return &v1.JobMetadata{
		Owner:      strings.TrimSpace(string(user)),
		Repository: &repo,
		Trigger:    trigger,
	}, nil
}

// configureRepoFromOrigin is very much geared towards GitHub origins in the form of:
//     https://github.com/32leaves/werft.git
// It might work on others, but that's neither tested nor intended.
func configureRepoFromOrigin(repo *v1.Repository, origin string) error {
	ourl, err := url.Parse(strings.TrimSpace(string(origin)))
	if err != nil {
		return err
	}

	repo.Host = ourl.Host

	segs := strings.Split(strings.Trim(ourl.Path, "/"), "/")
	if len(segs) >= 2 {
		repo.Owner = segs[0]
		repo.Repo = strings.TrimSuffix(segs[1], ".git")
	}

	return nil
}

// adds the annotations from --annotation to the metadata
func addUserAnnotations(md *v1.JobMetadata) {
	annotations, _ := runCmd.PersistentFlags().GetStringToString("annotations")
	for k, v := range annotations {
		md.Annotations = append(md.Annotations, &v1.Annotation{
			Key:   k,
			Value: v,
		})
	}
}

func getWaitUntil() (*time.Time, error) {
	w, _ := runCmd.PersistentFlags().GetString("wait-until")
	if w == "" {
		return nil, nil
	}

	dt, err := time.ParseDuration(w)
	if err == nil {
		t := time.Now().Add(dt)
		return &t, nil
	}
	t, err := time.Parse(time.RFC3339, w)
	if err == nil {
		return &t, nil
	}

	return nil, xerrors.Errorf("cannot parse wait-until: %w", err)
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.PersistentFlags().StringP("job-file", "j", "", "location of the job file (defaults to the default job in the werft config)")
	runCmd.PersistentFlags().String("config-file", "$CWD/.werft/config.yaml", "location of the werft config file")
	runCmd.PersistentFlags().String("trigger", "manual", "job trigger. One of push, manual")
	runCmd.PersistentFlags().BoolP("follow", "f", false, "follow the log output once the job is running")
	runCmd.PersistentFlags().StringToStringP("annotations", "a", map[string]string{}, "adds an annotation to the job")
	runCmd.PersistentFlags().String("follow-with-prefix", "", "prints the log output with a prefix and disbales colors - useful for starting jobs from within jobs")
	runCmd.PersistentFlags().String("wait-until", "", "delays the execution of the job by/until some time - use a valid duration (e.g. 5h) or RFC3339 timestamp")
}

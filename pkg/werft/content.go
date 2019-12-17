package werft

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	// PathWerftConfig is the path relative to the repo root where we expect to find the werft config YAML
	PathWerftConfig = ".werft/config.yaml"
)

// ContentProvider provides access to job workspace content
type ContentProvider interface {
	// InitContainer builds the container that will initialize the job content.
	// The VolumeMount for /workspace is added by the caller.
	// Name and ImagePullPolicy will be overwriten.
	InitContainer() corev1.Container

	// Serve provides additional services required during initialization.
	// This function is expected to return immediately.
	Serve(jobName string) error
}

// FileProvider provides access to a single file
type FileProvider interface {
	// Download provides access to a single file
	Download(ctx context.Context, path string) (io.ReadCloser, error)
}

// LocalContentProvider provides access to local files
type LocalContentProvider struct {
	TarStream io.Reader

	Namespace  string
	Kubeconfig *rest.Config
	Clientset  kubernetes.Interface
}

// InitContainer builds the container that will initialize the job content.
func (lcp *LocalContentProvider) InitContainer() corev1.Container {
	return corev1.Container{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "while [ ! -f /workspace/.ready ]; do [ -f /workspace/.failed ] && exit 1; sleep 1; done"},
		WorkingDir: "/workspace",
	}
}

// Serve provides additional services required during initialization.
func (lcp *LocalContentProvider) Serve(jobName string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		err := lcp.copyToPod(jobName)
		if err == nil {
			break
		}

		log.WithError(err).Debug("could not initialize (yet), will try again")
		<-ticker.C
	}
	log.Debug("local content served")

	return nil
}

func (lcp *LocalContentProvider) copyToPod(name string) error {
	req := lcp.Clientset.CoreV1().RESTClient().
		Post().
		Namespace(lcp.Namespace).
		Resource("pods").
		Name(name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "werft-checkout",
			Command:   []string{"sh", "-c", "cd /workspace && tar xz; if [ $? == 0 ]; then touch .ready; else touch .failed; fi"},
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	remoteExec, err := remotecommand.NewSPDYExecutor(lcp.Kubeconfig, "POST", req.URL())
	if err != nil {
		return xerrors.Errorf("executor run: %w", err)
	}

	// This call waits for the process to end
	err = remoteExec.Stream(remotecommand.StreamOptions{
		Stdin:  lcp.TarStream,
		Stdout: log.New().WithField("pod", name).WriterLevel(log.DebugLevel),
		Stderr: log.New().WithField("pod", name).WriterLevel(log.ErrorLevel),
		Tty:    false,
	})
	if err != nil {
		return err
	}

	return nil
}

// tarWithReadyFile adds a gzipped tar entry containting an empty file named .ready to the stream
type tarWithReadyFile struct {
	O         io.Reader
	remainder []byte
	eof       bool
}

func (t *tarWithReadyFile) Read(p []byte) (n int, err error) {
	if len(t.remainder) > 0 {
		n = copy(p, t.remainder)
		t.remainder = t.remainder[n:]
		return
	}
	if len(t.remainder) == 0 && t.eof {
		log.Debug("tarWithReadyFile EOF")
		return n, io.EOF
	}

	n, err = t.O.Read(p)
	log.WithField("n", n).WithError(err).Debug("incoming tar data")
	if err == io.EOF {
		t.eof = true
		err = nil

		buf := bytes.NewBuffer(nil)
		gzipW := gzip.NewWriter(buf)
		tarW := tar.NewWriter(gzipW)
		tarW.WriteHeader(&tar.Header{
			Name:     ".ready",
			Size:     0,
			Typeflag: tar.TypeBlock,
		})
		tarW.Close()
		gzipW.Close()
		t.remainder = buf.Bytes()
	}
	return
}

// GitHubContentProvider provides access to GitHub content
type GitHubContentProvider struct {
	Owner    string
	Repo     string
	Revision string
	Client   *github.Client
	Token    string
}

// Download provides access to a single file
func (gcp *GitHubContentProvider) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	return gcp.Client.Repositories.DownloadContents(ctx, gcp.Owner, gcp.Repo, path, &github.RepositoryContentGetOptions{
		Ref: gcp.Revision,
	})
}

// InitContainer builds the container that will initialize the job content.
func (gcp *GitHubContentProvider) InitContainer() corev1.Container {
	cloneCmd := "git clone"
	if gcp.Token != "" {
		cloneCmd = "git clone -c \"credential.helper=/bin/sh -c 'echo username=$GHTOKEN; echo password=x-oauth-basic'\""
	}
	cloneCmd = fmt.Sprintf("%s https://github.com/%s/%s.git .; git checkout %s", cloneCmd, gcp.Owner, gcp.Repo, gcp.Revision)

	return corev1.Container{
		Image: "alpine/git:latest",
		Env: []corev1.EnvVar{
			corev1.EnvVar{
				Name:  "GHTOKEN",
				Value: gcp.Token,
			},
		},
		Command: []string{
			"sh", "-c",
			cloneCmd,
		},
		WorkingDir: "/workspace",
	}
}

// Serve provides additional services required during initialization.
func (gcp *GitHubContentProvider) Serve(jobName string) error {
	return nil
}

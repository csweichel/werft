package keel

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
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
	// PathKeelConfig is the path relative to the repo root where we expect to find the keel config YAML
	PathKeelConfig = ".keel/config.yaml"
)

// ContentProvider provides access to job workspace content
type ContentProvider interface {
	// Download provides access to a single file
	Download(ctx context.Context, path string) (io.ReadCloser, error)

	// InitContainer builds the container that will initialize the job content.
	// The VolumeMount for /workspace is added by the caller.
	// Name and ImagePullPolicy will be overwriten.
	InitContainer() corev1.Container

	// Serve provides additional services required during initialization.
	// This function is expected to return immediately.
	Serve(jobName string) error
}

// LocalContentProvider provides access to local files
type LocalContentProvider struct {
	TarStream    io.Reader
	FileProvider func(path string) (io.ReadCloser, error)

	Namespace  string
	Kubeconfig *rest.Config
	Clientset  kubernetes.Interface
}

// Download provides access to a single file
func (lcp *LocalContentProvider) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	return lcp.FileProvider(path)
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
	dfs, err := ioutil.TempFile(os.TempDir(), "keel-lcp")
	if err != nil {
		return err
	}
	defer dfs.Close()
	defer os.Remove(dfs.Name())

	// upload the file to this server
	_, err = io.Copy(dfs, lcp.TarStream)
	if err != nil {
		return err
	}
	// reset the position in the file - important: otherwise the re-upload to the container fails
	_, err = dfs.Seek(0, 0)

	req := lcp.Clientset.CoreV1().RESTClient().
		Post().
		Namespace(lcp.Namespace).
		Resource("pods").
		Name(name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "keel-checkout",
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
		Stdin:  dfs,
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
}

// Download provides access to a single file
func (gcp *GitHubContentProvider) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	return gcp.Client.Repositories.DownloadContents(ctx, gcp.Owner, gcp.Repo, path, &github.RepositoryContentGetOptions{
		Ref: gcp.Revision,
	})
}

// InitContainer builds the container that will initialize the job content.
func (gcp *GitHubContentProvider) InitContainer() corev1.Container {
	return corev1.Container{
		Image: "alpine/git:latest",
		Command: []string{
			"sh", "-c",
			fmt.Sprintf("git clone https://github.com/%s/%s.git .; git checkout %s", gcp.Owner, gcp.Repo, gcp.Revision),
		},
		WorkingDir: "/workspace",
	}
}

// Serve provides additional services required during initialization.
func (gcp *GitHubContentProvider) Serve(jobName string) error {
	return nil
}

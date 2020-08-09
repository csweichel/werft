package werft

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"time"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/google/go-github/v31/github"
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

// RepositoryProvider provides access to a repository
type RepositoryProvider interface {
	// Resolve resolves the repo's revision based on its ref(erence).
	// If the revision is already set, this operation does nothing.
	Resolve(repo *v1.Repository) error

	// ContentProvider produces a content provider for a particular repo
	ContentProvider(repo *v1.Repository) (ContentProvider, error)

	// FileProvider provides direct access to repository content
	FileProvider(repo *v1.Repository) (FileProvider, error)
}

// ContentProvider provides access to job workspace content
type ContentProvider interface {
	// InitContainer builds the container that will initialize the job content.
	// The VolumeMount for /workspace is added by the caller.
	// Name and ImagePullPolicy will be overwriten.
	InitContainer() ([]corev1.Container, error)

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
func (lcp *LocalContentProvider) InitContainer() ([]corev1.Container, error) {
	return []corev1.Container{
		{
			Name:       "content-upload",
			Image:      "alpine:latest",
			Command:    []string{"sh", "-c", "while [ ! -f /workspace/.ready ]; do [ -f /workspace/.failed ] && exit 1; sleep 1; done"},
			WorkingDir: "/workspace",
		},
	}, nil
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
	Auth     GitCredentialHelper
}

// GitHubContentProviderSideload enables side-loading of files after a Git clone
type GitHubContentProviderSideload struct {
	TarStream io.Reader

	Namespace  string
	Kubeconfig *rest.Config
	Clientset  kubernetes.Interface
}

// Download provides access to a single file
func (gcp *GitHubContentProvider) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	return gcp.Client.Repositories.DownloadContents(ctx, gcp.Owner, gcp.Repo, path, &github.RepositoryContentGetOptions{
		Ref: gcp.Revision,
	})
}

// InitContainer builds the container that will initialize the job content.
func (gcp *GitHubContentProvider) InitContainer() ([]corev1.Container, error) {
	var (
		user string
		pass string
		err  error
	)
	if gcp.Auth != nil {
		user, pass, err = gcp.Auth(context.Background())
		if err != nil {
			return nil, err
		}
	}

	cloneCmd := "git clone"
	if user != "" || pass != "" {
		cloneCmd = fmt.Sprintf("git clone -c \"credential.helper=/bin/sh -c 'echo username=$GHUSER_SECRET; echo password=$GHPASS_SECRET'\"")
	}
	cloneCmd = fmt.Sprintf("%s https://github.com/%s/%s.git .; git checkout %s", cloneCmd, gcp.Owner, gcp.Repo, gcp.Revision)

	return []corev1.Container{
		corev1.Container{
			Name:  "github-checkout",
			Image: "alpine/git:latest",
			Command: []string{
				"sh", "-c",
				cloneCmd,
			},
			Env: []corev1.EnvVar{
				corev1.EnvVar{
					Name:  "GHUSER_SECRET",
					Value: user,
				},
				corev1.EnvVar{
					Name:  "GHPASS_SECRET",
					Value: pass,
				},
			},
			WorkingDir: "/workspace",
		},
	}, nil
}

// Serve provides additional services required during initialization.
func (gcp *GitHubContentProvider) Serve(jobName string) error {
	return nil
}

// SideloadingContentProvider first runs the delegate and then sideloads files
type SideloadingContentProvider struct {
	Delegate ContentProvider

	TarStream  io.Reader
	Namespace  string
	Kubeconfig *rest.Config
	Clientset  kubernetes.Interface
}

// InitContainer adds the sideload init container
func (s *SideloadingContentProvider) InitContainer() ([]corev1.Container, error) {
	res, err := s.Delegate.InitContainer()
	if err != nil {
		return nil, err
	}

	res = append(res, corev1.Container{
		Name:  "sideload",
		Image: "alpine/git:latest",
		Command: []string{
			"sh", "-c",
			"echo waiting for sideload; while [ ! -f /workspace/.ready ]; do [ -f /workspace/.failed ] && exit 1; sleep 1; done",
		},
		WorkingDir: "/workspace",
	})
	return res, nil
}

// Serve serves the actual sideload
func (s *SideloadingContentProvider) Serve(jobName string) error {
	err := s.Delegate.Serve(jobName)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		err := s.sideload(jobName)
		if err == nil {
			break
		}

		log.WithError(err).Debug("could not initialize (yet), will try again")
		<-ticker.C
	}
	log.Debug("local content served")

	return nil
}

func (s *SideloadingContentProvider) sideload(jobName string) error {
	req := s.Clientset.CoreV1().RESTClient().
		Post().
		Namespace(s.Namespace).
		Resource("pods").
		Name(jobName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "sideload",
			Command:   []string{"sh", "-c", "cd /workspace && tar xz; if [ $? == 0 ]; then touch .ready; else touch .failed; fi"},
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	remoteExec, err := remotecommand.NewSPDYExecutor(s.Kubeconfig, "POST", req.URL())
	if err != nil {
		return xerrors.Errorf("executor run: %w", err)
	}

	// This call waits for the process to end
	err = remoteExec.Stream(remotecommand.StreamOptions{
		Stdin:  s.TarStream,
		Stdout: log.New().WithField("pod", jobName).WriterLevel(log.DebugLevel),
		Stderr: log.New().WithField("pod", jobName).WriterLevel(log.ErrorLevel),
		Tty:    false,
	})
	if err != nil {
		return err
	}

	return nil
}

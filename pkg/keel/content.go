package keel

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
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
	BasePath string

	Namespace  string
	Kubeconfig *rest.Config
	Clientset  kubernetes.Interface
}

// Download provides access to a single file
func (lcp *LocalContentProvider) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	return os.OpenFile(filepath.Join(lcp.BasePath, path), os.O_RDONLY, 0644)
}

// InitContainer builds the container that will initialize the job content.
func (lcp *LocalContentProvider) InitContainer() corev1.Container {
	return corev1.Container{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "while [ ! -f /workspace/.ready ]; do sleep 1; done"},
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
	req := lcp.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(name).
		Namespace(lcp.Namespace).
		SubResource("exec")
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	if err != nil {
		return xerrors.Errorf("executor run: %w", err)
	}

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   []string{"sh", "-c", "cd /workspace && tar xz && touch .ready"},
		Container: "keel-checkout",
		Stdin:     true,
		Stdout:    false,
		Stderr:    false,
		TTY:       false,
	}, parameterCodec)

	cfg := lcp.Kubeconfig
	cfg.Timeout = 10 * time.Second
	remoteExec, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return xerrors.Errorf("executor run: %w", err)
	}

	inr, inw := io.Pipe()
	errchan := make(chan error)
	go func() {
		err = remoteExec.Stream(remotecommand.StreamOptions{
			Stdin: inr,
			Tty:   false,
		})
		if err != nil {
			errchan <- err
		}
		close(errchan)
	}()

	select {
	case err := <-errchan:
		return err
	case <-time.After(time.Second):
		// this is a bit tricky. If exec.Stream errors, it returns "immediately". If it doesn't it blocks until the process ends.
		// So we wait some time to catch the initial, setup errors here. All other errors are passed to the caller via errchan
		// and have to be handled there
	}

	cmd := exec.Command("tar", "cz", ".")
	cmd.Dir = lcp.BasePath
	cmd.Stdout = inw
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
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

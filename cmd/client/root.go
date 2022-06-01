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
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/golang/protobuf/jsonpb"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

var rootCmdOpts struct {
	Verbose          bool
	Host             string
	Kubeconfig       string
	K8sNamespace     string
	K8sLabelSelector string
	K8sPodPort       string
	DialMode         string
	CredentialHelper string
	TLSMode          string
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:          "werft",
	Short:        "werft is a very simple GitHub triggered and Kubernetes powered CI system",
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if rootCmdOpts.Verbose {
			log.SetLevel(log.DebugLevel)
			log.Debug("verbose logging enabled")
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		// We'll do some common error handling here, aiming to make the errors more actionable.
		if s, ok := status.FromError(err); ok {
			switch s.Code() {
			case codes.Unauthenticated:
				fmt.Print("\033[1mtip:\033[0m Try and use a credential helper by setting the WERFT_CREDENTIAL_HELPER env var.\n\n")
			case codes.Internal:
				fmt.Print("\033[1mtip:\033[0m There seems to be a problem with your werft installation - please get in contact with whoever is operating this installation.\n\n")
			}
		}

		os.Exit(1)
	}
}

const (
	dialModeHost       = "host"
	dialModeKubernetes = "kubernetes"
	tlsDialMode        = "none"
)

func init() {
	werftHost := os.Getenv("WERFT_HOST")
	if werftHost == "" {
		werftHost = "localhost:7777"
	}
	werftKubeconfig := os.Getenv("KUBECONFIG")
	if werftKubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.WithError(err).Warn("cannot determine user's home directory")
		} else {
			werftKubeconfig = filepath.Join(home, ".kube", "config")
		}
	}
	werftNamespace := os.Getenv("WERFT_K8S_NAMESPACE")
	werftLabelSelector := os.Getenv("WERFT_K8S_LABEL")
	if werftLabelSelector == "" {
		werftLabelSelector = "app.kubernetes.io/name=werft"
	}
	werftPodPort := os.Getenv("WERFT_K8S_POD_PORT")
	if werftPodPort == "" {
		werftPodPort = "7777"
	}
	dialMode := os.Getenv("WERFT_DIAL_MODE")
	if dialMode == "" {
		dialMode = string(dialModeHost)
	}
	tlsMode := os.Getenv("WERFT_TLS_MODE")
	if tlsMode == "" {
		tlsMode = tlsDialMode
	}

	rootCmd.PersistentFlags().BoolVar(&rootCmdOpts.Verbose, "verbose", false, "en/disable verbose logging")
	rootCmd.PersistentFlags().StringVar(&rootCmdOpts.DialMode, "dial-mode", dialMode, "dial mode that determines how we connect to werft. Valid values are \"host\" or \"kubernetes\" (defaults to WERFT_DIAL_MODE env var).")
	rootCmd.PersistentFlags().StringVar(&rootCmdOpts.Host, "host", werftHost, "[host dial mode] werft host to talk to (defaults to WERFT_HOST env var)")
	rootCmd.PersistentFlags().StringVar(&rootCmdOpts.Kubeconfig, "kubeconfig", werftKubeconfig, "[kubernetes dial mode] kubeconfig file to use (defaults to KUEBCONFIG env var)")
	rootCmd.PersistentFlags().StringVar(&rootCmdOpts.K8sNamespace, "k8s-namespace", werftNamespace, "[kubernetes dial mode] Kubernetes namespace in which to look for the werft pods (defaults to WERFT_K8S_NAMESPACE env var, or configured kube context namespace)")
	rootCmd.PersistentFlags().StringVar(&rootCmdOpts.CredentialHelper, "credential-helper", os.Getenv("WERFT_CREDENTIAL_HELPER"), "[host dial mode] credential helper to use (defaults to WERFT_CREDENTIAL_HELPER env var)")
	rootCmd.PersistentFlags().StringVar(&rootCmdOpts.TLSMode, "tls-mode", tlsMode, "[tls mode] determines TLS mode to use when talking to werft. Values values are \"none\" (i.e. insecure), \"system\" (use system certificates) or \"/path/to/ca.pem\" (Defaults to \"none\").")
	// The following are such specific flags that really only matters if one doesn't use the stock helm charts.
	// They can still be set using an env var, but there's no need to clutter the CLI with them.
	rootCmdOpts.K8sLabelSelector = werftLabelSelector
	rootCmdOpts.K8sPodPort = werftPodPort
}

type closableGrpcClientConnInterface interface {
	grpc.ClientConnInterface
	io.Closer
}

func configureTLSOption(tlsMode string) grpc.DialOption {
	switch tlsMode {
	case "none":
		return grpc.WithInsecure()
	case "system":
		c, err := x509.SystemCertPool()
		if err != nil {
			log.WithError(err).Fatal("cannot load system certificates")
		}
		return grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(c, ""))
	default:
		credentials, err := credentials.NewClientTLSFromFile(tlsMode, "")
		if err != nil {
			log.WithError(err).Fatal("cannot load specfifed certificates")
		}
		return grpc.WithTransportCredentials(credentials)
	}
}

func dial() (res closableGrpcClientConnInterface) {
	var err error
	tlsDialOption := configureTLSOption(rootCmdOpts.TLSMode)
	switch rootCmdOpts.DialMode {
	case dialModeHost:
		res, err = grpc.Dial(rootCmdOpts.Host, tlsDialOption)
	case dialModeKubernetes:
		res, err = dialKubernetes(tlsDialOption)
	default:
		log.Fatalf("unknown dial mode: %s", rootCmdOpts.DialMode)
	}
	if err != nil {
		log.WithError(err).Fatal("cannot connect to werft server")
	}
	return
}

func getRequestContext(md *v1.JobMetadata) (ctx context.Context, cancel context.CancelFunc, err error) {
	reqMD := make(metadata.MD)
	if rootCmdOpts.CredentialHelper != "" {
		var (
			m      jsonpb.Marshaler
			mdJSON string
		)
		if md != nil {
			mdJSON, err = m.MarshalToString(md)
			if err != nil {
				return nil, nil, err
			}
		}

		cmd := exec.Command(rootCmdOpts.CredentialHelper)
		cmd.Stdin = bytes.NewReader([]byte(mdJSON))
		out, err := cmd.CombinedOutput()
		log.WithField("input", string(mdJSON)).WithError(err).WithField("output", string(out)).Debug("ran credential helper")
		if err != nil {
			return nil, nil, err
		}

		token := strings.TrimSpace(string(out))
		reqMD.Set("x-auth-token", token)
	}

	ctx, cancel = context.WithCancel(context.Background())
	ctx = metadata.NewOutgoingContext(ctx, reqMD)

	return
}

func getLocalJobName(client v1.WerftServiceClient, args []string) (jobname string, md *v1.JobMetadata, err error) {
	var (
		name            string
		localJobContext *v1.JobMetadata
	)
	if len(args) == 0 {
		wd, err := os.Getwd()
		if err != nil {
			return "", nil, err
		}
		localJobContext, err = getLocalJobContext(wd, v1.JobTrigger_TRIGGER_MANUAL)
		if err != nil {
			return "", nil, fmt.Errorf("cannot find local job context: %w", err)
		}
		var cancel context.CancelFunc

		ctx, _, err := getRequestContext(localJobContext)
		if err != nil {
			return "", nil, err
		}
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		name, err = findJobByMetadata(ctx, localJobContext, client)
		cancel()
		if err != nil {
			return "", nil, err
		}
		if name == "" {
			return "", nil, fmt.Errorf("no job found - please specify job name")
		}

		fmt.Printf("re-running \033[34m\033[1m%s\t\033\033[0m\n", name)
	} else {
		name = args[0]
	}
	return name, localJobContext, nil
}

func dialKubernetes(dialOption grpc.DialOption) (closableGrpcClientConnInterface, error) {
	kubecfg, namespace, err := getKubeconfig(rootCmdOpts.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("cannot load kubeconfig %s: %w", rootCmdOpts.Kubeconfig, err)
	}
	if rootCmdOpts.K8sNamespace != "" {
		namespace = rootCmdOpts.K8sNamespace
	}

	clientSet, err := kubernetes.NewForConfig(kubecfg)
	if err != nil {
		return nil, err
	}

	pod, err := findWerftPod(clientSet, namespace, rootCmdOpts.K8sLabelSelector)
	if err != nil {
		return nil, fmt.Errorf("cannot find werft pod: %w", err)
	}

	localPort, err := findFreeLocalPort()
	if err != nil {
		return nil, fmt.Errorf("cannot find free port: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	readychan, errchan := forwardPort(ctx, kubecfg, namespace, pod, fmt.Sprintf("%d:%s", localPort, rootCmdOpts.K8sPodPort))
	select {
	case err := <-errchan:
		cancel()
		return nil, err
	case <-readychan:
	}

	res, err := grpc.Dial(fmt.Sprintf("localhost:%d", localPort), dialOption)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("cannot dial forwarded connection: %w", err)
	}

	return closableConn{
		ClientConnInterface: res,
		Closer:              func() error { cancel(); return nil },
	}, nil
}

type closableConn struct {
	grpc.ClientConnInterface
	Closer func() error
}

func (c closableConn) Close() error {
	return c.Closer()
}

func findFreeLocalPort() (int, error) {
	const (
		start = 30000
		end   = 60000
	)
	for p := start; p <= end; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err == nil {
			l.Close()
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free local port found")
}

// GetKubeconfig loads kubernetes connection config from a kubeconfig file
func getKubeconfig(kubeconfig string) (res *rest.Config, namespace string, err error) {
	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{},
	)
	namespace, _, err = cfg.Namespace()
	if err != nil {
		return nil, "", err
	}

	res, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, namespace, err
	}

	return res, namespace, nil
}

// findWerftPod returns the first pod we found for a particular component
func findWerftPod(clientSet kubernetes.Interface, namespace, selector string) (podName string, err error) {
	pods, err := clientSet.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return "", err
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pod in %s with label component=%s", namespace, selector)
	}
	return pods.Items[0].Name, nil
}

// ForwardPort establishes a TCP port forwarding to a Kubernetes pod
func forwardPort(ctx context.Context, config *rest.Config, namespace, pod, port string) (readychan chan struct{}, errchan chan error) {
	errchan = make(chan error, 1)
	readychan = make(chan struct{}, 1)

	roundTripper, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		errchan <- err
		return
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, pod)
	hostIP := strings.TrimLeft(config.Host, "https://")
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)

	stopChan := make(chan struct{}, 1)
	fwdReadyChan := make(chan struct{}, 1)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	forwarder, err := portforward.New(dialer, []string{port}, stopChan, fwdReadyChan, out, errOut)
	if err != nil {
		panic(err)
	}

	var once sync.Once
	go func() {
		err := forwarder.ForwardPorts()
		if err != nil {
			errchan <- err
		}
		once.Do(func() { close(readychan) })
	}()

	go func() {
		select {
		case <-readychan:
			// we're out of here
		case <-ctx.Done():
			close(stopChan)
		}
	}()

	go func() {
		for range fwdReadyChan {
		}

		if errOut.Len() != 0 {
			errchan <- fmt.Errorf(errOut.String())
			return
		}

		once.Do(func() { close(readychan) })
	}()

	return
}

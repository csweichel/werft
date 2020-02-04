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
	"database/sql"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/32leaves/werft/pkg/executor"
	"github.com/32leaves/werft/pkg/logcutter"
	plugin "github.com/32leaves/werft/pkg/plugin/host"
	"github.com/32leaves/werft/pkg/store"
	"github.com/32leaves/werft/pkg/store/postgres"
	"github.com/32leaves/werft/pkg/werft"
	rice "github.com/GeertJohan/go.rice"
	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/github"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run <config.json>",
	Short: "Starts the werft server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if v, _ := cmd.Flags().GetBool("verbose"); v {
			log.SetLevel(log.DebugLevel)
		}

		fc, err := ioutil.ReadFile(args[0])
		if err != nil {
			return err
		}

		var cfg Config
		err = yaml.Unmarshal(fc, &cfg)
		if err != nil {
			return err
		}

		log.Info("connecting to database")
		db, err := sql.Open("postgres", cfg.Storage.JobStore)
		if err != nil {
			return err
		}
		maxConns := 10
		maxIdleConns := 2
		if cfg.Storage.JobStoreMaxConnections > 0 {
			maxConns = cfg.Storage.JobStoreMaxConnections
		}
		if cfg.Storage.JobStoreMaxIdleConnections > 0 {
			maxIdleConns = cfg.Storage.JobStoreMaxIdleConnections
		}
		log.WithField("maxOpenConns", maxConns).WithField("maxIdleConns", maxIdleConns).Debug("setting max open connections on job store DB")
		db.SetMaxOpenConns(maxConns)
		db.SetMaxIdleConns(maxIdleConns)
		err = db.Ping()
		if err != nil {
			return err
		}

		log.Info("making sure database schema is up to date")
		err = postgres.Migrate(db)
		if err != nil {
			return err
		}
		jobStore, err := postgres.NewJobStore(db)
		if err != nil {
			return err
		}
		nrGroups, err := postgres.NewNumberGroup(db)
		if err != nil {
			return err
		}

		var kubeConfig *rest.Config
		if cfg.Kubeconfig == "" {
			kubeConfig, err = rest.InClusterConfig()
			if err != nil {
				return err
			}
			kubeConfig.RateLimiter = &unlimitedRateLimiter{}
		} else {
			kubeConfig, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
			if err != nil {
				return err
			}
		}

		ghtr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, cfg.GitHub.AppID, cfg.GitHub.InstallationID, cfg.GitHub.PrivateKeyPath)
		if err != nil {
			return err
		}
		ghClient := github.NewClient(&http.Client{Transport: ghtr})

		execCfg := cfg.Executor
		if execCfg.Namespace == "" {
			execCfg.Namespace = "default"
		}

		logStore, err := store.NewFileLogStore(cfg.Storage.LogStore)
		if err != nil {
			return err
		}

		uiservice, err := werft.NewUIService(ghClient, cfg.Service.JobSpecRepos)
		if err != nil {
			return err
		}

		log.Info("connecting to kubernetes")
		exec, err := executor.NewExecutor(execCfg, kubeConfig)
		if err != nil {
			return err
		}
		exec.Run()
		service := &werft.Service{
			Logs:     logStore,
			Jobs:     jobStore,
			Groups:   nrGroups,
			Executor: exec,
			Cutter:   logcutter.DefaultCutter,
			GitHub: werft.GitHubSetup{
				WebhookSecret: []byte(cfg.GitHub.WebhookSecret),
				Client:        ghClient,
				Auth: func(ctx context.Context) (user string, pass string, err error) {
					tkn, err := ghtr.Token(ctx)
					if err != nil {
						return
					}
					user = "x-access-token"
					pass = tkn
					return
				},
			},
			Config: cfg.Werft,
		}
		if val, _ := cmd.Flags().GetString("debug-webui-proxy"); val != "" {
			cfg.Werft.DebugProxy = val
		}
		err = service.Start()
		if err != nil {
			log.WithError(err).Fatal("cannot start service")
		}

		grpcServer := grpc.NewServer(
			// We don't know how good our cients are at closing connections. If they don't close them properly
			// we'll be leaking goroutines left and right. Closing Idle connections should prevent that.
			// If a client gets disconnected because nothing happened for 15 minutes (e.g. no log output, no new job),
			// the client can simply reconnect if they're still interested. WebUI is pretty good at maintaining
			// connections anyways.
			grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: 15 * time.Minute}),
		)
		v1.RegisterWerftServiceServer(grpcServer, service)
		v1.RegisterWerftUIServer(grpcServer, uiservice)
		go startGRPC(grpcServer, fmt.Sprintf(":%d", cfg.Service.GRPCPort))
		go startWeb(service, grpcServer, fmt.Sprintf(":%d", cfg.Service.WebPort), cfg.Werft.DebugProxy)
		if cfg.Service.PromPort != 0 {
			go startPrometheus(fmt.Sprintf(":%d", cfg.Service.PromPort),
				jobStore.RegisterPrometheusMetrics,
				service.RegisterPrometheusMetrics,
			)
		}
		if cfg.Service.PprofPort != 0 {
			go startPProf(fmt.Sprintf(":%d", cfg.Service.PprofPort))
		}

		plugins, err := plugin.Start(cfg.Plugins, service)
		if err != nil {
			log.WithError(err).Fatal("cannot start plugins")
		}
		go func() {
			for e := range plugins.Errchan {
				log.WithError(e.Err).WithField("plugin", e.Reg.Name).Warn("plugin error")
			}
		}()
		defer plugins.Stop()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		log.Info("werft is up and running. Stop with SIGINT or CTRL+C")
		<-sigChan
		log.Info("Received SIGINT - shutting down")

		return nil
	},
}

// startWeb starts the werft web UI service
func startWeb(srv *werft.Service, grpcServer *grpc.Server, addr string, debugProxy string) {
	var webuiServer http.Handler
	if debugProxy != "" {
		tgt, err := url.Parse(debugProxy)
		if err != nil {
			// this is debug only - it's ok to panic
			panic(err)
		}

		log.WithField("target", tgt).Debug("proxying to webui server")
		webuiServer = httputil.NewSingleHostReverseProxy(tgt)
	} else {
		// WebUI is a single-page app, hence any path that does not resolve to a static file must result in /index.html.
		// As a (rather crude) fix we intercept the response writer to find out if the FileServer returned an error. If so
		// we return /index.html instead.
		dws := http.FileServer(rice.MustFindBox("../../pkg/webui/build").HTTPBox())
		webuiServer = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dws.ServeHTTP(&interceptResponseWriter{
				ResponseWriter: w,
				errH: func(rw http.ResponseWriter, code int) {
					r.URL.Path = "/"
					rw.Header().Set("Content-Type", "text/html; charset=utf-8")
					dws.ServeHTTP(rw, r)
				},
			}, r)
		})
	}

	grpcWebServer := grpcweb.WrapServer(grpcServer)

	mux := http.NewServeMux()
	mux.HandleFunc("/github/app", srv.HandleGithubWebhook)
	mux.Handle("/", hstsHandler(
		grpcTrafficSplitter(
			webuiServer,
			grpcWebServer,
		),
	))

	log.WithField("addr", addr).Info("serving werft web service")
	err := http.ListenAndServe(addr, mux)
	if err != nil {
		log.WithField("addr", addr).WithError(err).Warn("cannot serve web service")
	}
}

// startGRPC starts the werft GRPC service
func startGRPC(srv *grpc.Server, addr string) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.WithError(err).Error("cannot start GRPC server")
	}

	log.WithField("addr", addr).Info("serving werft GRPC service")
	err = srv.Serve(lis)
	if err != nil {
		log.WithError(err).Error("cannot start GRPC server")
	}
}

// startPrometheus starts a Prometheus metrics server on addr.
func startPrometheus(addr string, regfuncs ...func(prometheus.Registerer)) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	)
	for _, f := range regfuncs {
		f(reg)
	}

	handler := http.NewServeMux()
	handler.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	log.WithField("addr", addr).Info("started Prometheus metrics server")
	err := http.ListenAndServe(addr, handler)
	if err != nil {
		log.WithError(err).Fatal("cannot start Prometheus metrics server")
	}
}

// startPProf starts a pprof server on addr
func startPProf(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	log.WithField("addr", addr).Info("serving pprof service")
	err := http.ListenAndServe(addr, mux)
	if err != nil {
		log.WithField("addr", addr).WithError(err).Warn("cannot serve pprof service")
	}
}

// hstsHandler wraps an http.HandlerFunc sfuch that it sets the HSTS header.
func hstsHandler(fn http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		fn(w, r)
	})
}

func grpcTrafficSplitter(fallback http.Handler, wrappedGrpc *grpcweb.WrappedGrpcServer) http.HandlerFunc {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if wrappedGrpc.IsGrpcWebRequest(req) || wrappedGrpc.IsAcceptableGrpcCorsRequest(req) {
			wrappedGrpc.ServeHTTP(resp, req)
		} else {
			// Fall back to other servers.
			fallback.ServeHTTP(resp, req)
		}
	})
}

type interceptResponseWriter struct {
	http.ResponseWriter
	errH func(http.ResponseWriter, int)
}

func (w *interceptResponseWriter) WriteHeader(status int) {
	if status >= http.StatusBadRequest {
		w.errH(w.ResponseWriter, status)
		w.errH = nil
	} else {
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *interceptResponseWriter) Write(p []byte) (n int, err error) {
	if w.errH == nil {
		return len(p), nil
	}
	return w.ResponseWriter.Write(p)
}

// unlimitedRateLimiter removes all client side rate limits
type unlimitedRateLimiter struct{}

// TryAccept returns true if a token is taken immediately. Otherwise,
// it returns false.
func (*unlimitedRateLimiter) TryAccept() bool {
	return true
}

// Accept returns once a token becomes available.
func (*unlimitedRateLimiter) Accept() {
	return
}

// Stop stops the rate limiter, subsequent calls to CanAccept will return false
func (*unlimitedRateLimiter) Stop() {
	return
}

// QPS returns QPS of this rate limiter
func (*unlimitedRateLimiter) QPS() float32 {
	return math.MaxFloat32
}

// Wait returns nil if a token is taken before the Context is done.
func (*unlimitedRateLimiter) Wait(ctx context.Context) error {
	return nil
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().String("debug-webui-proxy", "", "proxies the web UI to this address")
	runCmd.Flags().Bool("verbose", false, "enable verbose debug output")
}

// Config configures the werft server
type Config struct {
	Werft   werft.Config `yaml:"werft"`
	Service struct {
		WebPort      int      `yaml:"webPort"`
		GRPCPort     int      `yaml:"grpcPort"`
		PromPort     int      `yaml:"prometheusPort,omitempty"`
		PprofPort    int      `yaml:"pprofPort,omitempty"`
		JobSpecRepos []string `yaml:"jobSpecRepos"`
	}
	Storage struct {
		LogStore                   string `yaml:"logsPath"`
		JobStore                   string `yaml:"jobsConnectionString"`
		JobStoreMaxConnections     int    `yaml:"jobsMaxConnections"`
		JobStoreMaxIdleConnections int    `yaml:"jobsMaxIdleConnections"`
	} `yaml:"storage"`
	Executor   executor.Config `yaml:"executor"`
	Kubeconfig string          `yaml:"kubeconfig,omitempty"`
	GitHub     struct {
		WebhookSecret  string `yaml:"webhookSecret"`
		PrivateKeyPath string `yaml:"privateKeyPath"`
		InstallationID int64  `yaml:"installationID,omitempty"`
		AppID          int64  `yaml:"appID"`
	} `yaml:"github"`
	Plugins plugin.Config
}

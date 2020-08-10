package host

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/csweichel/werft/pkg/plugin/common"
	"github.com/csweichel/werft/pkg/werft"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

// Registration registers a plugin
type Registration struct {
	Name    string        `yaml:"name"`
	Command []string      `yaml:"command"`
	Type    []common.Type `yaml:"type"`
	Config  yaml.Node     `yaml:"config"`
}

// Config configures the plugin system
type Config []Registration

// Plugins represents an initialized plugin system
type Plugins struct {
	Errchan chan Error

	stopchan chan struct{}
	stopwg   sync.WaitGroup

	sockets      map[string]string
	werftService v1.WerftServiceServer
	repoProvider *compoundRepositoryProvider
}

// RepositoryProvider provides access to all repo providers contributed via plugins
func (p *Plugins) RepositoryProvider() werft.RepositoryProvider {
	return p.repoProvider
}

// Stop stops all plugins
func (p *Plugins) Stop() {
	close(p.stopchan)
	p.stopwg.Wait()

	for _, s := range p.sockets {
		os.Remove(s)
	}
}

// Error is passed down the plugins error chan
type Error struct {
	Err error
	Reg *Registration
}

// Start starts all configured plugins
func Start(cfg Config, srv v1.WerftServiceServer) (*Plugins, error) {
	errchan, stopchan := make(chan Error), make(chan struct{})

	plugins := &Plugins{
		Errchan:      errchan,
		stopchan:     stopchan,
		sockets:      make(map[string]string),
		werftService: srv,
		repoProvider: &compoundRepositoryProvider{},
	}

	for _, pr := range cfg {
		err := plugins.startPlugin(pr)
		if err != nil {
			return nil, xerrors.Errorf("cannot start plugin %s: %w", pr.Name, err)
		}
	}

	return plugins, nil
}

func (p *Plugins) socketFor(t common.Type) (string, error) {
	switch t {
	case common.TypeIntegration:
		return p.socketForIntegrationPlugin()
	case common.TypeRepository:
		return p.sockerForRepositoryPlugin()
	default:
		return "", xerrors.Errorf("unknown plugin type %s", t)
	}
}

func (p *Plugins) socketForIntegrationPlugin() (string, error) {
	if socket, ok := p.sockets[string(common.TypeIntegration)]; ok {
		return socket, nil
	}

	socketFN := filepath.Join(os.TempDir(), fmt.Sprintf("werft-plugin-integration-%d.sock", time.Now().UnixNano()))
	lis, err := net.Listen("unix", socketFN)
	if err != nil {
		return "", xerrors.Errorf("cannot start integration plugin server: %w", err)
	}
	s := grpc.NewServer()
	v1.RegisterWerftServiceServer(s, p.werftService)
	go func() {
		err := s.Serve(lis)
		if err != nil {
			p.Errchan <- Error{Err: err}
		}
		delete(p.sockets, string(common.TypeIntegration))
	}()

	go func() {
		p.stopwg.Add(1)
		defer p.stopwg.Done()

		<-p.stopchan
		s.Stop()
	}()

	p.sockets[string(common.TypeIntegration)] = socketFN
	return socketFN, nil
}

func (p *Plugins) sockerForRepositoryPlugin() (string, error) {
	return filepath.Join(os.TempDir(), fmt.Sprintf("werft-plugin-repo-%d.sock", time.Now().UnixNano())), nil
}

func (p *Plugins) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	segs := strings.Split(req.URL.Path, "/")
	plgn := segs[0]
	skt, ok := p.sockets[plgn]
	if !ok {
		resp.WriteHeader(http.StatusNotFound)
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "localhost"

			pth := req.URL.Path
			pth = strings.TrimPrefix(pth, "/")
			pth = strings.TrimPrefix(pth, plgn)
			req.URL.Path = pth
		},
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", skt)
			},
		},
	}
	proxy.ServeHTTP(resp, req)
}

func (p *Plugins) startPlugin(reg Registration) error {
	cfgfile, err := ioutil.TempFile(os.TempDir(), "werft-plugin-cfg")
	if err != nil {
		return xerrors.Errorf("cannot create plugin config: %w", err)
	}
	err = yaml.NewEncoder(cfgfile).Encode(&reg.Config)
	if err != nil {
		return xerrors.Errorf("cannot write plugin config: %w", err)
	}
	err = cfgfile.Close()
	if err != nil {
		return xerrors.Errorf("cannot write plugin config: %w", err)
	}

	for _, t := range reg.Type {
		socket, err := p.socketFor(t)
		if err != nil {
			return err
		}

		pluginName := fmt.Sprintf("%s-%s", reg.Name, t)
		pluginLog := log.WithField("plugin", pluginName)
		stdout := pluginLog.WriterLevel(log.InfoLevel)
		stderr := pluginLog.WriterLevel(log.ErrorLevel)

		env := os.Environ()
		if t == common.TypeIntegration {
			skt := filepath.Join(os.TempDir(), fmt.Sprintf("werft-plugin-proxy-%d.sock", time.Now().UnixNano()))
			env = append(env, fmt.Sprintf("WERFT_PLUGIN_PROXY_SOCKET=%s", skt))
			p.sockets[reg.Name] = skt
		}

		var (
			command string
			args    []string
		)
		if len(reg.Command) > 0 {
			command = reg.Command[0]
			args = reg.Command[1:]
		} else {
			command = fmt.Sprintf("werft-plugin-%s", reg.Name)
		}
		args = append(args, string(t), cfgfile.Name(), socket)

		cmd := exec.Command(command, args...)
		cmd.Env = env
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		err = cmd.Start()
		if err != nil {
			stdout.Close()
			stderr.Close()
			return err
		}
		pluginLog.Info("plugin started")

		var mayFail bool
		go func() {
			err := cmd.Wait()
			if err != nil && !mayFail {
				p.Errchan <- Error{
					Err: err,
					Reg: &reg,
				}
			}

			stdout.Close()
			stderr.Close()
		}()
		go func() {
			p.stopwg.Add(1)
			defer p.stopwg.Done()

			<-p.stopchan
			pluginLog.Info("stopping plugin")
			mayFail = true
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		}()

		if t == common.TypeRepository {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// repo plugins register repo provider at some point - listen for that
			err := p.tryAndRegisterRepoProvider(ctx, pluginLog, socket)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *Plugins) tryAndRegisterRepoProvider(ctx context.Context, pluginLog *log.Entry, socket string) error {
	var (
		t        = time.NewTicker(2 * time.Second)
		firstrun = make(chan struct{}, 1)
		conn     *grpc.ClientConn
		err      error
	)
	firstrun <- struct{}{}

	defer t.Stop()
	for {
		select {
		case <-firstrun:
		case <-t.C:
		case <-ctx.Done():
			return ctx.Err()
		case <-p.stopchan:
			return nil
		}

		conn, err = grpc.Dial("unix://"+socket, grpc.WithInsecure())
		if err != nil {
			pluginLog.Debug("cannot connect to socket (yet)")
			continue
		}
		client := common.NewRepositoryPluginClient(conn)
		rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		host, err := client.RepoHost(rctx, &common.RepoHostRequest{})
		cancel()
		if err != nil {
			conn.Close()
			pluginLog.WithError(err).Debug("cannot connect to socket (yet)")
			continue
		}

		pluginLog.WithField("host", host.Host).Info("registered repo provider")
		p.repoProvider.registerProvider(host.Host, &pluginHostProvider{client})
		return nil
	}
}

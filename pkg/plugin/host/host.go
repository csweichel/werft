package host

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/32leaves/werft/pkg/plugin/common"
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

	stopchan     chan struct{}
	sockets      map[string]string
	werftService v1.WerftServiceServer
}

// Stop stops all plugins
func (p *Plugins) Stop() {
	// TODO: backsync stopping using waitgroup
	close(p.stopchan)

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
	}

	for _, pr := range cfg {
		err := plugins.startPlugin(pr)
		if err != nil {
			return nil, xerrors.Errorf("cannot start integration plugin %s: %w", pr.Name, err)
		}
	}

	return plugins, nil
}

func (p *Plugins) socketFor(t common.Type) (string, error) {
	switch t {
	case common.TypeIntegration:
		return p.socketForIntegrationPlugin()
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
		return "", xerrors.Errorf("cannot start inegration plugin server: %w", err)
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
		<-p.stopchan
		s.GracefulStop()
	}()

	p.sockets[string(common.TypeIntegration)] = socketFN
	return socketFN, nil
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
		cmd.Env = os.Environ()
		cmd.Stdout = stdout
		cmd.Stderr = stderr
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
			<-p.stopchan
			mayFail = true
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		}()
	}

	return nil
}

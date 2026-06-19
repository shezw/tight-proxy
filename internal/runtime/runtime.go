package runtime

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/shezw/tight-proxy/internal/config"
	"github.com/shezw/tight-proxy/internal/proxy"
	"github.com/shezw/tight-proxy/internal/relay"
	"github.com/shezw/tight-proxy/internal/systemproxy"
)

type State struct {
	Running        bool          `json:"running"`
	Address        string        `json:"address"`
	RelayAddresses []string      `json:"relayAddresses"`
	Config         config.Config `json:"config"`
	Whitelist      string        `json:"whitelist"`
	WhitelistPath  string        `json:"whitelistPath"`
}

type Runtime struct {
	configPath string
	mu         sync.Mutex
	proxy      *proxy.Server
	relay      *relay.Server
	system     systemproxy.Manager
}

func New(configPath string) *Runtime {
	return &Runtime{configPath: configPath, system: systemproxy.New()}
}

func (r *Runtime) State() (State, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stateLocked()
}

func (r *Runtime) Save(cfg config.Config, whitelistText string) (State, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := config.Save(r.configPath, cfg); err != nil {
		return State{}, err
	}
	cfg = config.Normalize(cfg)
	if err := config.SaveWhitelist(cfg, r.configPath, whitelistText); err != nil {
		return State{}, err
	}
	if r.proxy != nil {
		if err := r.stopLocked(); err != nil {
			return State{}, err
		}
		if err := r.startLocked(); err != nil {
			return State{}, err
		}
	}
	return r.stateLocked()
}

func (r *Runtime) StartProxy() (State, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.proxy == nil {
		if err := r.startLocked(); err != nil {
			return State{}, err
		}
	}
	return r.stateLocked()
}

func (r *Runtime) StopProxy() (State, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.stopLocked(); err != nil {
		return State{}, err
	}
	return r.stateLocked()
}

func (r *Runtime) startLocked() error {
	cfg, err := config.Load(r.configPath)
	if err != nil {
		return err
	}
	cfg.Enabled = true
	if err := config.Save(r.configPath, cfg); err != nil {
		return err
	}
	whitelist, err := config.LoadWhitelist(cfg, r.configPath)
	if err != nil {
		return err
	}
	server := proxy.New(cfg, whitelist)
	if _, err := server.Start(); err != nil {
		return err
	}
	relayServer := relay.New(cfg.Relay)
	if _, err := relayServer.Start(); err != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Stop(ctx)
		return err
	}
	if err := r.system.Enable(cfg); err != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Stop(ctx)
		_ = relayServer.Stop()
		return err
	}
	r.proxy = server
	r.relay = relayServer
	return nil
}

func (r *Runtime) stopLocked() error {
	if r.proxy == nil && r.relay == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	if r.proxy != nil {
		err = r.proxy.Stop(ctx)
	}
	r.proxy = nil
	if r.relay != nil {
		if relayErr := r.relay.Stop(); err == nil {
			err = relayErr
		}
	}
	r.relay = nil
	if restoreErr := r.system.Restore(); err == nil {
		err = restoreErr
	}
	return err
}

func (r *Runtime) stateLocked() (State, error) {
	cfg, err := config.Load(r.configPath)
	if err != nil {
		return State{}, err
	}
	whitelist, err := config.LoadWhitelist(cfg, r.configPath)
	if err != nil {
		return State{}, err
	}
	var address string
	var relayAddresses []string
	if r.proxy != nil {
		address = addrString(r.proxy.Addr())
	}
	if r.relay != nil {
		for _, addr := range r.relay.Addrs() {
			relayAddresses = append(relayAddresses, addrString(addr))
		}
	}
	return State{
		Running:        r.proxy != nil,
		Address:        address,
		RelayAddresses: relayAddresses,
		Config:         cfg,
		Whitelist:      config.WhitelistText(whitelist),
		WhitelistPath:  config.Resolve(r.configPath, cfg.WhitelistFile),
	}, nil
}

func addrString(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	return addr.String()
}

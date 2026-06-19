//go:build darwin

package systemproxy

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"

	"github.com/shezw/tight-proxy/internal/config"
)

type proxyState struct {
	enabled bool
	server  string
	port    string
}

type serviceState struct {
	web    proxyState
	secure proxyState
}

type platformManager struct {
	hasBackup bool
	services  map[string]serviceState
}

func newPlatformManager() Manager {
	return &platformManager{}
}

func (m *platformManager) Enable(cfg config.Config) error {
	services, err := networkServices()
	if err != nil {
		return err
	}
	if !m.hasBackup {
		m.services = make(map[string]serviceState)
		for _, service := range services {
			web, _ := readProxyState(service, "getwebproxy")
			secure, _ := readProxyState(service, "getsecurewebproxy")
			m.services[service] = serviceState{web: web, secure: secure}
		}
		m.hasBackup = true
	}

	host := cfg.Listen.Host
	if host == "" || host == "localhost" {
		host = "127.0.0.1"
	}
	port := strconv.Itoa(cfg.Listen.Port)
	for _, service := range services {
		if err := runNetworkSetup("setwebproxy", service, host, port); err != nil {
			return err
		}
		if err := runNetworkSetup("setwebproxystate", service, "on"); err != nil {
			return err
		}
		if err := runNetworkSetup("setsecurewebproxy", service, host, port); err != nil {
			return err
		}
		if err := runNetworkSetup("setsecurewebproxystate", service, "on"); err != nil {
			return err
		}
		_ = runNetworkSetup("setproxybypassdomains", service, "localhost", "127.0.0.1", "::1")
	}
	return nil
}

func (m *platformManager) Restore() error {
	if !m.hasBackup {
		return nil
	}
	for service, state := range m.services {
		if err := restoreProxyState(service, "web", state.web); err != nil {
			return err
		}
		if err := restoreProxyState(service, "secureweb", state.secure); err != nil {
			return err
		}
	}
	m.hasBackup = false
	return nil
}

func restoreProxyState(service string, kind string, state proxyState) error {
	setProxy := "set" + kind + "proxy"
	setState := "set" + kind + "proxystate"
	if state.server != "" && state.port != "" {
		if err := runNetworkSetup(setProxy, service, state.server, state.port); err != nil {
			return err
		}
	}
	value := "off"
	if state.enabled {
		value = "on"
	}
	return runNetworkSetup(setState, service, value)
}

func networkServices() ([]string, error) {
	out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return nil, err
	}
	var services []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "An asterisk") || strings.HasPrefix(line, "*") {
			continue
		}
		services = append(services, line)
	}
	return services, nil
}

func readProxyState(service string, action string) (proxyState, error) {
	out, err := exec.Command("networksetup", "-"+action, service).Output()
	if err != nil {
		return proxyState{}, err
	}
	state := proxyState{}
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "Enabled":
			state.enabled = strings.EqualFold(value, "Yes")
		case "Server":
			state.server = value
		case "Port":
			state.port = value
		}
	}
	return state, nil
}

func runNetworkSetup(args ...string) error {
	fullArgs := make([]string, 0, len(args)+1)
	for index, arg := range args {
		if index == 0 {
			fullArgs = append(fullArgs, "-"+arg)
		} else {
			fullArgs = append(fullArgs, arg)
		}
	}
	cmd := exec.Command("networksetup", fullArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return &commandError{err: err, stderr: strings.TrimSpace(stderr.String())}
		}
		return err
	}
	return nil
}

type commandError struct {
	err    error
	stderr string
}

func (e *commandError) Error() string {
	return e.err.Error() + ": " + e.stderr
}

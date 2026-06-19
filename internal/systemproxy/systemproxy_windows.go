//go:build windows

package systemproxy

import (
	"fmt"
	"net"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows/registry"

	"github.com/shezw/tight-proxy/internal/config"
)

const internetSettingsPath = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

type platformManager struct {
	hasBackup     bool
	proxyEnable   uint64
	proxyServer   string
	proxyOverride string
}

func newPlatformManager() Manager {
	return &platformManager{}
}

func (m *platformManager) Enable(cfg config.Config) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if !m.hasBackup {
		m.proxyEnable, _, _ = key.GetIntegerValue("ProxyEnable")
		m.proxyServer, _, _ = key.GetStringValue("ProxyServer")
		m.proxyOverride, _, _ = key.GetStringValue("ProxyOverride")
		m.hasBackup = true
	}

	addr := net.JoinHostPort(cfg.Listen.Host, strconv.Itoa(cfg.Listen.Port))
	if cfg.Listen.Host == "127.0.0.1" || cfg.Listen.Host == "localhost" || cfg.Listen.Host == "" {
		addr = "127.0.0.1:" + strconv.Itoa(cfg.Listen.Port)
	}
	if err := key.SetDWordValue("ProxyEnable", 1); err != nil {
		return err
	}
	if err := key.SetStringValue("ProxyServer", addr); err != nil {
		return err
	}
	if err := key.SetStringValue("ProxyOverride", "localhost;127.0.0.1;::1;<local>"); err != nil {
		return err
	}
	return notifyWinINet()
}

func (m *platformManager) Restore() error {
	if !m.hasBackup {
		return nil
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if err := key.SetDWordValue("ProxyEnable", uint32(m.proxyEnable)); err != nil {
		return err
	}
	if m.proxyServer == "" {
		_ = key.DeleteValue("ProxyServer")
	} else if err := key.SetStringValue("ProxyServer", m.proxyServer); err != nil {
		return err
	}
	if m.proxyOverride == "" {
		_ = key.DeleteValue("ProxyOverride")
	} else if err := key.SetStringValue("ProxyOverride", m.proxyOverride); err != nil {
		return err
	}
	m.hasBackup = false
	return notifyWinINet()
}

func notifyWinINet() error {
	wininet := syscall.NewLazyDLL("wininet.dll")
	internetSetOption := wininet.NewProc("InternetSetOptionW")
	for _, option := range []uintptr{39, 37} {
		ret, _, err := internetSetOption.Call(0, option, 0, 0)
		if ret == 0 {
			return fmt.Errorf("InternetSetOptionW(%d): %w", option, err)
		}
	}
	return nil
}

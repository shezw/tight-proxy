//go:build !windows && !darwin

package systemproxy

import "github.com/shezw/tight-proxy/internal/config"

type platformManager struct{}

func newPlatformManager() Manager {
	return &platformManager{}
}

func (m *platformManager) Enable(_ config.Config) error {
	return nil
}

func (m *platformManager) Restore() error {
	return nil
}

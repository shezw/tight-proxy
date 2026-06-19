package systemproxy

import "github.com/shezw/tight-proxy/internal/config"

type Manager interface {
	Enable(cfg config.Config) error
	Restore() error
}

func New() Manager {
	return newPlatformManager()
}

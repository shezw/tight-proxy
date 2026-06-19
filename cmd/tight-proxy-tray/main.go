package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/shezw/tight-proxy/internal/config"
	"github.com/shezw/tight-proxy/internal/control"
	rtpkg "github.com/shezw/tight-proxy/internal/runtime"
	"github.com/shezw/tight-proxy/internal/tray"
	"github.com/shezw/tight-proxy/internal/webui"
)

func main() {
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rt := rtpkg.New(cfgPath)
	root, err := webui.FS()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	controlServer := control.New(rt, cfg.ControlListen, root)
	if _, err := controlServer.Start(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	controlURL := "http://" + cfg.ControlListen.Host + ":" + strconv.Itoa(cfg.ControlListen.Port) + "/"
	tray.Run(rt, controlURL)
	_ = controlServer.Stop()
}

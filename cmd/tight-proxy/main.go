package main

import (
	"encoding/json"
	"flag"
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
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printHelp()
		return nil
	}
	switch args[0] {
	case "init":
		return initConfig(args[1:])
	case "check":
		return check(args[1:])
	case "start":
		return start(args[1:])
	case "web":
		return web(args[1:], false)
	case "tray":
		return web(args[1:], true)
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func printHelp() {
	fmt.Print(`tight-proxy

Usage:
  tight-proxy init [--config ./tight-proxy.config.json]
  tight-proxy check [--config ./tight-proxy.config.json]
  tight-proxy start [--config ./tight-proxy.config.json]
  tight-proxy web [--config ./tight-proxy.config.json]
  tight-proxy tray [--config ./tight-proxy.config.json]

Options:
  --config <path>       Config file path
  --listen-host <host>  Local proxy entry host
  --listen-port <port>  Local proxy entry port
  --upstream <url>      http://user:pass@127.0.0.1:8080, https://..., ftp://..., socks5://...
  --ui-host <host>      Control panel host
  --ui-port <port>      Control panel port
`)
}

func flagSet(name string, args []string) (*flag.FlagSet, *string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "config file")
	return fs, cfgPath, fs.Parse(args)
}

func initConfig(args []string) error {
	fs, cfgPath, err := flagSet("init", args)
	if err != nil {
		return err
	}
	_ = fs
	cfg := config.Default()
	if err := config.Save(*cfgPath, cfg); err != nil {
		return err
	}
	if err := config.SaveWhitelist(cfg, *cfgPath, "# One domain per line. Empty means all domains use upstream.\nexample.com\n"); err != nil {
		return err
	}
	fmt.Printf("created %s\n", *cfgPath)
	return nil
}

func loadWithFlags(name string, args []string) (config.Config, string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "config file")
	listenHost := fs.String("listen-host", "", "proxy listen host")
	listenPort := fs.Int("listen-port", 0, "proxy listen port")
	uiHost := fs.String("ui-host", "", "control listen host")
	uiPort := fs.Int("ui-port", 0, "control listen port")
	upstream := fs.String("upstream", "", "upstream URL")
	if err := fs.Parse(args); err != nil {
		return config.Config{}, "", err
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return cfg, *cfgPath, err
	}
	if *listenHost != "" {
		cfg.Listen.Host = *listenHost
	}
	if *listenPort != 0 {
		cfg.Listen.Port = *listenPort
	}
	if *uiHost != "" {
		cfg.ControlListen.Host = *uiHost
	}
	if *uiPort != 0 {
		cfg.ControlListen.Port = *uiPort
	}
	if *upstream != "" {
		parsed, err := config.ParseUpstream(*upstream)
		if err != nil {
			return cfg, *cfgPath, err
		}
		config.ApplyUpstream(&cfg, parsed)
	}
	return config.Normalize(cfg), *cfgPath, nil
}

func check(args []string) error {
	cfg, cfgPath, err := loadWithFlags("check", args)
	if err != nil {
		return err
	}
	whitelist, err := config.LoadWhitelist(cfg, cfgPath)
	if err != nil {
		return err
	}
	data, _ := json.MarshalIndent(map[string]any{
		"config":           cfg,
		"whitelistEntries": len(whitelist),
	}, "", "  ")
	fmt.Println(string(data))
	return nil
}

func start(args []string) error {
	cfg, cfgPath, err := loadWithFlags("start", args)
	if err != nil {
		return err
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}
	rt := rtpkg.New(cfgPath)
	state, err := rt.StartProxy()
	if err != nil {
		return err
	}
	fmt.Printf("tight-proxy listening on %s\n", state.Address)
	fmt.Println("upstreams configured from tight-proxy.config.json")
	select {}
}

func web(args []string, withTray bool) error {
	cfg, cfgPath, err := loadWithFlags("web", args)
	if err != nil {
		return err
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}
	rt := rtpkg.New(cfgPath)
	root, err := webui.FS()
	if err != nil {
		return err
	}
	controlServer := control.New(rt, cfg.ControlListen, root)
	addr, err := controlServer.Start()
	if err != nil {
		return err
	}
	controlURL := "http://" + cfg.ControlListen.Host + ":" + strconv.Itoa(cfg.ControlListen.Port) + "/"
	fmt.Printf("tight-proxy control panel: %s\n", controlURL)
	fmt.Printf("control server listening on %s\n", addr.String())
	if withTray {
		tray.Run(rt, controlURL)
		return controlServer.Stop()
	}
	select {}
}

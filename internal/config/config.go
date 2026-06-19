package config

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Enabled        bool      `json:"enabled"`
	Listen         Listen    `json:"listen"`
	ControlListen  Listen    `json:"controlListen"`
	WhitelistFile  string    `json:"whitelistFile"`
	Upstreams      Upstreams `json:"upstreams"`
	Relay          Relay     `json:"relay"`
	LegacyUpstream *Upstream `json:"upstream,omitempty"`
}

type Listen struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type Upstream struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Upstreams struct {
	HTTP   ProxyEndpoint `json:"http"`
	HTTPS  ProxyEndpoint `json:"https"`
	FTP    ProxyEndpoint `json:"ftp"`
	SOCKS5 ProxyEndpoint `json:"socks5"`
}

type ProxyEndpoint struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Relay struct {
	Enabled bool        `json:"enabled"`
	Rules   []RelayRule `json:"rules"`
}

type RelayRule struct {
	Enabled bool   `json:"enabled"`
	Entry   Listen `json:"entry"`
	Exit    Listen `json:"exit"`
}

func DefaultPath() string {
	wd, err := os.Getwd()
	if err != nil {
		return "tight-proxy.config.json"
	}
	return filepath.Join(wd, "tight-proxy.config.json")
}

func Default() Config {
	return Config{
		Enabled: true,
		Listen: Listen{
			Host: "127.0.0.1",
			Port: 7890,
		},
		ControlListen: Listen{
			Host: "127.0.0.1",
			Port: 3000,
		},
		WhitelistFile: "whitelist.txt",
		Upstreams: Upstreams{
			HTTP: ProxyEndpoint{
				Enabled: true,
				Host:    "127.0.0.1",
				Port:    8080,
			},
			HTTPS: ProxyEndpoint{
				Host: "127.0.0.1",
				Port: 8080,
			},
			FTP: ProxyEndpoint{
				Host: "127.0.0.1",
				Port: 21,
			},
			SOCKS5: ProxyEndpoint{
				Host: "127.0.0.1",
				Port: 1080,
			},
		},
		Relay: Relay{
			Enabled: false,
			Rules: []RelayRule{
				{
					Enabled: true,
					Entry: Listen{
						Host: "0.0.0.0",
						Port: 34567,
					},
					Exit: Listen{
						Host: "127.0.0.1",
						Port: 45678,
					},
				},
			},
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		path = DefaultPath()
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return Normalize(cfg), nil
}

func Save(path string, cfg Config) error {
	if path == "" {
		path = DefaultPath()
	}
	cfg = Normalize(cfg)
	cfg.LegacyUpstream = nil
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func Normalize(cfg Config) Config {
	def := Default()
	if cfg.Listen.Host == "" {
		cfg.Listen.Host = def.Listen.Host
	}
	if cfg.Listen.Port == 0 {
		cfg.Listen.Port = def.Listen.Port
	}
	if cfg.ControlListen.Host == "" {
		cfg.ControlListen.Host = def.ControlListen.Host
	}
	if cfg.ControlListen.Port == 0 {
		cfg.ControlListen.Port = def.ControlListen.Port
	}
	if cfg.WhitelistFile == "" {
		cfg.WhitelistFile = def.WhitelistFile
	}
	if cfg.LegacyUpstream != nil && !hasAnyEndpoint(cfg.Upstreams) {
		cfg.Upstreams = def.Upstreams
		setEndpoint(&cfg, cfg.LegacyUpstream.Type, ProxyEndpoint{
			Enabled:  true,
			Host:     cfg.LegacyUpstream.Host,
			Port:     cfg.LegacyUpstream.Port,
			Username: cfg.LegacyUpstream.Username,
			Password: cfg.LegacyUpstream.Password,
		})
	}
	cfg.Upstreams.HTTP = normalizeEndpoint(cfg.Upstreams.HTTP, def.Upstreams.HTTP, "http")
	cfg.Upstreams.HTTPS = normalizeEndpoint(cfg.Upstreams.HTTPS, def.Upstreams.HTTPS, "https")
	cfg.Upstreams.FTP = normalizeEndpoint(cfg.Upstreams.FTP, def.Upstreams.FTP, "ftp")
	cfg.Upstreams.SOCKS5 = normalizeEndpoint(cfg.Upstreams.SOCKS5, def.Upstreams.SOCKS5, "socks5")
	cfg.Relay = normalizeRelay(cfg.Relay, def.Relay)
	return cfg
}

func normalizeRelay(relay Relay, fallback Relay) Relay {
	if len(relay.Rules) == 0 {
		relay.Rules = fallback.Rules
	}
	for index := range relay.Rules {
		rule := &relay.Rules[index]
		if rule.Entry.Host == "" {
			rule.Entry.Host = "0.0.0.0"
		}
		if rule.Entry.Port == 0 {
			rule.Entry.Port = 34567
		}
		if rule.Exit.Host == "" {
			rule.Exit.Host = "127.0.0.1"
		}
		if rule.Exit.Port == 0 {
			rule.Exit.Port = 45678
		}
	}
	return relay
}

func hasAnyEndpoint(upstreams Upstreams) bool {
	return upstreams.HTTP.Host != "" || upstreams.HTTPS.Host != "" || upstreams.FTP.Host != "" || upstreams.SOCKS5.Host != ""
}

func normalizeEndpoint(endpoint ProxyEndpoint, fallback ProxyEndpoint, proxyType string) ProxyEndpoint {
	if endpoint.Host == "" {
		endpoint.Host = fallback.Host
	}
	if endpoint.Port == 0 {
		if fallback.Port != 0 {
			endpoint.Port = fallback.Port
		} else {
			endpoint.Port = DefaultPort(proxyType)
		}
	}
	return endpoint
}

func setEndpoint(cfg *Config, proxyType string, endpoint ProxyEndpoint) {
	switch NormalizeProxyType(proxyType) {
	case "https":
		cfg.Upstreams.HTTPS = endpoint
	case "ftp":
		cfg.Upstreams.FTP = endpoint
	case "socks5":
		cfg.Upstreams.SOCKS5 = endpoint
	default:
		cfg.Upstreams.HTTP = endpoint
	}
}

func ApplyUpstream(cfg *Config, upstream Upstream) {
	endpoint := ProxyEndpoint{
		Enabled:  true,
		Host:     upstream.Host,
		Port:     upstream.Port,
		Username: upstream.Username,
		Password: upstream.Password,
	}
	setEndpoint(cfg, upstream.Type, endpoint)
}

func UpstreamFor(cfg Config, kind string) (Upstream, bool) {
	cfg = Normalize(cfg)
	kind = NormalizeProxyType(kind)
	var endpoint ProxyEndpoint
	upstreamType := kind
	switch kind {
	case "https":
		endpoint = cfg.Upstreams.HTTPS
		upstreamType = "http"
	case "ftp":
		endpoint = cfg.Upstreams.FTP
	default:
		endpoint = cfg.Upstreams.HTTP
		kind = "http"
		upstreamType = "http"
	}
	if endpoint.Enabled {
		return endpoint.AsUpstream(upstreamType), true
	}
	if cfg.Upstreams.SOCKS5.Enabled {
		return cfg.Upstreams.SOCKS5.AsUpstream("socks5"), true
	}
	return Upstream{}, false
}

func (endpoint ProxyEndpoint) AsUpstream(proxyType string) Upstream {
	return Upstream{
		Type:     NormalizeProxyType(proxyType),
		Host:     endpoint.Host,
		Port:     endpoint.Port,
		Username: endpoint.Username,
		Password: endpoint.Password,
	}
}

func NormalizeProxyType(proxyType string) string {
	proxyType = strings.ToLower(strings.TrimSpace(proxyType))
	if proxyType == "socks" {
		return "socks5"
	}
	if proxyType == "" {
		return "http"
	}
	return proxyType
}

func DefaultPort(proxyType string) int {
	switch strings.ToLower(proxyType) {
	case "https":
		return 443
	case "ftp":
		return 21
	case "socks", "socks5":
		return 1080
	default:
		return 8080
	}
}

func Resolve(basePath, maybeRelative string) string {
	if maybeRelative == "" || filepath.IsAbs(maybeRelative) {
		return maybeRelative
	}
	if basePath == "" {
		basePath = DefaultPath()
	}
	return filepath.Join(filepath.Dir(basePath), maybeRelative)
}

func LoadWhitelist(cfg Config, cfgPath string) ([]string, error) {
	path := Resolve(cfgPath, cfg.WhitelistFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ParseWhitelist(string(data)), nil
}

func SaveWhitelist(cfg Config, cfgPath string, text string) error {
	path := Resolve(cfgPath, cfg.WhitelistFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func ParseWhitelist(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(strings.ToLower(line))
		line = strings.TrimPrefix(line, "*.")
		line = strings.TrimSuffix(line, ".")
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func WhitelistText(entries []string) string {
	return strings.Join(entries, "\n")
}

func MatchDomain(host string, whitelist []string) bool {
	host = NormalizeHost(host)
	if host == "" {
		return false
	}
	if len(whitelist) == 0 {
		return true
	}
	for _, entry := range whitelist {
		entry = NormalizeHost(entry)
		if host == entry || strings.HasSuffix(host, "."+entry) {
			return true
		}
	}
	return false
}

func NormalizeHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	host = strings.TrimPrefix(host, "[")
	if idx := strings.IndexByte(host, ']'); idx >= 0 {
		host = host[:idx]
	} else if strings.Count(host, ":") == 1 {
		host = strings.Split(host, ":")[0]
	}
	return strings.TrimSuffix(host, ".")
}

func ParseUpstream(raw string) (Upstream, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Upstream{}, err
	}
	port := DefaultPort(u.Scheme)
	if u.Port() != "" {
		parsed, err := strconv.Atoi(u.Port())
		if err != nil {
			return Upstream{}, err
		}
		port = parsed
	}
	password, _ := u.User.Password()
	return Upstream{
		Type:     NormalizeProxyType(u.Scheme),
		Host:     u.Hostname(),
		Port:     port,
		Username: u.User.Username(),
		Password: password,
	}, nil
}

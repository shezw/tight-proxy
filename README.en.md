# tight-proxy

[中文](README.md) | English

`tight-proxy` is a lightweight cross-platform proxy tool written in Go for Windows, macOS, and Linux. It can run as a local HTTP proxy entry, and it can also act as a host relay that forwards inbound TCP connections to configured exit addresses.

## Features

- Local HTTP proxy entry, for example `127.0.0.1:7890`
- Lightweight local web control panel
- System tray mode with a lightning-in-circle icon
- CLI commands: `init`, `check`, `start`, `web`, and `tray`
- Domain whitelist matching, one domain per line
- Upstream proxy support for `HTTP`, `HTTPS`, `FTP`, and `SOCKS5`
- Optional upstream username/password authentication
- Host relay rules, equivalent to `socat TCP-LISTEN:34567,fork,reuseaddr TCP:127.0.0.1:45678`

## UI

Overview:

![Control panel overview](docs/images/control-panel-overview.jpg)

Local proxy entry:

![Local proxy entry](docs/images/local-entry.jpg)

Upstream proxy configuration:

![Upstream proxy configuration](docs/images/upstream-settings.jpg)

Host relay mode supports multiple TCP forwarding rules:

![Relay rules](docs/images/relay-rules.jpg)

Whitelist entries are configured one domain per line:

![Whitelist](docs/images/whitelist.jpg)

## Behavior

The local proxy listens on one local entry port. Point your browser or system proxy setting to that entry. Target ports are not limited by this local entry port; requests can still target `:80`, `:443`, `:3000`, or any other destination port.

Whitelist rules:

- Empty whitelist: all domains use the upstream proxy
- Non-empty whitelist: matching domains use the upstream proxy, non-matching domains connect directly

Domain matching includes exact domains and subdomains. `example.com` matches both `example.com` and `api.example.com`.

Upstream selection:

- HTTP requests use the HTTP row
- HTTPS `CONNECT` requests use the HTTPS row
- FTP URLs use the FTP row
- SOCKS5 can be used as a fallback when the protocol-specific row is disabled
- If neither the protocol-specific row nor SOCKS5 is enabled, the request connects directly

The HTTPS row means "proxy endpoint used for HTTPS traffic". tight-proxy uses standard HTTP `CONNECT` to reach that proxy endpoint.

Relay rules:

- Enable "作为主机 / 开启中转" to expose relay rules
- Each rule has one entry address and one exit address
- `0.0.0.0:34567 -> 127.0.0.1:45678` means other devices can connect to this host on `34567`, and tight-proxy forwards the TCP stream to local `45678`
- Enabled relay rules start and stop together with the proxy

## Build

```bash
go mod tidy
go build -o dist/tight-proxy ./cmd/tight-proxy
go build -o dist/tight-proxy-tray ./cmd/tight-proxy-tray
```

Windows x86_64:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o dist/tight-proxy-windows-amd64.exe ./cmd/tight-proxy
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags='-H windowsgui' -o dist/tight-proxy-tray-windows-amd64.exe ./cmd/tight-proxy-tray
```

For normal Windows usage, use `tight-proxy-tray-windows-amd64.exe`.

## Release

The current version starts at `0.1.13`. The version is stored in [VERSION](VERSION), using `MAJOR.MINOR.PATCH`.

Release flow:

```bash
git tag v0.1.13
git push origin main v0.1.13
```

You can also create a `v0.1.13` release on GitHub. GitHub Actions only builds and uploads release assets when a release is published.

Release targets:

- `macos-arm64`
- `macos-x86_64`
- `windows-arm64`
- `windows-x86_64`
- `linux-arm64`
- `linux-x86_64`

Bump version:

```bash
./scripts/bump-version.sh patch
./scripts/bump-version.sh minor
./scripts/bump-version.sh major
```

## CLI

Create config:

```bash
./dist/tight-proxy init
```

Start only the proxy:

```bash
./dist/tight-proxy start
```

Start the web control panel:

```bash
./dist/tight-proxy web
```

Start tray mode:

```bash
./dist/tight-proxy tray
```

Check config:

```bash
./dist/tight-proxy check
```

Override config at launch:

```bash
./dist/tight-proxy web \
  --listen-host 127.0.0.1 \
  --listen-port 7890 \
  --ui-host 127.0.0.1 \
  --ui-port 3000 \
  --upstream socks5://user:pass@127.0.0.1:1080
```

On Windows and macOS, starting the proxy also enables the current user's system proxy and points it to the local tight-proxy entry. Stopping or quitting restores the previous system proxy settings.

Linux desktop proxy settings are not universal, so Linux currently requires setting the browser or desktop proxy to the local tight-proxy entry manually.

## Example Config

```json
{
  "enabled": true,
  "listen": {
    "host": "127.0.0.1",
    "port": 7890
  },
  "controlListen": {
    "host": "127.0.0.1",
    "port": 3000
  },
  "whitelistFile": "whitelist.txt",
  "upstreams": {
    "http": {
      "enabled": true,
      "host": "127.0.0.1",
      "port": 8080,
      "username": "",
      "password": ""
    },
    "https": {
      "enabled": false,
      "host": "127.0.0.1",
      "port": 8080,
      "username": "",
      "password": ""
    },
    "ftp": {
      "enabled": false,
      "host": "127.0.0.1",
      "port": 21,
      "username": "",
      "password": ""
    },
    "socks5": {
      "enabled": false,
      "host": "127.0.0.1",
      "port": 1080,
      "username": "",
      "password": ""
    }
  },
  "relay": {
    "enabled": false,
    "rules": [
      {
        "enabled": true,
        "entry": {
          "host": "0.0.0.0",
          "port": 34567
        },
        "exit": {
          "host": "127.0.0.1",
          "port": 45678
        }
      }
    ]
  }
}
```

Whitelist example:

```text
example.com
github.com
```

## Notes

`HTTP` and `HTTPS` upstream proxies use standard proxy forwarding and `CONNECT`.

`SOCKS5` supports no-auth and username/password authentication.

`FTP` is accepted as an upstream proxy type for HTTP-style FTP proxy endpoints. FTP does not have one universal proxy tunnel protocol.

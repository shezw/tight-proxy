# tight-proxy

中文 | [English](README.en.md)

`tight-proxy` 是一个用 Go 写的轻量跨平台代理工具，支持 Windows、macOS、Linux。它可以作为本机 HTTP 代理入口，也可以作为主机开启 TCP 中转，把外部连接转发到指定出口地址。

## 功能

- 本地 HTTP 代理入口，例如 `127.0.0.1:7890`
- 本地 Web 控制面板
- 系统托盘模式，使用闪电加圆圈图标
- 命令行模式：`init`、`check`、`start`、`web`、`tray`
- 白名单按域名匹配，一行一个域名
- 上游代理支持 `HTTP`、`HTTPS`、`FTP`、`SOCKS5`
- 上游代理支持可选用户名和密码
- 主机中转规则，等价于 `socat TCP-LISTEN:34567,fork,reuseaddr TCP:127.0.0.1:45678`

## 界面

总览：

![控制面板总览](docs/images/control-panel-overview.jpg)

本地代理入口：

![本地代理入口](docs/images/local-entry.jpg)

上游代理配置：

![上游代理配置](docs/images/upstream-settings.jpg)

作为主机开启中转，支持多条入口到出口的 TCP 转发规则：

![中转规则](docs/images/relay-rules.jpg)

白名单按域名逐行配置：

![白名单](docs/images/whitelist.jpg)

## 行为说明

本地代理只监听一个入口端口。浏览器或系统代理指向这个入口即可，目标网站端口不受这个入口端口限制，访问 `:80`、`:443`、`:3000` 或其他目标端口都可以。

白名单规则：

- 白名单为空：所有域名都走上游代理
- 白名单非空：匹配域名走上游代理，不匹配域名直连

域名匹配包含精确域名和子域名。配置 `example.com` 会匹配 `example.com` 和 `api.example.com`。

上游选择：

- HTTP 请求使用 HTTP 行
- HTTPS `CONNECT` 请求使用 HTTPS 行
- FTP URL 使用 FTP 行
- 协议行未启用时，SOCKS5 可作为 fallback
- 如果协议行和 SOCKS5 都未启用，则直连

HTTPS 行表示“HTTPS 流量使用的代理端点”，程序会使用标准 HTTP `CONNECT` 连接到该代理端点。

中转规则：

- 勾选“作为主机 / 开启中转”后生效
- 每条规则包含一个代理入口和一个代理出口
- `0.0.0.0:34567 -> 127.0.0.1:45678` 表示其他设备可以连本机 `34567`，tight-proxy 会把 TCP 流转发到本机 `45678`
- 所有启用的中转规则随代理一起启动和停止

## 构建

```bash
go mod tidy
go build -o dist/tight-proxy ./cmd/tight-proxy
go build -o dist/tight-proxy-tray ./cmd/tight-proxy-tray
```

Windows x86_64：

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o dist/tight-proxy-windows-amd64.exe ./cmd/tight-proxy
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags='-H windowsgui' -o dist/tight-proxy-tray-windows-amd64.exe ./cmd/tight-proxy-tray
```

建议普通 Windows 用户使用托盘版 `tight-proxy-tray-windows-amd64.exe`。

## 发布

当前版本从 `0.1.13` 开始，版本号保存在 [VERSION](VERSION)，使用 `MAJOR.MINOR.PATCH` 三段式。

发布流程：

```bash
git tag v0.1.13
git push origin main v0.1.13
```

也可以在 GitHub 上创建 `v0.1.13` release。release 发布后，GitHub Actions 才会构建并上传这些产物：

- `macos-arm64`
- `macos-x86_64`
- `windows-arm64`
- `windows-x86_64`
- `linux-arm64`
- `linux-x86_64`

升级版本：

```bash
./scripts/bump-version.sh patch
./scripts/bump-version.sh minor
./scripts/bump-version.sh major
```

## 命令行

创建配置：

```bash
./dist/tight-proxy init
```

只启动代理：

```bash
./dist/tight-proxy start
```

启动 Web 控制面板：

```bash
./dist/tight-proxy web
```

启动系统托盘：

```bash
./dist/tight-proxy tray
```

检查配置：

```bash
./dist/tight-proxy check
```

启动时覆盖配置：

```bash
./dist/tight-proxy web \
  --listen-host 127.0.0.1 \
  --listen-port 7890 \
  --ui-host 127.0.0.1 \
  --ui-port 3000 \
  --upstream socks5://user:pass@127.0.0.1:1080
```

Windows 和 macOS 上，启动代理会自动设置当前用户的系统代理；停止或退出时会恢复之前的系统代理设置。

Linux 桌面环境没有统一代理设置接口，目前需要手动把浏览器或桌面代理指向 tight-proxy 的本地入口。

## 配置示例

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

白名单示例：

```text
example.com
github.com
```

## 备注

`HTTP` 和 `HTTPS` 上游使用标准代理转发和 `CONNECT`。

`SOCKS5` 支持无认证和用户名密码认证。

`FTP` 作为 HTTP 风格 FTP 代理端点支持；FTP 本身没有统一的代理隧道协议。

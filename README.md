# proxmox-license-proxy

[![CI](https://github.com/TheMeinerLP/proxmox-license-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/TheMeinerLP/proxmox-license-proxy/actions/workflows/ci.yml)
[![Go Reference](https://img.shields.io/badge/go-1.26-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A single Go binary that emulates the Proxmox subscription endpoint for **labs and
homelabs**, and doubles as the client tool to point a Proxmox host at it.

> **For private / internal test environments only.** This tool does not contact
> Proxmox' real servers and changes nothing on a host unless you explicitly run the
> client steps there. Removing the subscription nag on a *production* host bypasses a
> commercial license - production deployments need a real Proxmox subscription.

## How it works

Proxmox verifies online keys by POSTing to `shop.proxmox.com/.../verify.php`. The
response is only "signed" with `md5(SHARED_KEY_DATA + check_token)`, where the
constant is public (open-source `proxmox-subscription`). The proxy reproduces a
valid response locally. Point `shop.proxmox.com` at the proxy, trust its
certificate, and Proxmox writes `status = active`.

Hosts that contact the proxy are **auto-registered as pending** and only become
`active` once an admin **approves** them.

## Install

### One-line install (recommended)

Auto-detects your CPU architecture and package format (`.deb`/`.rpm`/`.apk`) and
installs the matching package (including shell completion and the systemd
service) from the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/TheMeinerLP/proxmox-license-proxy/main/install.sh | sh
```

Want just the CLI, with **no service and no config** (e.g. to run `serve` by hand
on a single host)? Use CLI-only mode - it installs the binary to
`/usr/local/bin` plus shell completions:

```sh
curl -fsSL https://raw.githubusercontent.com/TheMeinerLP/proxmox-license-proxy/main/install.sh | PMOX_CLI_ONLY=1 sh
```

Pin a version with `VERSION=0.2.0`, or review the script first:
`curl -fsSL .../install.sh -o install.sh && less install.sh && sh install.sh`.

---

All artifacts are also attached to every [GitHub Release](https://github.com/TheMeinerLP/proxmox-license-proxy/releases/latest)
(`linux`/`darwin`, `amd64`/`arm64`) for manual install - replace `<ver>` with the
release version.

### Debian / Ubuntu / Proxmox (`.deb`)

The package installs a hardened **systemd service**, a dedicated `pmox` user and a
default config at `/etc/pmox/config.yaml`:

```sh
ARCH=amd64   # or arm64
curl -fsSLO https://github.com/TheMeinerLP/proxmox-license-proxy/releases/latest/download/proxmox-license-proxy_<ver>_linux_${ARCH}.deb
sudo apt install ./proxmox-license-proxy_<ver>_linux_${ARCH}.deb
```

The service is **enabled but not started** on install - review the config first,
then start it:

```sh
sudoedit /etc/pmox/config.yaml
sudo systemctl start proxmox-license-proxy
systemctl status proxmox-license-proxy
```

### RHEL / Fedora (`.rpm`), Alpine (`.apk`)

```sh
sudo dnf install ./proxmox-license-proxy_<ver>_linux_amd64.rpm
sudo apk add --allow-untrusted proxmox-license-proxy_<ver>_linux_amd64.apk
```

### Docker (multi-arch, GitHub Container Registry)

```sh
docker run -d --name pmox -p 443:8443 -v pmox:/data \
  ghcr.io/themeinerlp/proxmox-license-proxy:latest
# or use the bundled compose file (builds locally):
docker compose up -d
```

### Plain binary (any Linux/macOS)

The `tar.gz` bundles the binary plus a systemd unit and sample config under
`contrib/`:

```sh
curl -fsSL https://github.com/TheMeinerLP/proxmox-license-proxy/releases/latest/download/proxmox-license-proxy_<ver>_linux_amd64.tar.gz | tar xz
sudo install proxmox-license-proxy /usr/local/bin/
```

### From source (Go 1.26+)

```sh
git clone https://github.com/TheMeinerLP/proxmox-license-proxy && cd proxmox-license-proxy
CGO_ENABLED=0 go build -ldflags "-s -w" -o proxmox-license-proxy .
```

## Quickstart (server, Docker)

```sh
docker compose up -d --build
# mint a lab key (interactive product picker, or pass --product pbs --level c --yes)
docker compose exec proxy proxmox-license-proxy license generate --product pbs --yes
docker compose exec proxy proxmox-license-proxy server pending
docker compose exec proxy proxmox-license-proxy server approve <serverid>
```

## Quickstart (client, on the Proxmox host)

```sh
# install the binary + trust cert + edit /etc/hosts, interactively
proxmox-license-proxy client install
# ... then on the host:
proxmox-backup-manager subscription set pbsc-1ab1234567
proxmox-backup-manager subscription update
```

The server announces itself on the LAN via **mDNS**, so `client install` offers the
discovered servers and lets you **pick which server IP** to use (no need to know the
address up front). List them anytime with `client discover`; disable advertising
with `serve --mdns=false`.

## Configuration

Config precedence: flags > `PMOX_*` env > config file > defaults.

| Key (config.yaml) | Env | Default |
|---|---|---|
| `listen` | `PMOX_LISTEN` | `:443` |
| `log` | `PMOX_LOG` | `info` |
| `registry_file` | `PMOX_REGISTRY_FILE` | `/etc/pmox/registry.json` |
| `tls.mode` | `PMOX_TLS_MODE` | `auto` (auto/files/http) |
| `hosts.file` / `hosts.ip` | `PMOX_HOSTS_FILE` / `PMOX_HOSTS_IP` | `/etc/hosts` / - |

See [`config.example.yaml`](config.example.yaml), or scaffold one with
`proxmox-license-proxy config init` (interactive setup: `setup server`).

## Commands

`serve`, `status`, `license {add,generate,list,show,rm,set-due,export,import}`
- `server {list,pending,approve,reject,block,rm}`, `client {install,uninstall,discover}`
- `cert {generate,install}`, `hosts {enable,disable,status}`
- `config {init,show,path}`, `version`, `completion`

`license generate` mints a **lab-only** key: it is format-valid (so the emulation
works) but deliberately marked - the key carries a visible `1ab` ("lab") signature
(e.g. `pbsc-1ab879865b`) and its product name is tagged
`(LAB, proxmox-license-proxy - NOT FOR PRODUCTION)`, which Proxmox shows in its
subscription panel. The command prints a warning banner and requires confirmation.

**Every** license must carry the `1ab` signature - `license add` (and the REST
API / `import`) reject any key without it. This guarantees the proxy can only
ever manage clearly-marked lab keys, never something mistakable for a real
subscription. The easiest path is `license generate`.

`approve`/`reject`/`block` accept multiple server ids; `approve`/`reject` also
take `--all` (all pending hosts) and `--note`. Read commands (`status`,
`license list/show`, `server list/pending`) support `-o`/`--output table|json|yaml`.

Destructive commands (`license rm`, `server rm`, `hosts disable`) prompt for
confirmation; pass `-y`/`--yes` to skip it. The registry keeps a `.bak` of the
previous state on every write.

## REST API

`POST /modules/servers/licensing/verify.php`, `GET /ca.crt`, `GET /healthz` -
`GET /readyz`, `GET /status`, `/api/licenses`, `/api/servers`

## Development

CI (GitHub Actions) is the source of truth - it runs tests (`-race`),
`golangci-lint` and a build on every push/PR. Locally:

```sh
go test -race ./...
golangci-lint run        # same config as CI
go build ./...
```

Commits follow [Conventional Commits](https://www.conventionalcommits.org).
[Release Please](https://github.com/googleapis/release-please) maintains a
release PR; merging it tags the release, after which GoReleaser publishes the
binaries + `.deb`/`.rpm`/`.apk` packages and a multi-arch image to `ghcr.io`.

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## Security & responsible use

This is for home labs and internal test clusters - not a way to dodge paying for
production. If you run Proxmox in production, buy a subscription. See
[SECURITY.md](SECURITY.md) for the full policy and how to report vulnerabilities.

## License

[MIT](LICENSE)

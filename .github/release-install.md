## Install

**One-liner** — auto-detects CPU architecture and package format (`.deb`/`.rpm`/`.apk`):

```sh
curl -fsSL https://raw.githubusercontent.com/TheMeinerLP/proxmox-license-proxy/main/install.sh | sh
```

**Debian / Ubuntu / Proxmox**

```sh
curl -fsSLO https://github.com/TheMeinerLP/proxmox-license-proxy/releases/download/{{TAG}}/proxmox-license-proxy_{{VERSION}}_linux_amd64.deb
sudo apt install ./proxmox-license-proxy_{{VERSION}}_linux_amd64.deb
```

**RHEL / Fedora (`.rpm`) · Alpine (`.apk`)** — assets attached below for `amd64` and `arm64`.

**Docker**

```sh
docker pull ghcr.io/themeinerlp/proxmox-license-proxy:{{VERSION}}
```

Full instructions → https://github.com/TheMeinerLP/proxmox-license-proxy#install

---
> ⚠️ For private / internal lab use only. Production hosts need a real Proxmox subscription.

#!/bin/sh
set -e

if command -v systemctl >/dev/null 2>&1; then
    systemctl stop proxmox-license-proxy.service || true
    systemctl disable proxmox-license-proxy.service || true
fi

exit 0

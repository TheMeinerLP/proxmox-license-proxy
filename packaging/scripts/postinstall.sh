#!/bin/sh
set -e

# 1) Create the pmox system group + user (idempotent, portable across distros).
if ! getent group pmox >/dev/null 2>&1; then
    if command -v groupadd >/dev/null 2>&1; then
        groupadd --system pmox
    elif command -v addgroup >/dev/null 2>&1; then
        addgroup -S pmox # busybox/alpine
    fi
fi
if ! getent passwd pmox >/dev/null 2>&1; then
    if command -v useradd >/dev/null 2>&1; then
        useradd --system --gid pmox --no-create-home \
            --home-dir /etc/pmox --shell /usr/sbin/nologin pmox
    elif command -v adduser >/dev/null 2>&1; then
        adduser -S -D -H -h /etc/pmox -s /sbin/nologin -G pmox pmox # alpine
    fi
fi

# 2) Migrate a registry from the old /var/lib/pmox location (pre-/etc/pmox
#    layout) so an upgrade keeps existing hosts and the persisted auto cert.
mkdir -p /etc/pmox
if [ -f /var/lib/pmox/registry.json ] && [ ! -f /etc/pmox/registry.json ]; then
    for f in registry.json registry.json.bak registry.json.lock tls-auto.crt tls-auto.key; do
        [ -e "/var/lib/pmox/$f" ] && mv "/var/lib/pmox/$f" "/etc/pmox/$f" 2>/dev/null || true
    done
fi

# 3) Ownership + permissions. /etc/pmox is a setgid, group-pmox directory so a
#    root-run CLI and the pmox service can share the registry: new files inherit
#    group pmox, and the app pins them to 0660 (group-writable). config.yaml
#    stays root-owned but group-readable so the service can read it.
chown root:pmox /etc/pmox
chmod 2770 /etc/pmox
chown root:pmox /etc/pmox/config.yaml 2>/dev/null || true
chmod 0640 /etc/pmox/config.yaml 2>/dev/null || true
for f in registry.json registry.json.bak registry.json.lock; do
    [ -e "/etc/pmox/$f" ] && chown pmox:pmox "/etc/pmox/$f" && chmod 0660 "/etc/pmox/$f"
done

# 4) systemd: reload + enable, but DO NOT start on first install - the admin
#    should review /etc/pmox/config.yaml (and the :443 / DNS implications) first.
if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
    systemctl enable proxmox-license-proxy.service || true
    # On upgrade, restart only if it was already running.
    if systemctl is-active --quiet proxmox-license-proxy.service; then
        systemctl try-restart proxmox-license-proxy.service || true
    fi
fi

exit 0

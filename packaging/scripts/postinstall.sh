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
            --home-dir /var/lib/pmox --shell /usr/sbin/nologin pmox
    elif command -v adduser >/dev/null 2>&1; then
        adduser -S -D -H -h /var/lib/pmox -s /sbin/nologin -G pmox pmox # alpine
    fi
fi

# 2) Ownership + permissions on data and config.
#    /var/lib/pmox is a setgid, group-pmox directory so a root-run CLI and the
#    pmox service can share the registry: new files inherit group pmox, and the
#    app pins them to 0660 (group-writable). The recursive chown/chmod also heals
#    a registry that an earlier root-run CLI left owned by root.
mkdir -p /var/lib/pmox
chown -R pmox:pmox /var/lib/pmox
chmod 2770 /var/lib/pmox
for f in registry.json registry.json.bak registry.json.lock; do
    [ -e "/var/lib/pmox/$f" ] && chmod 0660 "/var/lib/pmox/$f"
done
chown root:pmox /etc/pmox /etc/pmox/config.yaml 2>/dev/null || true
chmod 0750 /etc/pmox 2>/dev/null || true
chmod 0640 /etc/pmox/config.yaml 2>/dev/null || true

# 3) systemd: reload + enable, but DO NOT start on first install - the admin
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

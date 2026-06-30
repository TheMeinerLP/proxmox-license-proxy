# Thin image shared by the e2e proxy and client containers. It embeds a
# prebuilt binary (built by run.sh) plus fake Proxmox tools, so a "Proxmox host"
# can be simulated without a real PVE/PBS install.
FROM debian:stable-slim

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates curl \
	&& rm -rf /var/lib/apt/lists/*

# Prebuilt binary and the fake `pvesubscription` / `proxmox-backup-manager`
# tools (only pve+pbs, so product auto-detection resolves to those two).
COPY bin/proxmox-license-proxy /usr/local/bin/proxmox-license-proxy
COPY fakebin/pvesubscription /usr/local/bin/pvesubscription
COPY fakebin/proxmox-backup-manager /usr/local/bin/proxmox-backup-manager
RUN chmod +x /usr/local/bin/proxmox-license-proxy \
	/usr/local/bin/pvesubscription /usr/local/bin/proxmox-backup-manager \
	&& mkdir -p /etc/pmox /etc/pve /etc/proxmox-backup /var/lib/fake-proxmox

ENV PMOX_REGISTRY_FILE=/etc/pmox/registry.json

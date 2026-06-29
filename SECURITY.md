# Security Policy

## Intended use

`proxmox-license-proxy` is meant for **private, internal and lab environments
only**. It emulates the Proxmox subscription endpoint so home labs and internal
test clusters can silence the "No valid subscription" notice without contacting
Proxmox' servers.

It is **not** a way to avoid paying for production systems. Proxmox VE / Backup
Server / Mail Gateway are excellent products built by a company that needs
revenue to keep building them. If you run Proxmox in production, buy a
subscription: <https://www.proxmox.com/en/proxmox-virtual-environment/pricing>.

The proxy only has an effect on a host where you explicitly redirect
`shop.proxmox.com` to it and trust its certificate. Never do that on a machine
that should use a real subscription, and be especially careful with shared DNS.

## Reporting a vulnerability

If you find a security issue in this tool, please **do not open a public
issue**. Instead, report it privately via GitHub Security Advisories
("Report a vulnerability" on the Security tab) or by email to the maintainer.

Please include a description, reproduction steps and the affected version
(`proxmox-license-proxy version`). You can expect an initial response within a
few days.

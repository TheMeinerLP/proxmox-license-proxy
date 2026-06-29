package main

import "proxmox-license-proxy/internal/cli"

// Build metadata, injected via -ldflags at build time (see GoReleaser / CI).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.Execute(cli.BuildInfo{Version: version, Commit: commit, Date: date})
}

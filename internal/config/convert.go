package config

import (
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
)

// ToSettings is the CONVERTER: raw Config -> validated domain.
//
// This is where parsing (strings -> typed values) AND validation happen. If
// anything fails it returns a descriptive error and NO half-built Settings. It
// is called exactly once, right after viper.Unmarshal.
func (c *Config) ToSettings() (*Settings, error) {
	if c.Listen == "" {
		return nil, fmt.Errorf("config: 'listen' must not be empty")
	}
	if c.RegistryFile == "" {
		return nil, fmt.Errorf("config: 'registry_file' must not be empty")
	}

	level, err := parseLogLevel(c.LogLevel)
	if err != nil {
		return nil, err
	}

	tls, err := c.TLS.toDomain()
	if err != nil {
		return nil, err
	}

	hosts, err := c.Hosts.toDomain()
	if err != nil {
		return nil, err
	}

	autoApprove, err := c.AutoApprove.toDomain()
	if err != nil {
		return nil, err
	}

	return &Settings{
		Listen:       c.Listen,
		LogLevel:     level,
		RegistryFile: c.RegistryFile,
		TLS:          tls,
		Hosts:        hosts,
		Offline: OfflineSettings{
			PrivateKey: c.Offline.PrivateKey,
			PublicKey:  c.Offline.PublicKey,
		},
		AutoApprove: autoApprove,
	}, nil
}

// privateNetworks are the ranges the `auto_approve.private` shorthand trusts:
// RFC1918, IPv6 ULA, loopback and link-local. A homelab host almost always
// reaches the proxy from one of these.
var privateNetworks = []string{
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16",
	"127.0.0.0/8", "::1/128", "fc00::/7", "fe80::/10",
}

func (a AutoApprove) toDomain() (AutoApproveSettings, error) {
	if !a.Enabled {
		return AutoApproveSettings{}, nil
	}

	var cidrs []string
	if a.Private {
		cidrs = append(cidrs, privateNetworks...)
	}
	cidrs = append(cidrs, a.Networks...)
	if len(cidrs) == 0 {
		return AutoApproveSettings{}, fmt.Errorf(
			"config: auto_approve.enabled needs auto_approve.private: true or at least one auto_approve.networks entry")
	}

	nets := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		p, err := netip.ParsePrefix(strings.TrimSpace(c))
		if err != nil {
			return AutoApproveSettings{}, fmt.Errorf("config: auto_approve.networks %q is not a valid CIDR: %w", c, err)
		}
		nets = append(nets, p.Masked())
	}
	return AutoApproveSettings{Enabled: true, Networks: nets}, nil
}

func (t TLS) toDomain() (TLSSettings, error) {
	// Empty mode -> sensible default instead of an error.
	mode := TLSMode(strings.ToLower(strings.TrimSpace(t.Mode)))
	if mode == "" {
		mode = TLSModeAuto
	}

	switch mode {
	case TLSModeAuto, TLSModeHTTP:
		// ok, no files needed
	case TLSModeFiles:
		if t.Cert == "" || t.Key == "" {
			return TLSSettings{}, fmt.Errorf("config: tls.mode=files requires tls.cert and tls.key")
		}
	default:
		return TLSSettings{}, fmt.Errorf("config: invalid tls.mode %q (allowed: auto, files, http)", t.Mode)
	}

	names := t.Names
	if len(names) == 0 {
		names = []string{"shop.proxmox.com"}
	}

	return TLSSettings{
		Mode:  mode,
		Cert:  t.Cert,
		Key:   t.Key,
		Names: names,
	}, nil
}

func (h Hosts) toDomain() (HostsSettings, error) {
	file := h.File
	if file == "" {
		file = "/etc/hosts"
	}

	// IP is optional. If set, it must be a valid address.
	var ip netip.Addr
	if h.IP != "" {
		parsed, err := netip.ParseAddr(h.IP)
		if err != nil {
			return HostsSettings{}, fmt.Errorf("config: hosts.ip %q is not a valid IP: %w", h.IP, err)
		}
		ip = parsed
	}

	names := h.Names
	if len(names) == 0 {
		names = []string{"shop.proxmox.com"}
	}

	return HostsSettings{
		File:  file,
		IP:    ip,
		Names: names,
	}, nil
}

// parseLogLevel maps the usual log-level strings onto slog.Level.
func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("config: unknown log level %q (allowed: debug, info, warn, error)", raw)
	}
}

package config

import (
	"log/slog"
	"net/netip"
	"path/filepath"
)

// This file holds the DOMAIN model of the application configuration.
//
// Separation of concerns:
//   - config.go (Config)   = "wire format": exactly what is in the YAML/ENV,
//                            loose strings, mapstructure tags. Only for reading.
//   - model.go  (Settings) = "domain": typed, validated, decoupled from Viper.
//                            The rest of the app works with THIS (http, hosts, ...).
//   - convert.go           = the converter Config -> Settings (with validation).
//
// Benefit: the business logic knows neither Viper nor raw strings, is easy to
// test (build Settings directly in a test) and invalid values surface once at
// conversion time, not somewhere deep in the code.

// TLSMode is a real enum type rather than a free-form string. Invalid values
// cannot exist in the domain model at all - the converter rejects them.
type TLSMode string

const (
	TLSModeAuto  TLSMode = "auto"  // self-signed certificate held in memory
	TLSModeFiles TLSMode = "files" // certificate/key loaded from files
	TLSModeHTTP  TLSMode = "http"  // plaintext HTTP (behind a TLS proxy)
)

// Settings is the validated, typed application configuration.
type Settings struct {
	Listen       string
	LogLevel     slog.Level
	RegistryFile string

	TLS         TLSSettings
	Hosts       HostsSettings
	Offline     OfflineSettings
	AutoApprove AutoApproveSettings
	API         APISettings
}

// APISettings holds the validated REST API configuration.
type APISettings struct {
	AdminToken string
}

// AutoCertPaths returns where the `auto` TLS mode persists its self-signed
// certificate and key: alongside the registry file, so they share the same
// state directory and survive restarts/upgrades. Centralised here so the server
// (setupTLS) and `doctor` always agree on the location.
func (s *Settings) AutoCertPaths() (certPath, keyPath string) {
	dir := filepath.Dir(s.RegistryFile)
	return filepath.Join(dir, "tls-auto.crt"), filepath.Join(dir, "tls-auto.key")
}

// AutoApproveSettings decides whether a host contacting from a given address is
// auto-approved on first contact. Networks holds the trusted CIDRs (already
// expanded from the `private` shorthand) as masked prefixes.
type AutoApproveSettings struct {
	Enabled  bool
	Networks []netip.Prefix
}

// Allows reports whether addr is trusted for auto-approval: false unless
// auto-approval is enabled and addr falls inside one of the trusted networks.
func (a AutoApproveSettings) Allows(addr netip.Addr) bool {
	if !a.Enabled || !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	for _, n := range a.Networks {
		if n.Contains(addr) {
			return true
		}
	}
	return false
}

type TLSSettings struct {
	Mode  TLSMode
	Cert  string
	Key   string
	Names []string // SANs for the auto certificate
}

type HostsSettings struct {
	// IP is typed: a netip.Addr can only be a valid address.
	// An "empty" (invalid) Addr means: not set.
	File  string
	IP    netip.Addr
	Names []string
}

type OfflineSettings struct {
	PrivateKey string
	PublicKey  string
}

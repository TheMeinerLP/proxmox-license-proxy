package config

import (
	"log/slog"
	"net/netip"
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
// cannot exist in the domain model at all – the converter rejects them.
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

	TLS     TLSSettings
	Hosts   HostsSettings
	Offline OfflineSettings
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

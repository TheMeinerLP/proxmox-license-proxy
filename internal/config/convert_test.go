package config

import (
	"log/slog"
	"testing"
)

func validConfig() Config {
	return Config{Listen: ":443", RegistryFile: "/etc/pmox/registry.json"}
}

func TestToSettings_DefaultsApplied(t *testing.T) {
	c := validConfig()
	s, err := c.ToSettings()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.TLS.Mode != TLSModeAuto {
		t.Errorf("default TLS mode = %q, want auto", s.TLS.Mode)
	}
	if s.Hosts.File != "/etc/hosts" {
		t.Errorf("default hosts.file = %q", s.Hosts.File)
	}
	if len(s.TLS.Names) != 1 || s.TLS.Names[0] != "shop.proxmox.com" {
		t.Errorf("default tls.names = %v", s.TLS.Names)
	}
	if s.LogLevel != slog.LevelInfo {
		t.Errorf("default log level = %v", s.LogLevel)
	}
}

func TestToSettings_Errors(t *testing.T) {
	cases := map[string]func(c *Config){
		"empty listen":       func(c *Config) { c.Listen = "" },
		"empty registry":     func(c *Config) { c.RegistryFile = "" },
		"files without cert": func(c *Config) { c.TLS.Mode = "files" },
		"invalid tls mode":   func(c *Config) { c.TLS.Mode = "bogus" },
		"invalid hosts ip":   func(c *Config) { c.Hosts.IP = "not-an-ip" },
		"unknown log level":  func(c *Config) { c.LogLevel = "loud" },
	}
	for name, mutate := range cases {
		c := validConfig()
		mutate(&c)
		if _, err := c.ToSettings(); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestToSettings_FilesModeValid(t *testing.T) {
	c := validConfig()
	c.TLS = TLS{Mode: "files", Cert: "/c.pem", Key: "/k.pem"}
	s, err := c.ToSettings()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.TLS.Mode != TLSModeFiles || s.TLS.Cert != "/c.pem" {
		t.Errorf("files mode not carried through: %+v", s.TLS)
	}
}

func TestToSettings_LogLevels(t *testing.T) {
	for in, want := range map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	} {
		c := validConfig()
		c.LogLevel = in
		s, err := c.ToSettings()
		if err != nil || s.LogLevel != want {
			t.Errorf("log %q -> (%v,%v), want %v", in, s.LogLevel, err, want)
		}
	}
}

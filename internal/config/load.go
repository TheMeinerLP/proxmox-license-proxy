package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Load merges defaults, an optional config file and PMOX_* environment
// variables, then converts the raw wire-config into validated domain Settings.
// cfgFile overrides the default search locations (./config.yaml,
// /etc/pmox/config.yaml) when non-empty. The second return value is the config
// file that was actually read (empty when none was found).
//
// Precedence: flags (applied by the caller) > PMOX_* env > config file > defaults.
func Load(cfgFile string) (*Settings, string, error) {
	v := viper.New()

	// Defaults (lowest precedence) - the tool works without any config file.
	v.SetDefault("listen", ":443")
	v.SetDefault("log", "info")
	// Keep config, registry and the auto cert together under /etc/pmox so the
	// CLI and the service agree on one folder even when no config file is found.
	// The packaged service makes /etc/pmox writable for this (ReadWritePaths).
	v.SetDefault("registry_file", "/etc/pmox/registry.json")
	v.SetDefault("tls.mode", "auto")
	v.SetDefault("hosts.file", "/etc/hosts")
	// Register the auto-approve keys so PMOX_AUTO_APPROVE_* env binding works
	// (viper's AutomaticEnv only resolves nested keys it already knows).
	v.SetDefault("auto_approve.enabled", false)
	v.SetDefault("auto_approve.private", false)
	// Same reason: register api.admin_token so PMOX_API_ADMIN_TOKEN binds.
	v.SetDefault("api.admin_token", "")

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/pmox")
	}

	// Environment: PMOX_LISTEN, PMOX_TLS_MODE, PMOX_REGISTRY_FILE, ...
	v.SetEnvPrefix("PMOX")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// A missing config file is fine (defaults + env still apply).
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, "", fmt.Errorf("read config: %w", err)
		}
	}

	var raw Config
	if err := v.Unmarshal(&raw); err != nil {
		return nil, "", fmt.Errorf("parse config: %w", err)
	}

	s, err := raw.ToSettings()
	if err != nil {
		return nil, "", err
	}
	return s, v.ConfigFileUsed(), nil
}

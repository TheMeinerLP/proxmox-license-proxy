package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShadowedConfigWarning(t *testing.T) {
	dir := t.TempDir()
	sysCfg := filepath.Join(dir, "system.yaml")
	localCfg := filepath.Join(dir, "config.yaml")
	for _, p := range []string{sysCfg, localCfg} {
		if err := os.WriteFile(p, []byte("log: info\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Save and restore the package globals the function reads.
	origSys, origUsed, origFlag := systemConfigPath, cfgUsed, cfgFile
	t.Cleanup(func() { systemConfigPath, cfgUsed, cfgFile = origSys, origUsed, origFlag })
	systemConfigPath = sysCfg

	t.Run("warns when a local config shadows the existing system one", func(t *testing.T) {
		cfgFile, cfgUsed = "", localCfg
		if w := shadowedConfigWarning(); w == "" {
			t.Error("expected a warning when a local config shadows the system one")
		}
	})

	t.Run("silent with explicit -c", func(t *testing.T) {
		cfgFile, cfgUsed = localCfg, localCfg
		if w := shadowedConfigWarning(); w != "" {
			t.Errorf("explicit -c should not warn, got %q", w)
		}
	})

	t.Run("silent when the system config is the one in use", func(t *testing.T) {
		cfgFile, cfgUsed = "", sysCfg
		if w := shadowedConfigWarning(); w != "" {
			t.Errorf("using the system config should not warn, got %q", w)
		}
	})

	t.Run("silent when no system config exists", func(t *testing.T) {
		cfgFile, cfgUsed = "", localCfg
		systemConfigPath = filepath.Join(dir, "absent.yaml")
		defer func() { systemConfigPath = sysCfg }()
		if w := shadowedConfigWarning(); w != "" {
			t.Errorf("no system config means nothing to shadow, got %q", w)
		}
	})
}

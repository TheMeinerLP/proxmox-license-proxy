package registry

import (
	"os"
	"path/filepath"

	"proxmox-license-proxy/internal/fileio"
)

// LegacyRegistryPath is where pre-2.0 installs kept the registry, before config,
// registry and the auto cert were consolidated under /etc/pmox.
const LegacyRegistryPath = "/var/lib/pmox/registry.json"

// MigrateLegacy copies a pre-2.0 registry (plus its backup and the persisted
// auto TLS cert) from the old location into newRegistry's directory, but only
// when newRegistry does not yet exist and the legacy file does - so it never
// overwrites current data and is a no-op on a fresh or already-migrated install.
// It returns the migrated file names. The Debian/RPM/APK packages do the same in
// their postinstall; this covers manual/binary installs and Docker setups that
// kept the old path. File permissions are preserved (the private key stays 0600).
func MigrateLegacy(legacyRegistry, newRegistry string) ([]string, error) {
	if legacyRegistry == newRegistry {
		return nil, nil
	}
	if _, err := os.Stat(newRegistry); err == nil {
		return nil, nil // new location already populated
	}
	if _, err := os.Stat(legacyRegistry); err != nil {
		return nil, nil // nothing to migrate
	}

	oldDir := filepath.Dir(legacyRegistry)
	newDir := filepath.Dir(newRegistry)
	if err := os.MkdirAll(newDir, 0o750); err != nil {
		return nil, err
	}

	pairs := [][2]string{
		{legacyRegistry, newRegistry},
		{legacyRegistry + ".bak", newRegistry + ".bak"},
		{filepath.Join(oldDir, "tls-auto.crt"), filepath.Join(newDir, "tls-auto.crt")},
		{filepath.Join(oldDir, "tls-auto.key"), filepath.Join(newDir, "tls-auto.key")},
	}
	var migrated []string
	for _, p := range pairs {
		src, dst := p[0], p[1]
		fi, err := os.Stat(src)
		if err != nil {
			continue // optional file absent
		}
		data, err := fileio.ReadFile(src)
		if err != nil {
			return migrated, err
		}
		if err := os.WriteFile(dst, data, fi.Mode().Perm()); err != nil {
			return migrated, err
		}
		migrated = append(migrated, filepath.Base(dst))
	}
	return migrated, nil
}

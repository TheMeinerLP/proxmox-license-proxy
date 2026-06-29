package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacy(t *testing.T) {
	old := filepath.Join(t.TempDir(), "old", "registry.json")
	newp := filepath.Join(t.TempDir(), "new", "registry.json")
	if err := os.MkdirAll(filepath.Dir(old), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte(`{"licenses":[]}`), 0o660); err != nil {
		t.Fatal(err)
	}
	// A persisted key should migrate with its 0600 mode intact.
	keyPath := filepath.Join(filepath.Dir(old), "tls-auto.key")
	if err := os.WriteFile(keyPath, []byte("KEY"), 0o600); err != nil {
		t.Fatal(err)
	}

	migrated, err := MigrateLegacy(old, newp)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrated) != 2 {
		t.Fatalf("want registry.json + tls-auto.key migrated, got %v", migrated)
	}
	if data, _ := os.ReadFile(newp); string(data) != `{"licenses":[]}` {
		t.Errorf("registry not copied: %q", data)
	}
	if fi, err := os.Stat(filepath.Join(filepath.Dir(newp), "tls-auto.key")); err != nil {
		t.Fatal(err)
	} else if fi.Mode().Perm() != 0o600 {
		t.Errorf("key mode not preserved: %v", fi.Mode().Perm())
	}
}

func TestMigrateLegacyNoOpWhenNewExists(t *testing.T) {
	old := filepath.Join(t.TempDir(), "registry.json")
	newp := filepath.Join(t.TempDir(), "registry.json")
	_ = os.WriteFile(old, []byte("OLD"), 0o660)
	_ = os.WriteFile(newp, []byte("NEW"), 0o660)

	migrated, err := MigrateLegacy(old, newp)
	if err != nil || len(migrated) != 0 {
		t.Fatalf("should not migrate over existing data: %v %v", migrated, err)
	}
	if data, _ := os.ReadFile(newp); string(data) != "NEW" {
		t.Errorf("existing registry was overwritten: %q", data)
	}
}

func TestMigrateLegacyNoLegacy(t *testing.T) {
	newp := filepath.Join(t.TempDir(), "registry.json")
	migrated, err := MigrateLegacy(filepath.Join(t.TempDir(), "absent.json"), newp)
	if err != nil || migrated != nil {
		t.Fatalf("no-op expected with no legacy file: %v %v", migrated, err)
	}
}

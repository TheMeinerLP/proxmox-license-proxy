package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckRegistryWritable(t *testing.T) {
	dir := t.TempDir()
	if err := checkRegistryWritable(filepath.Join(dir, "registry.json")); err != nil {
		t.Fatalf("writable dir reported an error: %v", err)
	}

	// A directory that does not exist yet but whose parent is writable is fine:
	// checkRegistryWritable creates it.
	nested := filepath.Join(dir, "sub", "registry.json")
	if err := checkRegistryWritable(nested); err != nil {
		t.Fatalf("creatable nested dir reported an error: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(nested)); err != nil {
		t.Errorf("nested dir was not created: %v", err)
	}
}

func TestCheckRegistryWritableReadOnly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	dir := t.TempDir()
	ro := filepath.Join(dir, "ro")
	if err := os.Mkdir(ro, 0o500); err != nil {
		t.Fatal(err)
	}
	err := checkRegistryWritable(filepath.Join(ro, "registry.json"))
	if err == nil {
		t.Fatal("expected an error for a read-only registry directory")
	}
}

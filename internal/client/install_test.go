package client

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInstallBinaryActions(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dest := filepath.Join(dir, "sub", "dest") // sub dir must be created

	writeFile(t, src, "binary-A")

	res, err := InstallBinary(src, dest)
	if err != nil || res.Action != ActionInstalled {
		t.Fatalf("first install = (%+v,%v), want installed", res, err)
	}
	info, _ := os.Stat(dest)
	if info.Mode().Perm() != 0o755 {
		t.Errorf("dest mode = %v, want 0755", info.Mode().Perm())
	}
	if b, _ := os.ReadFile(dest); string(b) != "binary-A" {
		t.Errorf("dest content = %q", b)
	}

	// same bytes -> unchanged
	if res, _ := InstallBinary(src, dest); res.Action != ActionUnchanged {
		t.Errorf("second install = %s, want unchanged", res.Action)
	}

	// different bytes -> updated
	writeFile(t, src, "binary-B")
	if res, _ := InstallBinary(src, dest); res.Action != ActionUpdated {
		t.Errorf("third install = %s, want updated", res.Action)
	}
}

func TestUninstallBinary(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")
	writeFile(t, dest, "x")

	if existed, err := UninstallBinary(dest); err != nil || !existed {
		t.Fatalf("uninstall = (%v,%v), want (true,nil)", existed, err)
	}
	if existed, err := UninstallBinary(dest); err != nil || existed {
		t.Fatalf("second uninstall = (%v,%v), want (false,nil)", existed, err)
	}
}

func TestCurrentBinary(t *testing.T) {
	p, err := CurrentBinary()
	if err != nil || p == "" {
		t.Fatalf("CurrentBinary = (%q,%v)", p, err)
	}
}

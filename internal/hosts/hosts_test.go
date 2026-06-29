package hosts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempHosts(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEnableDryRunDoesNotWrite(t *testing.T) {
	p := tempHosts(t, "127.0.0.1 localhost\n")
	out, err := Enable(p, "10.0.0.5", []string{"shop.proxmox.com"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "10.0.0.5 shop.proxmox.com") || !strings.Contains(out, "127.0.0.1 localhost") {
		t.Fatalf("dry-run output wrong:\n%s", out)
	}
	// file must be unchanged
	raw, _ := os.ReadFile(p)
	if strings.Contains(string(raw), "shop.proxmox.com") {
		t.Fatal("dry-run wrote to the file")
	}
}

func TestEnableDisableRoundtrip(t *testing.T) {
	p := tempHosts(t, "127.0.0.1 localhost\n")

	if _, err := Enable(p, "10.0.0.5", []string{"shop.proxmox.com"}, false); err != nil {
		t.Fatal(err)
	}
	present, block, _ := Status(p)
	if !present || !strings.Contains(block, "10.0.0.5 shop.proxmox.com") {
		t.Fatalf("after enable: present=%v block=%q", present, block)
	}

	// idempotent: enabling again must not duplicate the block
	if _, err := Enable(p, "10.0.0.5", []string{"shop.proxmox.com"}, false); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(p)
	if n := strings.Count(string(raw), "shop.proxmox.com"); n != 1 {
		t.Fatalf("expected exactly one entry, got %d:\n%s", n, raw)
	}

	if _, err := Disable(p, false); err != nil {
		t.Fatal(err)
	}
	present, _, _ = Status(p)
	if present {
		t.Fatal("entry still present after disable")
	}
	raw, _ = os.ReadFile(p)
	if !strings.Contains(string(raw), "127.0.0.1 localhost") {
		t.Fatal("disable removed unrelated lines")
	}
}

func TestEnableValidation(t *testing.T) {
	p := tempHosts(t, "")
	if _, err := Enable(p, "", []string{"shop.proxmox.com"}, true); err == nil {
		t.Error("expected error for empty IP")
	}
	if _, err := Enable(p, "10.0.0.5", nil, true); err == nil {
		t.Error("expected error for empty names")
	}
}

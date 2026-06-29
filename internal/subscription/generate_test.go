package subscription

import (
	"strings"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	for _, p := range []string{"pve", "pbs", "pmg"} {
		key, err := GenerateKey(p, "c")
		if err != nil {
			t.Fatalf("GenerateKey(%q): %v", p, err)
		}
		if !ValidKey(key) {
			t.Errorf("generated key %q is not a valid Proxmox key", key)
		}
		if !IsLabKey(key) {
			t.Errorf("generated key %q not recognised as a lab key", key)
		}
		if !strings.Contains(key, "-"+LabSignature) {
			t.Errorf("generated key %q is missing the lab signature", key)
		}
	}

	// Default level is community.
	if _, err := GenerateKey("pve", ""); err != nil {
		t.Errorf("default level should work: %v", err)
	}

	// Invalid inputs are rejected.
	if _, err := GenerateKey("xxx", "c"); err == nil {
		t.Error("expected error for unknown product")
	}
	if _, err := GenerateKey("pve", "z"); err == nil {
		t.Error("expected error for unknown level")
	}

	// Two generated keys must differ (random suffix).
	a, _ := GenerateKey("pve", "c")
	b, _ := GenerateKey("pve", "c")
	if a == b {
		t.Errorf("generated keys should be random, got %q twice", a)
	}
}

func TestIsLabKey(t *testing.T) {
	if IsLabKey("pbsc-1234567890") {
		t.Error("a normal key must not be reported as a lab key")
	}
	if !IsLabKey("pvec-1ab0000000") {
		t.Error("a 1ab-signed key must be reported as a lab key")
	}
}

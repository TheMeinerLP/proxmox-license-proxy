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

	// PVE keys must carry a socket-count digit (pve[1248][cbsp]-...), otherwise
	// the Proxmox API rejects them with a regex error. PBS/PMG must not.
	pve, _ := GenerateKey("pve", "s")
	if !strings.HasPrefix(pve, "pve1s-") {
		t.Errorf("PVE key %q must start with socket digit + level (pve1s-)", pve)
	}
	if ValidKey("pvec-1ab0000000") {
		t.Error("a PVE key without a socket digit must be invalid")
	}
	pbs, _ := GenerateKey("pbs", "s")
	if !strings.HasPrefix(pbs, "pbss-") {
		t.Errorf("PBS key %q must not carry a socket digit (pbss-)", pbs)
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
	if !IsLabKey("pve1c-1ab0000000") {
		t.Error("a 1ab-signed key must be reported as a lab key")
	}
}

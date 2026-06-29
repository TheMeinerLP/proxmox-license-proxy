package subscription

import (
	"strings"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	for _, p := range []string{"pve", "pbs", "pmg"} {
		key, err := GenerateKey(p, "c", "")
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
	pve, _ := GenerateKey("pve", "s", "")
	if !strings.HasPrefix(pve, "pve1s-") {
		t.Errorf("PVE key %q must default to 1 socket (pve1s-)", pve)
	}
	if ValidKey("pvec-1ab0000000") {
		t.Error("a PVE key without a socket digit must be invalid")
	}
	pbs, _ := GenerateKey("pbs", "s", "")
	if !strings.HasPrefix(pbs, "pbss-") {
		t.Errorf("PBS key %q must not carry a socket digit (pbss-)", pbs)
	}

	// An explicit PVE socket count is encoded and must be valid.
	pve2, _ := GenerateKey("pve", "p", "2")
	if !strings.HasPrefix(pve2, "pve2p-") || !ValidKey(pve2) {
		t.Errorf("PVE 2-socket key %q wrong", pve2)
	}
	if _, err := GenerateKey("pve", "c", "3"); err == nil {
		t.Error("socket count 3 must be rejected (only 1/2/4/8)")
	}
	// sockets is ignored for PBS/PMG.
	if k, _ := GenerateKey("pbs", "c", "8"); !strings.HasPrefix(k, "pbsc-") {
		t.Errorf("PBS must ignore sockets, got %q", k)
	}

	// Default level is community.
	if _, err := GenerateKey("pve", "", ""); err != nil {
		t.Errorf("default level should work: %v", err)
	}

	// Invalid inputs are rejected.
	if _, err := GenerateKey("xxx", "c", ""); err == nil {
		t.Error("expected error for unknown product")
	}
	if _, err := GenerateKey("pve", "z", ""); err == nil {
		t.Error("expected error for unknown level")
	}

	// Two generated keys must differ (random suffix).
	a, _ := GenerateKey("pve", "c", "")
	b, _ := GenerateKey("pve", "c", "")
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

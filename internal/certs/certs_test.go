package certs

import (
	"regexp"
	"testing"
	"time"
)

func TestFingerprint(t *testing.T) {
	certPEM, _, err := GenerateSelfSigned([]string{"shop.proxmox.com"}, time.Hour)
	if err != nil {
		t.Fatalf("GenerateSelfSigned: %v", err)
	}

	fp := Fingerprint(certPEM)
	// 32 bytes -> 32 hex pairs joined by colons, upper-case hex.
	if !regexp.MustCompile(`^([0-9A-F]{2}:){31}[0-9A-F]{2}$`).MatchString(fp) {
		t.Errorf("fingerprint %q is not a colon-separated SHA-256", fp)
	}

	// Deterministic for the same input, and empty for junk.
	if Fingerprint(certPEM) != fp {
		t.Error("fingerprint is not stable for the same certificate")
	}
	if Fingerprint([]byte("not a pem")) != "" {
		t.Error("non-PEM input should yield an empty fingerprint")
	}
}

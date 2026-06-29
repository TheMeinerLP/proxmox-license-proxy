package app

import (
	"path/filepath"
	"strings"
	"testing"

	"proxmox-license-proxy/internal/registry"
	"proxmox-license-proxy/internal/subscription"
)

func newService(t *testing.T) *Service {
	t.Helper()
	return New(registry.NewStore(filepath.Join(t.TempDir(), "registry.json")))
}

func TestAddLicenseDefaults(t *testing.T) {
	lic, err := newService(t).AddLicense(AddLicenseInput{Key: "pbsc-1ab2345678"})
	if err != nil {
		t.Fatalf("AddLicense: %v", err)
	}
	if lic.Status != subscription.Approved {
		t.Errorf("default status = %q, want APPROVED", lic.Status)
	}
	if lic.Product != "pbs" || lic.ProductName == "" {
		t.Errorf("product not derived: %+v", lic)
	}
	if lic.RegDate == "" || lic.NextDueDate == "" {
		t.Errorf("dates not defaulted: %+v", lic)
	}
}

func TestAddLicenseValidation(t *testing.T) {
	s := newService(t)
	if _, err := s.AddLicense(AddLicenseInput{Key: ""}); err == nil {
		t.Error("expected error for empty key")
	}
	if _, err := s.AddLicense(AddLicenseInput{Key: "bogus"}); err == nil {
		t.Error("expected error for invalid key")
	}
	// Every license must carry the lab signature - even a non-lab Proxmox key is
	// rejected, and --force does not bypass it.
	if _, err := s.AddLicense(AddLicenseInput{Key: "pbsc-1234567890"}); err == nil {
		t.Error("a non-lab key must be rejected")
	}
	if _, err := s.AddLicense(AddLicenseInput{Key: "pbsc-1234567890", Force: true}); err == nil {
		t.Error("a non-lab key must be rejected even with --force")
	}
	// --force skips the strict Proxmox format check but still requires a lab key.
	if _, err := s.AddLicense(AddLicenseInput{Key: "custom-1abc0ffee", Force: true}); err != nil {
		t.Errorf("--force should accept a non-standard lab key: %v", err)
	}
	if _, err := s.AddLicense(AddLicenseInput{Key: "pbsc-1ab2345678", StatusRaw: "nope"}); err == nil {
		t.Error("expected error for invalid status")
	}
	if _, err := s.AddLicense(AddLicenseInput{Key: "pbsc-1ab2345678", NextDueDate: "31-12-2030"}); err == nil {
		t.Error("expected error for malformed due date")
	}
}

func TestGenerateLicense(t *testing.T) {
	s := newService(t)
	lic, err := s.GenerateLicense("pbs", "c", true)
	if err != nil {
		t.Fatalf("GenerateLicense: %v", err)
	}
	if !subscription.IsLabKey(lic.Key) {
		t.Errorf("generated key %q is not lab-marked", lic.Key)
	}
	if !strings.Contains(lic.ProductName, "proxmox-license-proxy") {
		t.Errorf("product name missing origin marker: %q", lic.ProductName)
	}
	if lic.Status != subscription.Approved {
		t.Errorf("generated license should be approved, got %q", lic.Status)
	}
	// store=true must persist it.
	got, ok, err := s.Store().GetLicense(lic.Key)
	if err != nil || !ok || got.Key != lic.Key {
		t.Errorf("generated license was not stored: ok=%v err=%v", ok, err)
	}

	// store=false must not persist.
	lic2, err := s.GenerateLicense("pve", "c", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Store().GetLicense(lic2.Key); ok {
		t.Error("store=false should not persist the license")
	}
}

func TestVerifyPendingThenApproved(t *testing.T) {
	s := newService(t)

	// First contact: host is auto-registered as pending -> not active.
	res := s.Verify("HW-1", "pbsc-1234567890", "tok")
	if res.Active || res.Response.Status != "invalid" {
		t.Fatalf("first contact should be invalid: %+v", res)
	}
	if res.HostStatus != subscription.Pending {
		t.Errorf("host should be pending, got %q", res.HostStatus)
	}

	// Approve, then re-verify -> active with the right md5 challenge.
	if _, err := s.Store().SetServerStatus("HW-1", subscription.Approved); err != nil {
		t.Fatal(err)
	}
	res = s.Verify("HW-1", "pbsc-1234567890", "tok")
	if !res.Active || res.Response.Status != "active" {
		t.Fatalf("approved host should be active: %+v", res)
	}
	if res.Response.ServerID != "HW-1" {
		t.Errorf("validdirectory must echo serverid, got %q", res.Response.ServerID)
	}
}

func TestVerifyNoServerID(t *testing.T) {
	res := newService(t).Verify("", "pbsc-1234567890", "tok")
	if res.Active {
		t.Error("a request without serverid must never be active")
	}
}

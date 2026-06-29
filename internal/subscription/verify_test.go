package subscription

import (
	"strings"
	"testing"
)

func TestSharedKeyDataConstant(t *testing.T) {
	// Pin the constant: changing it silently breaks every Proxmox host.
	if SharedKeyData != "kjfdlskfhiuewhfk947368" {
		t.Fatalf("SharedKeyData changed: %q", SharedKeyData)
	}
}

func TestChallengeHash_GoldenVector(t *testing.T) {
	// Known-good vector verified against a real Proxmox check.
	got := ChallengeHash("1768000000deadbeefcafebabe001122")
	want := "7e9b2da3c2f89fac8eeaa47b07438459"
	if got != want {
		t.Fatalf("ChallengeHash = %s, want %s", got, want)
	}
}

func TestValidKey(t *testing.T) {
	cases := map[string]bool{
		"pbsc-1234567890":  true,
		"pve1p-abcdef0123": true, // PVE: socket digit + level
		"pve8c-0011223344": true, // PVE: 8 sockets
		"pmgs-0011223344":  true,
		"pbsb-00112233ff":  true,
		"pvep-abcdef0123":  false, // PVE without socket digit
		"pve3c-1234567890": false, // invalid socket count (only 1/2/4/8)
		"pbs1c-1234567890": false, // PBS must NOT carry a socket digit
		"pbsc-123456789":   false, // 9 hex
		"pbsc-12345678901": false, // 11 hex
		"pbsx-1234567890":  false, // invalid level
		"pxxc-1234567890":  false, // invalid product
		"PBSC-1234567890":  false, // uppercase product
		"pbsc-123456789g":  false, // non-hex
		"pbsc-1234567890 ": false, // trailing space
		"":                 false,
	}
	for key, want := range cases {
		if got := ValidKey(key); got != want {
			t.Errorf("ValidKey(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestDescribe(t *testing.T) {
	cases := []struct {
		key, product, name, level string
	}{
		{"pbsc-1234567890", "pbs", "Proxmox Backup Server Community Subscription", "Community"},
		{"pve1p-abcdef0123", "pve", "Proxmox VE Premium Subscription", "Premium"},
		{"pmgs-0011223344", "pmg", "Proxmox Mail Gateway Standard Subscription", "Standard"},
		{"abc", "", "Proxmox Subscription", ""},
	}
	for _, c := range cases {
		p, n, l := Describe(c.key)
		if p != c.product || n != c.name || l != c.level {
			t.Errorf("Describe(%q) = (%q,%q,%q), want (%q,%q,%q)", c.key, p, n, l, c.product, c.name, c.level)
		}
	}
}

func TestRenderXML_Active(t *testing.T) {
	r := Response{
		Status:      "active",
		ServerID:    "HW-1",
		ProductName: "PBS",
		RegDate:     "2026-01-01",
		NextDueDate: "2030-01-01",
		CheckToken:  "tok",
	}
	want := `<?xml version="1.0" encoding="UTF-8"?>
<status>active</status>
<validdirectory>HW-1</validdirectory>
<productname>PBS</productname>
<regdate>2026-01-01</regdate>
<nextduedate>2030-01-01</nextduedate>
<md5hash>` + ChallengeHash("tok") + `</md5hash>
`
	if got := r.RenderXML(); got != want {
		t.Fatalf("RenderXML mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestRenderXML_EscapesAndMessage(t *testing.T) {
	r := Response{Status: "invalid", ProductName: "A&B<C", Message: "m", CheckToken: "x"}
	got := r.RenderXML()
	if !strings.Contains(got, "<productname>A&amp;B&lt;C</productname>") {
		t.Errorf("escaping missing in: %s", got)
	}
	if !strings.Contains(got, "<message>m</message>") {
		t.Errorf("message tag missing in: %s", got)
	}
}

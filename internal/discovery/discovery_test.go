package discovery

import (
	"net"
	"testing"
)

func TestFoundAddIPsDeduplicates(t *testing.T) {
	var f Found
	f.addIPs([]net.IP{
		net.ParseIP("10.0.0.5"),
		net.ParseIP("10.0.0.5"), // duplicate
		net.ParseIP("192.168.1.9"),
		net.ParseIP("::ffff:10.0.0.5"), // IPv4-mapped IPv6 == 10.0.0.5 after Unmap
	})
	if len(f.IPs) != 2 {
		t.Fatalf("expected 2 unique addresses, got %d: %v", len(f.IPs), f.IPs)
	}
	if f.IPs[0].String() != "10.0.0.5" || f.IPs[1].String() != "192.168.1.9" {
		t.Errorf("unexpected addresses: %v", f.IPs)
	}
}

func TestFoundScheme(t *testing.T) {
	if (Found{Text: []string{"tls=http"}}).Scheme() != "http" {
		t.Error("tls=http should map to http")
	}
	if (Found{Text: []string{"tls=auto"}}).Scheme() != "https" {
		t.Error("default should be https")
	}
	if (Found{}).Scheme() != "https" {
		t.Error("no text should default to https")
	}
}

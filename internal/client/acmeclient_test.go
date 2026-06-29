package client

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"proxmox-license-proxy/internal/config"
	"proxmox-license-proxy/internal/registry"
	"proxmox-license-proxy/internal/transport/httpapi"
)

// startServer spins up the real /api/v1 handler over plain HTTP for the client
// to talk to, so this exercises the full ACME client<->server flow.
func startServer(t *testing.T) (string, *http.Client) {
	t.Helper()
	reg := filepath.Join(t.TempDir(), "registry.json")
	settings := &config.Settings{
		Listen:       ":0",
		RegistryFile: reg,
		TLS:          config.TLSSettings{Mode: config.TLSModeHTTP},
		API:          config.APISettings{AdminToken: "secret"},
	}
	srv, err := httpapi.New(settings, registry.NewStore(reg), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts.URL, ts.Client()
}

func TestACMEClientFullFlow(t *testing.T) {
	base, httpc := startServer(t)

	keyPath := filepath.Join(t.TempDir(), "account.key")
	priv, err := LoadOrCreateAccountKey(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	// The key is persisted and stable.
	if priv2, err := LoadOrCreateAccountKey(keyPath); err != nil || !priv2.Equal(priv) {
		t.Fatalf("account key not stable: %v", err)
	}

	c := NewACMEClient(base, httpc, priv)
	if err := c.Directory(); err != nil {
		t.Fatalf("directory: %v", err)
	}

	const serverid = "AABBCCDDEEFF"
	acc, err := c.Register(serverid, "admin@example.test")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if acc.Status != "PENDING" {
		t.Fatalf("new account should be PENDING, got %q", acc.Status)
	}

	// Order before approval -> pending.
	ord, err := c.Order(serverid, []string{"pve"}, "", "")
	if err != nil {
		t.Fatalf("order: %v", err)
	}
	if ord.Status != "pending" {
		t.Fatalf("want pending before approval, got %q", ord.Status)
	}

	// Admin approves the account.
	approveAccount(t, base, httpc, c.AccountID())

	// Order again -> issued for both products.
	ord, err = c.Order(serverid, []string{"pve", "pbs"}, "s", "2")
	if err != nil {
		t.Fatalf("order after approval: %v", err)
	}
	if ord.Status != "valid" || len(ord.Subscriptions) != 2 {
		t.Fatalf("want 2 issued subscriptions, got status=%q subs=%d", ord.Status, len(ord.Subscriptions))
	}
	for _, s := range ord.Subscriptions {
		if s.Key == "" {
			t.Errorf("issued %s without a key", s.Product)
		}
	}

	// The account can revoke its own subscription.
	if err := c.Revoke(ord.Subscriptions[0].Key); err != nil {
		t.Fatalf("revoke: %v", err)
	}
}

func approveAccount(t *testing.T, base string, httpc *http.Client, thumb string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/admin/accounts/"+thumb+"/approve", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := httpc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve account: status %d", resp.StatusCode)
	}
}

func TestDetectProducts(t *testing.T) {
	look := func(bin string) (string, error) {
		if bin == "proxmox-backup-manager" {
			return "/usr/bin/" + bin, nil
		}
		return "", http.ErrNotSupported // any error => not found
	}
	dir := func(path string) bool { return path == "/etc/pve" }

	got := detectProducts(look, dir)
	codes := map[string]bool{}
	for _, p := range got {
		codes[p.Code] = true
	}
	if !codes["pbs"] || !codes["pve"] {
		t.Fatalf("expected pve (dir) and pbs (bin), got %v", codes)
	}
	if codes["pmg"] {
		t.Errorf("pmg should not be detected")
	}
}

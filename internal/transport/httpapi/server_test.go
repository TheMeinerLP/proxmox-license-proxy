package httpapi

import (
	"io"
	"log/slog"
	nethttp "net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"proxmox-license-proxy/internal/config"
	"proxmox-license-proxy/internal/registry"
	"proxmox-license-proxy/internal/subscription"
)

func newTestServer(t *testing.T) (*Server, *registry.Store) {
	t.Helper()
	reg := filepath.Join(t.TempDir(), "registry.json")
	settings := &config.Settings{
		Listen:       ":0",
		RegistryFile: reg,
		LogLevel:     slog.LevelError,
		TLS:          config.TLSSettings{Mode: config.TLSModeHTTP, Names: []string{"shop.proxmox.com"}},
		Hosts:        config.HostsSettings{File: "/etc/hosts", Names: []string{"shop.proxmox.com"}},
	}
	store := registry.NewStore(reg)
	s, err := New(settings, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return s, store
}

func postVerify(t *testing.T, h nethttp.Handler, key, dir, token string) string {
	t.Helper()
	form := url.Values{"licensekey": {key}, "dir": {dir}, "check_token": {token}}.Encode()
	req := httptest.NewRequest(nethttp.MethodPost, "/modules/servers/licensing/verify.php", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Body.String()
}

func TestVerifyPendingThenApproved(t *testing.T) {
	s, store := newTestServer(t)
	h := s.Handler()

	// first contact -> auto-registered pending -> invalid
	if body := postVerify(t, h, "pbsc-1234567890", "HW-1", "tok"); !strings.Contains(body, "<status>invalid</status>") {
		t.Fatalf("expected invalid, got: %s", body)
	}
	servers, _ := store.ListServers()
	if len(servers) != 1 || servers[0].Status != subscription.Pending {
		t.Fatalf("expected one pending host, got %+v", servers)
	}

	// approve -> active with the correct challenge hash
	if _, err := store.SetServerStatus("HW-1", subscription.Approved); err != nil {
		t.Fatal(err)
	}
	body := postVerify(t, h, "pbsc-1234567890", "HW-1", "tok")
	if !strings.Contains(body, "<status>active</status>") {
		t.Fatalf("expected active, got: %s", body)
	}
	if !strings.Contains(body, "<validdirectory>HW-1</validdirectory>") {
		t.Errorf("validdirectory missing: %s", body)
	}
	if !strings.Contains(body, "<md5hash>"+subscription.ChallengeHash("tok")+"</md5hash>") {
		t.Errorf("md5hash wrong: %s", body)
	}
}

func TestHealthEndpoints(t *testing.T) {
	s, _ := newTestServer(t)
	h := s.Handler()
	for _, path := range []string{"/healthz", "/readyz", "/status"} {
		req := httptest.NewRequest(nethttp.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != nethttp.StatusOK {
			t.Errorf("%s = %d, want 200", path, rec.Code)
		}
	}
}

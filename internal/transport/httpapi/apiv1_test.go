package httpapi

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"proxmox-license-proxy/internal/acme"
	"proxmox-license-proxy/internal/config"
	"proxmox-license-proxy/internal/registry"
)

const testBase = "http://example.com" // httptest default Host, http scheme

func newV1Server(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	reg := filepath.Join(t.TempDir(), "registry.json")
	settings := &config.Settings{
		Listen:       ":443",
		RegistryFile: reg,
		TLS:          config.TLSSettings{Mode: config.TLSModeHTTP},
		API:          config.APISettings{AdminToken: "secret"},
	}
	srv, err := New(settings, registry.NewStore(reg), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return srv, srv.Handler()
}

// nonce fetches a fresh replay nonce from the directory's new-nonce endpoint.
func nonce(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, testBase+"/api/v1/new-nonce", nil))
	n := rec.Header().Get("Replay-Nonce")
	if n == "" {
		t.Fatal("no Replay-Nonce issued")
	}
	return n
}

// signedPOST signs payload as a JWS (jwk header when newAccount, else kid) and
// POSTs it, returning the recorder.
func signedPOST(t *testing.T, h http.Handler, priv ed25519.PrivateKey, jwk acme.JWK, path string, newAccount bool, n string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	hdr := acme.ProtectedHeader{Nonce: n, URL: testBase + path}
	if newAccount {
		j := jwk
		hdr.JWK = &j
	} else {
		hdr.KID = jwk.Thumbprint()
	}
	var body []byte
	if payload != nil {
		body, _ = json.Marshal(payload)
	}
	jws, err := acme.Sign(priv, hdr, body)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(jws)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, testBase+path, bytes.NewReader(raw)))
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return m
}

func TestV1IssuanceFlow(t *testing.T) {
	srv, h := newV1Server(t)
	pub, priv, _ := ed25519.GenerateKey(nil)
	jwk := acme.JWKFromEd25519(pub)
	const serverid = "AABBCCDDEEFF"

	// 1) Register account (jwk header). It starts PENDING (no auto-approve).
	rec := signedPOST(t, h, priv, jwk, "/api/v1/new-account", true, nonce(t, h),
		map[string]string{"serverid": serverid})
	if rec.Code != http.StatusCreated {
		t.Fatalf("new-account: %d %s", rec.Code, rec.Body)
	}

	// 2) Order before approval -> pending, nothing issued.
	rec = signedPOST(t, h, priv, jwk, "/api/v1/new-order", false, nonce(t, h),
		map[string]any{"serverid": serverid, "products": []string{"pve"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("order: %d %s", rec.Code, rec.Body)
	}
	if got := decode(t, rec)["status"]; got != "pending" {
		t.Fatalf("want pending before approval, got %v", got)
	}

	// 3) Admin approves the ACME account (the issuance gate).
	areq := httptest.NewRequest(http.MethodPost, testBase+"/api/v1/admin/accounts/"+jwk.Thumbprint()+"/approve", nil)
	areq.Header.Set("Authorization", "Bearer secret")
	arec := httptest.NewRecorder()
	h.ServeHTTP(arec, areq)
	if arec.Code != http.StatusOK {
		t.Fatalf("admin approve account: %d %s", arec.Code, arec.Body)
	}

	// 4) Order again -> issued.
	rec = signedPOST(t, h, priv, jwk, "/api/v1/new-order", false, nonce(t, h),
		map[string]any{"serverid": serverid, "products": []string{"pve", "pbs"}})
	m := decode(t, rec)
	if m["status"] != "valid" {
		t.Fatalf("want valid after approval, got %v (%s)", m["status"], rec.Body)
	}
	subs, _ := m["subscriptions"].([]any)
	if len(subs) != 2 {
		t.Fatalf("want 2 subscriptions, got %d: %s", len(subs), rec.Body)
	}
	first := subs[0].(map[string]any)
	key := first["key"].(string)

	// The issued subscription is active and assigned to the host.
	if res := srv.app.Verify(serverid, key, "tok", false); !res.Active {
		t.Fatalf("verify should be active for issued key %s", key)
	}

	// 5) Client revokes its own subscription.
	rec = signedPOST(t, h, priv, jwk, "/api/v1/revoke", false, nonce(t, h),
		map[string]string{"key": key})
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body)
	}

	// 6) verify.php now reports inactive (Let's-Encrypt-style invalidation).
	if res := srv.app.Verify(serverid, key, "tok", false); res.Active {
		t.Fatalf("verify should be inactive after revoke for %s", key)
	}
}

func TestV1NonceReplayRejected(t *testing.T) {
	_, h := newV1Server(t)
	pub, priv, _ := ed25519.GenerateKey(nil)
	jwk := acme.JWKFromEd25519(pub)

	n := nonce(t, h)
	if rec := signedPOST(t, h, priv, jwk, "/api/v1/new-account", true, n, nil); rec.Code != http.StatusCreated {
		t.Fatalf("first use: %d %s", rec.Code, rec.Body)
	}
	// Reusing the same nonce must be rejected.
	if rec := signedPOST(t, h, priv, jwk, "/api/v1/new-account", true, n, nil); rec.Code == http.StatusCreated {
		t.Fatal("replayed nonce was accepted")
	}
}

func TestV1AdminRequiresToken(t *testing.T) {
	_, h := newV1Server(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, testBase+"/api/v1/admin/hosts", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 without token, got %d", rec.Code)
	}
}

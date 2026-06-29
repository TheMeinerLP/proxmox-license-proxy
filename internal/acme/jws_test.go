package acme

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func TestSignVerifyKID(t *testing.T) {
	pub, priv := mustKey(t)
	jwk := JWKFromEd25519(pub)
	hdr := ProtectedHeader{Nonce: "n1", URL: "https://proxy/api/v1/order", KID: jwk.Thumbprint()}

	jws, err := Sign(priv, hdr, []byte(`{"serverid":"AABBCC"}`))
	if err != nil {
		t.Fatal(err)
	}
	raw := mustJSON(t, jws)

	v, err := Verify(raw, func(kid string) (ed25519.PublicKey, bool) {
		if kid == jwk.Thumbprint() {
			return pub, true
		}
		return nil, false
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if v.Thumbprint != jwk.Thumbprint() {
		t.Errorf("thumbprint mismatch: %s", v.Thumbprint)
	}
	if string(v.Payload) != `{"serverid":"AABBCC"}` {
		t.Errorf("payload: %s", v.Payload)
	}
	if v.Header.URL != hdr.URL {
		t.Errorf("url: %s", v.Header.URL)
	}
}

func TestVerifyNewAccountJWK(t *testing.T) {
	pub, priv := mustKey(t)
	hdr := ProtectedHeader{Nonce: "n1", URL: "https://proxy/api/v1/new-account"}
	jwk := JWKFromEd25519(pub)
	hdr.JWK = &jwk

	jws, _ := Sign(priv, hdr, []byte(`{"contact":"x"}`))
	// resolveKey must NOT be consulted for an embedded-jwk request.
	v, err := Verify(mustJSON(t, jws), func(string) (ed25519.PublicKey, bool) {
		t.Error("resolveKey should not be called for a jwk request")
		return nil, false
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.JWK == nil || v.Thumbprint != jwk.Thumbprint() {
		t.Error("expected embedded jwk and matching thumbprint")
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	pub, priv := mustKey(t)
	jwk := JWKFromEd25519(pub)
	jws, _ := Sign(priv, ProtectedHeader{Nonce: "n", URL: "u", KID: jwk.Thumbprint()}, []byte(`{"a":1}`))
	jws.Payload = b64.EncodeToString([]byte(`{"a":2}`)) // tamper

	_, err := Verify(mustJSON(t, jws), func(string) (ed25519.PublicKey, bool) { return pub, true })
	if err == nil {
		t.Fatal("expected signature failure on tampered payload")
	}
}

func TestVerifyUnknownAccount(t *testing.T) {
	_, priv := mustKey(t)
	jws, _ := Sign(priv, ProtectedHeader{Nonce: "n", URL: "u", KID: "nope"}, nil)
	_, err := Verify(mustJSON(t, jws), func(string) (ed25519.PublicKey, bool) { return nil, false })
	if !errors.Is(err, ErrUnknownAccount) {
		t.Fatalf("want ErrUnknownAccount, got %v", err)
	}
}

func TestThumbprintStable(t *testing.T) {
	pub, _ := mustKey(t)
	// Two JWKs built from the same key must yield the same thumbprint.
	if JWKFromEd25519(pub).Thumbprint() != JWKFromEd25519(pub).Thumbprint() {
		t.Error("thumbprint not stable")
	}
	j := JWKFromEd25519(pub)
	got, err := j.Ed25519()
	if err != nil || !got.Equal(pub) {
		t.Errorf("roundtrip JWK->key failed: %v", err)
	}
}

func TestNonceSingleUse(t *testing.T) {
	ns := NewNonceStore(time.Minute)
	n, err := ns.Issue()
	if err != nil {
		t.Fatal(err)
	}
	if !ns.Use(n) {
		t.Error("first use should succeed")
	}
	if ns.Use(n) {
		t.Error("second use must fail (replay)")
	}
	if ns.Use("never-issued") {
		t.Error("unknown nonce must fail")
	}
}

func TestNonceExpiry(t *testing.T) {
	ns := NewNonceStore(-time.Second) // already expired on issue
	n, _ := ns.Issue()
	if ns.Use(n) {
		t.Error("expired nonce must not be accepted")
	}
}

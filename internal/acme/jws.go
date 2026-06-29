// Package acme implements the small subset of the ACME (RFC 8555) security model
// this tool needs: Ed25519 account keys, flattened JWS request signing and
// verification, RFC 7638 JWK thumbprints and single-use replay nonces. It is the
// shared crypto core for the client (signing) and the server (verifying); it
// knows nothing about HTTP routing or subscriptions.
package acme

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// b64 is the unpadded base64url encoding used throughout JOSE/ACME.
var b64 = base64.RawURLEncoding

// JWK is an Ed25519 public key in JWK form (an OKP key, RFC 8037).
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
}

// JWKFromEd25519 builds the JWK for an Ed25519 public key.
func JWKFromEd25519(pub ed25519.PublicKey) JWK {
	return JWK{Kty: "OKP", Crv: "Ed25519", X: b64.EncodeToString(pub)}
}

// Ed25519 decodes the JWK back into a public key.
func (j JWK) Ed25519() (ed25519.PublicKey, error) {
	if j.Kty != "OKP" || j.Crv != "Ed25519" {
		return nil, fmt.Errorf("unsupported JWK %s/%s (want OKP/Ed25519)", j.Kty, j.Crv)
	}
	raw, err := b64.DecodeString(j.X)
	if err != nil {
		return nil, fmt.Errorf("invalid JWK x: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid Ed25519 key length %d", len(raw))
	}
	return ed25519.PublicKey(raw), nil
}

// Thumbprint is the RFC 7638 SHA-256 thumbprint (base64url) of the JWK. The
// members are serialized in lexicographic order with no whitespace, exactly as
// the spec requires, so it is a stable, key-derived account id.
func (j JWK) Thumbprint() string {
	canonical := fmt.Sprintf(`{"crv":"%s","kty":"%s","x":"%s"}`, j.Crv, j.Kty, j.X)
	sum := sha256.Sum256([]byte(canonical))
	return b64.EncodeToString(sum[:])
}

// ProtectedHeader is the JWS protected header. Exactly one of KID (existing
// account) or JWK (new account) is set, matching ACME.
type ProtectedHeader struct {
	Alg   string `json:"alg"`
	Nonce string `json:"nonce"`
	URL   string `json:"url"`
	KID   string `json:"kid,omitempty"`
	JWK   *JWK   `json:"jwk,omitempty"`
}

// JWS is a flattened JSON JWS (RFC 7515 §7.2.2).
type JWS struct {
	Protected string `json:"protected"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

// Sign produces a flattened JWS over the header and payload using EdDSA. A nil
// payload yields an empty payload segment (ACME "POST-as-GET").
func Sign(priv ed25519.PrivateKey, hdr ProtectedHeader, payload []byte) (JWS, error) {
	hdr.Alg = "EdDSA"
	hb, err := json.Marshal(hdr)
	if err != nil {
		return JWS{}, err
	}
	p64 := b64.EncodeToString(hb)
	pl64 := ""
	if payload != nil {
		pl64 = b64.EncodeToString(payload)
	}
	sig := ed25519.Sign(priv, []byte(p64+"."+pl64))
	return JWS{Protected: p64, Payload: pl64, Signature: b64.EncodeToString(sig)}, nil
}

// Verified is the result of verifying a JWS: the decoded header and payload plus
// the account thumbprint (from the kid, or computed from the embedded jwk).
type Verified struct {
	Header     ProtectedHeader
	Payload    []byte
	JWK        *JWK   // non-nil only for a new-account (embedded key) request
	Thumbprint string // account id
}

// ErrUnknownAccount is returned when a kid does not resolve to a known key.
var ErrUnknownAccount = errors.New("unknown account")

// Verify parses and cryptographically verifies a flattened JWS. For a kid-based
// request it asks resolveKey for the account's public key; for a jwk-based
// (new-account) request it uses the embedded key. It checks only the signature
// and structure - the caller must still validate Header.URL and Header.Nonce.
func Verify(raw []byte, resolveKey func(kid string) (ed25519.PublicKey, bool)) (*Verified, error) {
	var j JWS
	if err := json.Unmarshal(raw, &j); err != nil {
		return nil, fmt.Errorf("invalid JWS JSON: %w", err)
	}
	hb, err := b64.DecodeString(j.Protected)
	if err != nil {
		return nil, fmt.Errorf("invalid protected header: %w", err)
	}
	var hdr ProtectedHeader
	if err := json.Unmarshal(hb, &hdr); err != nil {
		return nil, fmt.Errorf("invalid protected header JSON: %w", err)
	}
	if hdr.Alg != "EdDSA" {
		return nil, fmt.Errorf("unsupported alg %q (want EdDSA)", hdr.Alg)
	}

	var pub ed25519.PublicKey
	var thumb string
	switch {
	case hdr.JWK != nil && hdr.KID == "":
		if pub, err = hdr.JWK.Ed25519(); err != nil {
			return nil, err
		}
		thumb = hdr.JWK.Thumbprint()
	case hdr.KID != "" && hdr.JWK == nil:
		var ok bool
		if pub, ok = resolveKey(hdr.KID); !ok {
			return nil, ErrUnknownAccount
		}
		thumb = hdr.KID
	default:
		return nil, errors.New("JWS must carry exactly one of kid or jwk")
	}

	sig, err := b64.DecodeString(j.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature: %w", err)
	}
	if !ed25519.Verify(pub, []byte(j.Protected+"."+j.Payload), sig) {
		return nil, errors.New("JWS signature verification failed")
	}

	var payload []byte
	if j.Payload != "" {
		if payload, err = b64.DecodeString(j.Payload); err != nil {
			return nil, fmt.Errorf("invalid payload: %w", err)
		}
	}
	return &Verified{Header: hdr, Payload: payload, JWK: hdr.JWK, Thumbprint: thumb}, nil
}

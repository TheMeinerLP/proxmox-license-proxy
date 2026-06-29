package client

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"proxmox-license-proxy/internal/acme"
	"proxmox-license-proxy/internal/fileio"
)

// LoadOrCreateAccountKey returns the Ed25519 account key at path, creating and
// persisting a new one (PKCS#8 PEM, 0600) if none exists. The key is the host's
// stable ACME identity, like a Let's Encrypt account key.
func LoadOrCreateAccountKey(path string) (ed25519.PrivateKey, error) {
	if data, err := fileio.ReadFile(path); err == nil {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("account key %s is not PEM", path)
		}
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse account key %s: %w", path, err)
		}
		priv, ok := key.(ed25519.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("account key %s is not Ed25519", path)
		}
		return priv, nil
	}

	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, err
		}
	}
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		return nil, fmt.Errorf("write account key %s: %w", path, err)
	}
	return priv, nil
}

// ACMEClient talks to the /api/v1 ACME-style API: it signs requests with the
// account key and tracks the replay nonce across calls.
type ACMEClient struct {
	base  string
	http  *http.Client
	priv  ed25519.PrivateKey
	jwk   acme.JWK
	kid   string // JWK thumbprint = account id
	dir   map[string]string
	nonce string
}

// NewACMEClient builds a client for the proxy at base using the given HTTP
// client (which carries the TLS trust/pinning policy) and account key.
func NewACMEClient(base string, httpc *http.Client, priv ed25519.PrivateKey) *ACMEClient {
	pub := priv.Public().(ed25519.PublicKey)
	jwk := acme.JWKFromEd25519(pub)
	return &ACMEClient{base: base, http: httpc, priv: priv, jwk: jwk, kid: jwk.Thumbprint()}
}

// AccountID returns the account thumbprint (its id), e.g. for display.
func (c *ACMEClient) AccountID() string { return c.kid }

// Directory fetches the endpoint map; it must be called before the others.
func (c *ACMEClient) Directory() error {
	resp, err := c.http.Get(c.base + "/api/v1/directory")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("directory: status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(&c.dir)
}

func (c *ACMEClient) refreshNonce() error {
	url := c.endpoint("newNonce")
	resp, err := c.http.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	c.nonce = resp.Header.Get("Replay-Nonce")
	if c.nonce == "" {
		return fmt.Errorf("server did not issue a nonce")
	}
	return nil
}

func (c *ACMEClient) endpoint(name string) string {
	if u, ok := c.dir[name]; ok {
		return u
	}
	return c.base + "/api/v1/" + name
}

// post signs payload and POSTs it to the named directory endpoint, decoding the
// JSON response into out (when non-nil). useJWK embeds the account key (for
// new-account); otherwise the request is signed with the account id (kid).
func (c *ACMEClient) post(name string, payload any, useJWK bool, out any) (int, error) {
	if c.nonce == "" {
		if err := c.refreshNonce(); err != nil {
			return 0, err
		}
	}
	url := c.endpoint(name)
	hdr := acme.ProtectedHeader{Nonce: c.nonce, URL: url}
	if useJWK {
		j := c.jwk
		hdr.JWK = &j
	} else {
		hdr.KID = c.kid
	}

	var body []byte
	if payload != nil {
		var err error
		if body, err = json.Marshal(payload); err != nil {
			return 0, err
		}
	}
	jws, err := acme.Sign(c.priv, hdr, body)
	if err != nil {
		return 0, err
	}
	raw, err := json.Marshal(jws)
	if err != nil {
		return 0, err
	}

	resp, err := c.http.Post(url, "application/jose+json", bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	// Consume the fresh nonce so the next signed call can chain off it.
	if n := resp.Header.Get("Replay-Nonce"); n != "" {
		c.nonce = n
	} else {
		c.nonce = ""
	}
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return resp.StatusCode, fmt.Errorf("%s: %s", name, string(data))
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode %s response: %w", name, err)
		}
	}
	return resp.StatusCode, nil
}

// AccountResult is the new-account response.
type AccountResult struct {
	Thumbprint string `json:"thumbprint"`
	ServerID   string `json:"serverid"`
	Status     string `json:"status"`
}

// Register creates (or refreshes) the account, embedding the public key.
func (c *ACMEClient) Register(serverid, contact string) (AccountResult, error) {
	var res AccountResult
	_, err := c.post("newAccount", map[string]string{"serverid": serverid, "contact": contact}, true, &res)
	return res, err
}

// IssuedSubscription is one subscription returned by an order.
type IssuedSubscription struct {
	Product     string `json:"product"`
	Key         string `json:"key"`
	Status      string `json:"status"`
	ProductName string `json:"productName"`
	NextDueDate string `json:"nextDueDate"`
}

// OrderResult is the new-order response.
type OrderResult struct {
	ServerID      string               `json:"serverid"`
	Status        string               `json:"status"` // valid | pending
	Subscriptions []IssuedSubscription `json:"subscriptions"`
	Pending       []string             `json:"pending"`
	Problems      map[string]string    `json:"problems"`
}

// Order requests subscriptions for the given products. When the account is not
// yet approved the result is Status "pending" with the products listed in Pending.
func (c *ACMEClient) Order(serverid string, products []string, level, sockets string) (OrderResult, error) {
	var res OrderResult
	_, err := c.post("newOrder", map[string]any{
		"serverid": serverid, "products": products, "level": level, "sockets": sockets,
	}, false, &res)
	return res, err
}

// Revoke invalidates one of the account's own subscriptions.
func (c *ACMEClient) Revoke(key string) error {
	_, err := c.post("revoke", map[string]string{"key": key}, false, nil)
	return err
}

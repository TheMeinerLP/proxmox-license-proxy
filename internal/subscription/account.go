package subscription

// Account is an ACME-style client identity. A host (the Proxmox client) creates
// one by registering its public key; every subsequent request is JWS-signed with
// the matching private key. The account is referenced by the JWK thumbprint
// (a stable, key-derived id), so there is no separate secret to store server-side.
type Account struct {
	// Thumbprint is the RFC 7638 SHA-256 JWK thumbprint of the public key; it is
	// the account id used as the JWS "kid".
	Thumbprint string `json:"thumbprint" yaml:"thumbprint"`
	// PublicKey is the base64url-encoded raw Ed25519 public key (32 bytes), used
	// to verify the account's JWS signatures.
	PublicKey string `json:"publicKey" yaml:"publicKey"`
	// ServerID is the Proxmox host this account enrolled for (the verify "dir"),
	// as reported by the client. Informational - verify.php matches by key.
	ServerID string `json:"serverid,omitempty" yaml:"serverid,omitempty"`
	Contact  string `json:"contact,omitempty" yaml:"contact,omitempty"`
	// Status gates self-issuance, like host approval but for the ACME identity:
	// an APPROVED account may issue subscriptions; PENDING waits for an admin (or
	// the auto-approve-by-IP policy); BLOCKED is refused.
	Status    Status `json:"status" yaml:"status"`
	CreatedAt string `json:"createdAt" yaml:"createdAt"`
}

// Package certs generates self-signed TLS certificates and installs them into
// the system trust store. The Proxmox client only talks to shop.proxmox.com over
// HTTPS and verifies the certificate against the system trust store, so the
// generated certificate is issued for that hostname and must be trusted on the
// Proxmox host.
package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GenerateSelfSigned creates a self-signed certificate (PEM) and matching
// private key (PEM) for the given hosts/IPs.
func GenerateSelfSigned(hosts []string, validFor time.Duration) (certPEM, keyPEM []byte, err error) {
	if len(hosts) == 0 {
		return nil, nil, fmt.Errorf("at least one host is required")
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("generate key: %w", err)
	}

	serialMax := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialMax)
	if err != nil {
		return nil, nil, fmt.Errorf("serial: %w", err)
	}

	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: hosts[0], Organization: []string{"pmox-proxy"}},
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              now.Add(validFor),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}

	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("create certificate: %w", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return certPEM, keyPEM, nil
}

// Fingerprint returns the SHA-256 fingerprint of the first certificate in the
// PEM, formatted like "AB:CD:...". It lets a user verify a bootstrapped CA out
// of band (e.g. against `cert generate` output on the server).
func Fingerprint(certPEM []byte) string {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return ""
	}
	sum := sha256.Sum256(block.Bytes)
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":")
}

// InstallTrust copies a certificate into the system trust store and refreshes
// it. Requires root.
func InstallTrust(certPEM []byte, dest string) error {
	//nolint:gosec // G306: a CA cert is public and must be world-readable in the trust store
	if err := os.WriteFile(dest, certPEM, 0o644); err != nil {
		return fmt.Errorf("write %s (need root?): %w", dest, err)
	}
	out, err := exec.Command("update-ca-certificates").CombinedOutput()
	if err != nil {
		return fmt.Errorf("update-ca-certificates: %w: %s", err, out)
	}
	return nil
}

// RemoveTrust deletes a previously installed certificate and refreshes the trust
// store. The bool reports whether the file existed.
func RemoveTrust(dest string) (bool, error) {
	if err := os.Remove(dest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("remove %s (need root?): %w", dest, err)
	}
	out, err := exec.Command("update-ca-certificates", "--fresh").CombinedOutput()
	if err != nil {
		return true, fmt.Errorf("update-ca-certificates: %w: %s", err, out)
	}
	return true, nil
}

// Download fetches a certificate from the server's cert endpoint. TLS
// verification is skipped on purpose: this is a one-time trust-on-first-use
// bootstrap that runs BEFORE the certificate is trusted, so there is nothing to
// verify against yet. Callers should show Fingerprint(result) so the user can
// confirm the CA out of band.
func Download(url string) ([]byte, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			// G402: TOFU bootstrap fetch of the self-signed CA before it is trusted;
			// the caller verifies the fingerprint out of band.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // G402: see comment above
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

package cli

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"proxmox-license-proxy/internal/certs"
	"proxmox-license-proxy/internal/client"
	"proxmox-license-proxy/internal/hosts"
)

var (
	enrollServer     string
	enrollKeyPath    string
	enrollServerID   string
	enrollProducts   []string
	enrollLevel      string
	enrollSockets    string
	enrollContact    string
	enrollNoSet      bool
	enrollNoRedirect bool
	enrollIP         string
)

var clientEnrollCmd = &cobra.Command{
	Use:   "enroll",
	Short: "Obtain and install subscriptions automatically (ACME-style)",
	Long: "enroll detects the Proxmox products on this host (PVE/PBS/PMG), registers an\n" +
		"ACME account with the proxy, requests a subscription per product and installs\n" +
		"each one. If the account is not approved yet it prints its id and exits; re-run\n" +
		"after an admin approves it. Like certbot, but for the subscription nag.",
	RunE: runClientEnroll,
}

func runClientEnroll(cmd *cobra.Command, args []string) error {
	if enrollServer == "" {
		if !interactiveTTY() {
			return fmt.Errorf("--server is required (e.g. https://192.168.68.100)")
		}
		if err := promptInput("Proxy server URL", "https://192.168.68.100", &enrollServer); err != nil {
			return err
		}
	}
	enrollServer = strings.TrimRight(enrollServer, "/")

	// Trust the proxy CA (trust-on-first-use): download it, show the fingerprint
	// and build an HTTP client that pins it. Skipped for plain http servers.
	httpc, caPEM, err := enrollHTTPClient(enrollServer)
	if err != nil {
		return err
	}

	// Detect the products to enroll (override with --products).
	products := resolveEnrollProducts()
	if len(products) == 0 {
		return fmt.Errorf("no Proxmox products detected; pass --products pve,pbs,pmg to force")
	}
	codes := make([]string, len(products))
	for i, p := range products {
		codes[i] = p.Code
	}
	fmt.Printf("enrolling products: %s\n", strings.Join(codes, ", "))

	serverid := enrollServerID
	if serverid == "" {
		serverid = hostServerID()
	}

	// ACME: account key -> register -> order.
	priv, err := client.LoadOrCreateAccountKey(enrollKeyPath)
	if err != nil {
		return err
	}
	c := client.NewACMEClient(enrollServer, httpc, priv)
	if err := c.Directory(); err != nil {
		return fmt.Errorf("reach proxy API: %w", err)
	}
	acc, err := c.Register(serverid, enrollContact)
	if err != nil {
		return fmt.Errorf("register account: %w", err)
	}
	fmt.Printf("account: %s (status %s)\n", acc.Thumbprint, acc.Status)

	order, err := c.Order(serverid, codes, enrollLevel, enrollSockets)
	if err != nil {
		return fmt.Errorf("order subscriptions: %w", err)
	}
	if len(order.Subscriptions) == 0 {
		fmt.Printf("\naccount not approved yet - nothing issued.\n")
		fmt.Printf("ask an admin to approve account %s, then run `client enroll` again.\n", acc.Thumbprint)
		fmt.Printf("  admin: proxmox-license-proxy account approve %s\n", acc.Thumbprint)
		return nil
	}

	// Set up the redirect + trust so verify.php reaches the proxy, then install
	// each issued key with the product's own tool.
	if !enrollNoRedirect {
		if err := enrollRedirect(caPEM); err != nil {
			return err
		}
	}
	byCode := map[string]client.Product{}
	for _, p := range products {
		byCode[p.Code] = p
	}
	for _, sub := range order.Subscriptions {
		fmt.Printf("issued %s: %s\n", sub.Product, sub.Key)
		if enrollNoSet {
			continue
		}
		p, ok := byCode[sub.Product]
		if !ok {
			continue
		}
		if err := p.SetKey(sub.Key); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: could not set %s key automatically: %v\n", sub.Product, err)
			fmt.Printf("  set it manually: %s %s\n", strings.Join(p.SetCommand, " "), sub.Key)
		} else {
			fmt.Printf("  installed on %s\n", p.Name)
		}
	}
	if len(order.Pending) > 0 {
		fmt.Printf("still pending (account not approved for): %s\n", strings.Join(order.Pending, ", "))
	}
	fmt.Println("enroll complete.")
	return nil
}

// resolveEnrollProducts maps the --products override to Product entries, or
// auto-detects when not given.
func resolveEnrollProducts() []client.Product {
	if len(enrollProducts) == 0 {
		return client.DetectProducts()
	}
	all := client.AllProducts()
	byCode := map[string]client.Product{}
	for _, p := range all {
		byCode[p.Code] = p
	}
	var out []client.Product
	for _, code := range enrollProducts {
		if p, ok := byCode[strings.TrimSpace(code)]; ok {
			out = append(out, p)
		}
	}
	return out
}

// enrollHTTPClient builds an HTTP client trusting the proxy's CA. For an https
// server it downloads /ca.crt, prints the fingerprint and pins it; for an http
// server it returns the default client.
func enrollHTTPClient(server string) (*http.Client, []byte, error) {
	if strings.HasPrefix(server, "http://") {
		return &http.Client{Timeout: 30 * time.Second}, nil, nil
	}
	caPEM, err := certs.Download(server + "/ca.crt")
	if err != nil {
		return nil, nil, fmt.Errorf("download CA from %s/ca.crt: %w", server, err)
	}
	if fp := certs.Fingerprint(caPEM); fp != "" {
		fmt.Printf("proxy CA SHA-256: %s\n", fp)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, nil, fmt.Errorf("downloaded CA is not a valid certificate")
	}
	// The auto cert is issued for shop.proxmox.com; override ServerName so
	// verification succeeds even when we connect to the proxy by IP.
	tlsCfg := &tls.Config{RootCAs: pool, ServerName: "shop.proxmox.com", MinVersion: tls.VersionTLS12}
	httpc := &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}
	return httpc, caPEM, nil
}

// enrollRedirect trusts the CA in the system store and points shop.proxmox.com
// at the proxy, so the Proxmox products' subscription checks reach it.
func enrollRedirect(caPEM []byte) error {
	if len(caPEM) > 0 {
		if err := certs.InstallTrust(caPEM, "/usr/local/share/ca-certificates/pmox.crt"); err != nil {
			return fmt.Errorf("trust proxy CA: %w", err)
		}
	}
	ip := enrollIP
	if ip == "" {
		ip = hostFromURL(enrollServer)
	}
	if _, err := hosts.Enable("/etc/hosts", ip, []string{"shop.proxmox.com"}, false); err != nil {
		return fmt.Errorf("point shop.proxmox.com at the proxy: %w", err)
	}
	return nil
}

// hostServerID returns a stable host identifier for the account/order, from the
// machine id or hostname. It is informational; verify.php matches by key.
func hostServerID() string {
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			return id
		}
	}
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return "unknown"
}

func init() {
	clientCmd.AddCommand(clientEnrollCmd)
	clientEnrollCmd.Flags().StringVar(&enrollServer, "server", "", "proxy server URL (e.g. https://192.168.68.100)")
	clientEnrollCmd.Flags().StringVar(&enrollKeyPath, "account-key", "/etc/pmox/account.key", "path to the ACME account key (created if absent)")
	clientEnrollCmd.Flags().StringVar(&enrollServerID, "serverid", "", "host id to report (default: machine-id)")
	clientEnrollCmd.Flags().StringSliceVar(&enrollProducts, "products", nil, "products to enroll (default: auto-detect)")
	clientEnrollCmd.Flags().StringVar(&enrollLevel, "level", "", "subscription level c|b|s|p (default community)")
	clientEnrollCmd.Flags().StringVar(&enrollSockets, "sockets", "", "PVE CPU sockets 1|2|4|8")
	clientEnrollCmd.Flags().StringVar(&enrollContact, "contact", "", "optional contact recorded with the account")
	clientEnrollCmd.Flags().BoolVar(&enrollNoSet, "no-set", false, "do not run the product's subscription-set command")
	clientEnrollCmd.Flags().BoolVar(&enrollNoRedirect, "no-redirect", false, "do not trust the CA or edit /etc/hosts")
	clientEnrollCmd.Flags().StringVar(&enrollIP, "ip", "", "proxy IP for the hosts redirect (default: from --server)")
}

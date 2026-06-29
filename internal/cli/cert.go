package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"proxmox-license-proxy/internal/certs"
	"proxmox-license-proxy/internal/fileio"
)

var (
	certHosts    []string
	certOutCert  string
	certOutKey   string
	certDays     int
	certInstall  string
	certFrom     string
	certInstDest string
)

var certCmd = &cobra.Command{
	Use:   "cert",
	Short: "Generate and install TLS certificates",
}

var certGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a self-signed certificate for shop.proxmox.com",
	RunE: func(cmd *cobra.Command, args []string) error {
		// On a terminal, confirm the hostnames and validity instead of silently
		// using flag defaults; passing --host/--days skips the matching prompt.
		if interactiveTTY() && (!cmd.Flags().Changed("host") || !cmd.Flags().Changed("days")) {
			hostsCSV := strings.Join(certHosts, ", ")
			daysStr := strconv.Itoa(certDays)
			fields := []huh.Field{}
			if !cmd.Flags().Changed("host") {
				fields = append(fields, huh.NewInput().Title("Hostname(s) / IP(s) for the certificate").
					Description("Comma-separated. Proxmox talks to shop.proxmox.com, so keep that.").
					Value(&hostsCSV))
			}
			if !cmd.Flags().Changed("days") {
				fields = append(fields, huh.NewInput().Title("Validity in days").Value(&daysStr).
					Validate(func(s string) error { _, err := strconv.Atoi(s); return err }))
			}
			if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
				return err
			}
			certHosts = parseNetworks(hostsCSV) // reuse the comma/space splitter
			if d, err := strconv.Atoi(daysStr); err == nil {
				certDays = d
			}
		}
		if len(certHosts) == 0 {
			certHosts = []string{"shop.proxmox.com"}
		}

		certPEM, keyPEM, err := certs.GenerateSelfSigned(certHosts, time.Duration(certDays)*24*time.Hour)
		if err != nil {
			return err
		}
		//nolint:gosec // G306: a TLS certificate is public and meant to be world-readable
		if err := os.WriteFile(certOutCert, certPEM, 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(certOutKey, keyPEM, 0o600); err != nil {
			return err
		}
		fmt.Printf("wrote %s and %s for %v (valid %d days)\n", certOutCert, certOutKey, certHosts, certDays)
		fmt.Printf("  SHA-256: %s\n", certs.Fingerprint(certPEM))
		fmt.Printf("  next: trust it on a Proxmox host with `cert install --cert %s` (or `client install`)\n", certOutCert)
		return nil
	},
}

var certInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Trust a certificate (from a file or the server's /ca.crt)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// On a terminal with no source given, ask whether to fetch from the
		// running proxy or read a local file - the two real options.
		if interactiveTTY() && certFrom == "" && !cmd.Flags().Changed("cert") {
			source := "url"
			if err := promptSelect("Where is the certificate?", &source,
				huh.NewOption("Fetch from the proxy's /ca.crt URL", "url"),
				huh.NewOption("Read a local PEM file", "file"),
			); err != nil {
				return err
			}
			if source == "url" {
				if err := promptInput("Proxy URL", "https://192.168.68.100/ca.crt", &certFrom); err != nil {
					return err
				}
			} else {
				if err := promptInput("Certificate file", "cert.pem", &certInstall); err != nil {
					return err
				}
			}
		}

		var certPEM []byte
		var err error
		switch {
		case certFrom != "":
			certPEM, err = certs.Download(certFrom)
			if err != nil {
				return err
			}
		default:
			certPEM, err = fileio.ReadFile(certInstall)
			if err != nil {
				return err
			}
		}

		// Show the fingerprint and confirm before touching the system trust store.
		fmt.Printf("certificate SHA-256: %s\n", certs.Fingerprint(certPEM))
		if interactiveTTY() {
			if err := confirm(cmd, false, fmt.Sprintf("Trust this certificate (install into %s)?", certInstDest)); err != nil {
				return err
			}
		}

		if err := certs.InstallTrust(certPEM, certInstDest); err != nil {
			return err
		}
		fmt.Printf("installed certificate into the trust store at %s\n", certInstDest)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(certCmd)
	certCmd.AddCommand(certGenerateCmd, certInstallCmd)

	certGenerateCmd.Flags().StringSliceVar(&certHosts, "host", []string{"shop.proxmox.com"}, "hostname(s)/IP(s) for the certificate")
	certGenerateCmd.Flags().StringVar(&certOutCert, "out-cert", "cert.pem", "output certificate file")
	certGenerateCmd.Flags().StringVar(&certOutKey, "out-key", "key.pem", "output key file")
	certGenerateCmd.Flags().IntVar(&certDays, "days", 3650, "validity in days")

	certInstallCmd.Flags().StringVar(&certInstall, "cert", "cert.pem", "certificate file to install")
	certInstallCmd.Flags().StringVar(&certFrom, "from", "", "fetch the certificate from this URL (e.g. https://proxy/ca.crt)")
	certInstallCmd.Flags().StringVar(&certInstDest, "dest", "/usr/local/share/ca-certificates/pmox.crt", "destination in the trust store")
}

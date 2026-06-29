package cli

import (
	"fmt"
	"os"
	"time"

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
		return nil
	},
}

var certInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Trust a certificate (from a file or the server's /ca.crt)",
	RunE: func(cmd *cobra.Command, args []string) error {
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

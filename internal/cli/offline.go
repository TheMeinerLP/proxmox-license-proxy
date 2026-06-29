package cli

import "github.com/spf13/cobra"

var (
	offlineOutPriv  string
	offlineOutPub   string
	offlineServerID string
	offlinePriv     string
	offlineDue      string
	offlineSignOut  string
	offlinePub      string
	offlinePubDest  string
)

var offlineCmd = &cobra.Command{
	Use:   "offline",
	Short: "Offline signing key workflow (Ed25519)",
}

var offlineKeygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate an Ed25519 signing key pair",
	RunE:  notImplemented,
}

var offlineSignCmd = &cobra.Command{
	Use:   "sign <key>",
	Short: "Sign an offline subscription blob for a host",
	Args:  cobra.ExactArgs(1),
	RunE:  notImplemented,
}

var offlineInstallPubCmd = &cobra.Command{
	Use:   "install-pubkey",
	Short: "Replace the Proxmox offline signing public key on a host",
	RunE:  notImplemented,
}

func init() {
	rootCmd.AddCommand(offlineCmd)
	offlineCmd.AddCommand(offlineKeygenCmd, offlineSignCmd, offlineInstallPubCmd)

	offlineKeygenCmd.Flags().StringVar(&offlineOutPriv, "out-priv", "offline.key", "output private key file")
	offlineKeygenCmd.Flags().StringVar(&offlineOutPub, "out-pub", "offline.pub", "output public key file")

	offlineSignCmd.Flags().StringVar(&offlineServerID, "serverid", "", "target host server id (required)")
	offlineSignCmd.Flags().StringVar(&offlinePriv, "priv", "offline.key", "private key file to sign with")
	offlineSignCmd.Flags().StringVar(&offlineDue, "due", "", "expiry date YYYY-MM-DD")
	offlineSignCmd.Flags().StringVar(&offlineSignOut, "out", "", "output blob file (default: stdout)")

	offlineInstallPubCmd.Flags().StringVar(&offlinePub, "pub", "offline.pub", "public key file to install")
	offlineInstallPubCmd.Flags().StringVar(&offlinePubDest, "dest", "/usr/share/keyrings/proxmox-offline-signing-key.pub", "destination path on the host")
}

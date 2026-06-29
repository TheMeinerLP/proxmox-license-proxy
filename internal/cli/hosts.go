package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"proxmox-license-proxy/internal/hosts"
)

var (
	hostsFile    string
	hostsIP      string
	hostsDryRun  bool
	hostsYes     bool
	hostsEnableY bool
)

var hostsCmd = &cobra.Command{
	Use:   "hosts",
	Short: "Manage the shop.proxmox.com entry in /etc/hosts",
}

// resolveHostsFile picks the --file flag or falls back to the config value.
func resolveHostsFile() string {
	if hostsFile != "" {
		return hostsFile
	}
	return settings.Hosts.File
}

var hostsEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Point the Proxmox shop hostname at the proxy in the hosts file",
	RunE: func(cmd *cobra.Command, args []string) error {
		ip := hostsIP
		if ip == "" && settings.Hosts.IP.IsValid() {
			ip = settings.Hosts.IP.String()
		}
		// Ask for the proxy IP when it is not configured and we can prompt, so
		// the command does not silently fall back to an unexpected default.
		if ip == "" && interactiveTTY() {
			if err := promptInput("Proxy IP to point "+hostsNamesLabel()+" at", "e.g. 192.168.68.100", &ip); err != nil {
				return err
			}
		}

		// Editing /etc/hosts is system-wide; confirm on a terminal first. Scripts
		// and systemd (non-interactive) proceed without blocking, as before.
		if !hostsDryRun && interactiveTTY() {
			if err := confirm(cmd, hostsEnableY,
				fmt.Sprintf("Point %s at %s in %s?", hostsNamesLabel(), ipOrDefault(ip), resolveHostsFile())); err != nil {
				return err
			}
		}

		out, err := hosts.Enable(resolveHostsFile(), ip, settings.Hosts.Names, hostsDryRun)
		if err != nil {
			return err
		}
		if hostsDryRun {
			fmt.Print(out)
			return nil
		}
		fmt.Printf("updated %s:\n%s", resolveHostsFile(), out)
		return nil
	},
}

// hostsNamesLabel renders the managed hostnames for prompts, defaulting to the
// Proxmox shop host when none are configured.
func hostsNamesLabel() string {
	if len(settings.Hosts.Names) == 0 {
		return "shop.proxmox.com"
	}
	return fmt.Sprintf("%v", settings.Hosts.Names)
}

// ipOrDefault shows the IP that will be written, making the default explicit.
func ipOrDefault(ip string) string {
	if ip == "" {
		return "127.0.0.1 (default)"
	}
	return ip
}

var hostsDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Remove the managed hosts entry",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !hostsDryRun {
			if err := confirm(cmd, hostsYes, fmt.Sprintf("Remove the managed entry from %s?", resolveHostsFile())); err != nil {
				return err
			}
		}
		out, err := hosts.Disable(resolveHostsFile(), hostsDryRun)
		if err != nil {
			return err
		}
		if hostsDryRun {
			fmt.Print(out)
			return nil
		}
		fmt.Printf("removed managed entry from %s\n", resolveHostsFile())
		return nil
	},
}

var hostsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether the managed hosts entry is present",
	RunE: func(cmd *cobra.Command, args []string) error {
		present, block, err := hosts.Status(resolveHostsFile())
		if err != nil {
			return err
		}
		if !present {
			fmt.Printf("no managed entry in %s\n", resolveHostsFile())
			return nil
		}
		fmt.Printf("managed entry in %s:\n%s\n", resolveHostsFile(), block)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(hostsCmd)
	hostsCmd.AddCommand(hostsEnableCmd, hostsDisableCmd, hostsStatusCmd)

	hostsCmd.PersistentFlags().StringVar(&hostsFile, "file", "", "hosts file path (default: config hosts.file)")
	hostsCmd.PersistentFlags().BoolVar(&hostsDryRun, "dry-run", false, "print the change instead of writing")
	hostsDisableCmd.Flags().BoolVarP(&hostsYes, "yes", "y", false, "skip confirmation prompt")

	hostsEnableCmd.Flags().StringVar(&hostsIP, "ip", "", "proxy IP to point the names at (default: config hosts.ip)")
	hostsEnableCmd.Flags().BoolVarP(&hostsEnableY, "yes", "y", false, "skip confirmation prompt")
}

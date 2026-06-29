package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// defaultConfigPath is where the server (and its systemd unit) reads its config,
// and therefore the default target for `config init` / `setup server`.
const defaultConfigPath = "/etc/pmox/config.yaml"

// ensureDirFor creates the parent directory of path (e.g. /etc/pmox) so writing
// the config there works on a fresh host, not only after the package created it.
func ensureDirFor(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

// defaultConfigYAML is the scaffold written by `config init`. It mirrors the
// defaults applied by config.Load, with every key shown commented intent.
const defaultConfigYAML = `# proxmox-license-proxy configuration
listen: ":443"
log: "info"
registry_file: "/etc/pmox/registry.json"

tls:
  mode: "auto"          # auto | files | http
  # cert: "/etc/pmox/tls.crt"
  # key:  "/etc/pmox/tls.key"
  names:
    - "shop.proxmox.com"

hosts:
  file: "/etc/hosts"
  # ip: "127.0.0.1"
  names:
    - "shop.proxmox.com"

# Auto-approve hosts contacting from a trusted source IP (else they stay PENDING
# until 'server approve'). An operator's BLOCKED/REJECTED decision always wins.
auto_approve:
  enabled: false
  private: true         # trust RFC1918 / ULA / loopback / link-local
  # networks:
  #   - "100.64.0.0/10"
`

var configInitOut string
var configInitForce bool
var configInitDefaults bool

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect and scaffold the application config",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a config.yaml (guided wizard on a terminal, or --defaults)",
	Long: "On a terminal this runs the guided server wizard (same as `setup server`),\n" +
		"asking about TLS, the registry, auto-approval and more. Pass --defaults to\n" +
		"skip the questions and write a commented scaffold instead.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat(configInitOut); err == nil && !configInitForce {
			return fmt.Errorf("%s already exists; use --force to overwrite", configInitOut)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := ensureDirFor(configInitOut); err != nil {
			return fmt.Errorf("create directory for %s (need root?): %w", configInitOut, err)
		}

		// Guided by default on a terminal; --defaults forces the static scaffold.
		if !configInitDefaults && interactiveTTY() {
			choices, err := runServerWizard()
			if err != nil {
				return err
			}
			if err := applyServerWizard(choices).WriteConfigAs(configInitOut); err != nil {
				return fmt.Errorf("write %s: %w", configInitOut, err)
			}
			fmt.Printf("\nwrote %s\n", configInitOut)
			fmt.Printf("start the server with: proxmox-license-proxy serve --config %s\n", configInitOut)
			return nil
		}

		if err := os.WriteFile(configInitOut, []byte(defaultConfigYAML), 0o600); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", configInitOut)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the effective configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("listen:        %s\n", settings.Listen)
		fmt.Printf("log:           %s\n", settings.LogLevel)
		fmt.Printf("registry_file: %s\n", settings.RegistryFile)
		fmt.Printf("tls.mode:      %s\n", settings.TLS.Mode)
		fmt.Printf("tls.names:     %v\n", settings.TLS.Names)
		fmt.Printf("hosts.file:    %s\n", settings.Hosts.File)
		if settings.Hosts.IP.IsValid() {
			fmt.Printf("hosts.ip:      %s\n", settings.Hosts.IP)
		} else {
			fmt.Printf("hosts.ip:      (unset)\n")
		}
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the config file that was loaded",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfgUsed == "" {
			fmt.Println("(no config file found; using defaults and environment)")
			return nil
		}
		fmt.Println(cfgUsed)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInitCmd, configShowCmd, configPathCmd)

	configInitCmd.Flags().StringVar(&configInitOut, "out", defaultConfigPath, "output path")
	configInitCmd.Flags().BoolVar(&configInitForce, "force", false, "overwrite existing file")
	configInitCmd.Flags().BoolVar(&configInitDefaults, "defaults", false, "skip the wizard; write a commented default scaffold")
}

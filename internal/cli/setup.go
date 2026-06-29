package cli

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var setupOut string

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup wizards",
}

// setup server: collect settings and write a config.yaml.
// (Client setup lives in `client install`.)
var setupServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Interactively configure and write a server config.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		listen := ":443"
		tlsMode := "auto"
		registry := "/var/lib/pmox/registry.json"
		logLevel := "info"

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().Title("Listen address").Value(&listen),
				huh.NewSelect[string]().Title("TLS mode").
					Options(huh.NewOptions("auto", "files", "http")...).Value(&tlsMode),
				huh.NewInput().Title("Registry file (licenses + hosts)").Value(&registry),
				huh.NewSelect[string]().Title("Log level").
					Options(huh.NewOptions("info", "debug", "warn", "error")...).Value(&logLevel),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}

		v := viper.New()
		v.Set("listen", listen)
		v.Set("log", logLevel)
		v.Set("registry_file", registry)
		v.Set("tls.mode", tlsMode)
		v.Set("tls.names", []string{"shop.proxmox.com"})
		v.Set("hosts.file", "/etc/hosts")
		if err := v.WriteConfigAs(setupOut); err != nil {
			return fmt.Errorf("write %s: %w", setupOut, err)
		}
		fmt.Printf("wrote %s\nstart the server with: proxmox-license-proxy serve --config %s\n", setupOut, setupOut)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.AddCommand(setupServerCmd)
	setupServerCmd.Flags().StringVar(&setupOut, "out", "config.yaml", "output config file")
}

package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"proxmox-license-proxy/internal/subscription"
)

// statusReport is the machine-readable form of `status`.
type statusReport struct {
	Config       string `json:"config" yaml:"config"`
	RegistryFile string `json:"registryFile" yaml:"registryFile"`
	Licenses     int    `json:"licenses" yaml:"licenses"`
	Hosts        int    `json:"hosts" yaml:"hosts"`
	Pending      int    `json:"pending" yaml:"pending"`
	Listen       string `json:"listen" yaml:"listen"`
	TLSMode      string `json:"tlsMode" yaml:"tlsMode"`
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Self-diagnostics: config, registry and host counts",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := store().Load()
		if err != nil {
			return err
		}

		pending := 0
		for _, srv := range reg.Servers {
			if srv.Status == subscription.Pending {
				pending++
			}
		}

		cfg := cfgUsed
		if cfg == "" {
			cfg = "(defaults + environment)"
		}
		report := statusReport{
			Config:       cfg,
			RegistryFile: settings.RegistryFile,
			Licenses:     len(reg.Licenses),
			Hosts:        len(reg.Servers),
			Pending:      pending,
			Listen:       settings.Listen,
			TLSMode:      string(settings.TLS.Mode),
		}

		return render(report, func() error {
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintf(tw, "config\t%s\n", report.Config)
			fmt.Fprintf(tw, "registry file\t%s\n", report.RegistryFile)
			fmt.Fprintf(tw, "licenses\t%d\n", report.Licenses)
			fmt.Fprintf(tw, "hosts\t%d (%d pending approval)\n", report.Hosts, report.Pending)
			fmt.Fprintf(tw, "listen\t%s\n", report.Listen)
			fmt.Fprintf(tw, "tls mode\t%s\n", report.TLSMode)
			return tw.Flush()
		})
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var setupOut string

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup wizards",
}

// serverWizardChoices holds everything the server wizard collects, so the form
// logic stays separate from writing the file.
type serverWizardChoices struct {
	listen   string
	logLevel string
	registry string

	tlsMode string
	tlsCert string
	tlsKey  string

	autoApprove        bool
	autoApprovePrivate bool
	autoApproveNets    string // comma/space separated CIDRs

	hostsFile string
}

// runServerWizard walks the admin through every server setting in small, guided
// steps: the TLS-files paths only appear in `files` mode, and the auto-approve
// detail questions only appear once it is enabled. Returns the collected choices.
func runServerWizard() (serverWizardChoices, error) {
	c := serverWizardChoices{
		listen:             ":443",
		logLevel:           "info",
		registry:           "/var/lib/pmox/registry.json",
		tlsMode:            "auto",
		autoApprovePrivate: true,
		hostsFile:          "/etc/hosts",
	}

	// Step 1: core listener + storage + TLS mode.
	core := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("proxmox-license-proxy - server setup").
				Description("Answer a few questions; this writes a ready-to-run config.yaml.\nDefaults are fine for most labs - just press enter."),
			huh.NewInput().Title("Listen address").
				Description("host:port the HTTPS server binds. :443 lets Proxmox reach it without a port.").
				Value(&c.listen),
			huh.NewSelect[string]().Title("TLS mode").
				Description("auto: self-signed cert, persisted across restarts (recommended).\nfiles: your own cert/key.  http: plaintext (testing only).").
				Options(huh.NewOptions("auto", "files", "http")...).Value(&c.tlsMode),
			huh.NewInput().Title("Registry file").
				Description("where subscriptions + host approvals are stored (JSON).").
				Value(&c.registry),
			huh.NewSelect[string]().Title("Log level").
				Options(huh.NewOptions("info", "debug", "warn", "error")...).Value(&c.logLevel),
		),
	)
	if err := core.Run(); err != nil {
		return c, err
	}

	// Step 2: cert/key paths, only when the admin chose `files`.
	if c.tlsMode == "files" {
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("TLS certificate path").Placeholder("/etc/pmox/tls.crt").Value(&c.tlsCert),
			huh.NewInput().Title("TLS key path").Placeholder("/etc/pmox/tls.key").Value(&c.tlsKey),
		)).Run(); err != nil {
			return c, err
		}
	}

	// Step 3: auto-approval. Ask the headline question first, then only drill in
	// when it is on, so the common "leave it off" path is a single keystroke.
	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Auto-approve hosts by source IP?").
			Description("Off: every new host stays PENDING until you run `server approve`.\nOn: hosts from trusted networks become active on first contact.").
			Value(&c.autoApprove),
	)).Run(); err != nil {
		return c, err
	}
	if c.autoApprove {
		if err := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().Title("Trust private networks?").
				Description("Auto-approve RFC1918 / ULA / loopback / link-local sources.").
				Value(&c.autoApprovePrivate),
			huh.NewInput().Title("Extra trusted networks (optional)").
				Description("Comma-separated CIDRs, e.g. 100.64.0.0/10, 10.8.0.0/24. Leave blank for none.").
				Value(&c.autoApproveNets),
		)).Run(); err != nil {
			return c, err
		}
	}

	return c, nil
}

// applyServerWizard turns the collected choices into a viper config and returns
// it ready to write.
func applyServerWizard(c serverWizardChoices) *viper.Viper {
	v := viper.New()
	v.Set("listen", c.listen)
	v.Set("log", c.logLevel)
	v.Set("registry_file", c.registry)

	v.Set("tls.mode", c.tlsMode)
	v.Set("tls.names", []string{"shop.proxmox.com"})
	if c.tlsMode == "files" {
		if c.tlsCert != "" {
			v.Set("tls.cert", c.tlsCert)
		}
		if c.tlsKey != "" {
			v.Set("tls.key", c.tlsKey)
		}
	}

	v.Set("hosts.file", c.hostsFile)

	v.Set("auto_approve.enabled", c.autoApprove)
	v.Set("auto_approve.private", c.autoApprovePrivate)
	if nets := parseNetworks(c.autoApproveNets); len(nets) > 0 {
		v.Set("auto_approve.networks", nets)
	}
	return v
}

// parseNetworks splits a free-form "a, b c" list into trimmed, non-empty CIDRs.
func parseNetworks(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// setup server: full guided wizard that writes a config.yaml.
// (Client setup lives in `client install`.)
var setupServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Interactively configure and write a server config.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		choices, err := runServerWizard()
		if err != nil {
			return err
		}
		v := applyServerWizard(choices)
		if err := v.WriteConfigAs(setupOut); err != nil {
			return fmt.Errorf("write %s: %w", setupOut, err)
		}
		fmt.Printf("\nwrote %s\n", setupOut)
		fmt.Printf("start the server with: proxmox-license-proxy serve --config %s\n", setupOut)
		if choices.autoApprove {
			fmt.Println("auto-approve is ON - hosts from trusted networks activate on first contact.")
		} else {
			fmt.Println("approve hosts after first contact with: proxmox-license-proxy server approve")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.AddCommand(setupServerCmd)
	setupServerCmd.Flags().StringVar(&setupOut, "out", "config.yaml", "output config file")
}

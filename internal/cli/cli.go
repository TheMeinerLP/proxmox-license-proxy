package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"proxmox-license-proxy/internal/config"
)

// cfgFile holds the --config override (empty = search default locations).
var cfgFile string

// settings is the validated, typed configuration. It is populated in
// PersistentPreRunE before any command runs, so every command can rely on it.
var settings *config.Settings

// cfgUsed is the config file that was actually read (empty when none).
var cfgUsed string

var rootCmd = &cobra.Command{
	Use:   "proxmox-license-proxy",
	Short: "Local Proxmox subscription manager and emulator for labs and homelabs",
	Long: "proxmox-license-proxy emulates the Proxmox subscription endpoint and\n" +
		"manages subscription keys. Intended for internal/private test environments only.",
	SilenceUsage: true,
	// Load configuration once, before any subcommand executes.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return loadConfig()
	},
}

// BuildInfo carries version metadata injected at build time.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// build holds the values passed to Execute, for the `version` command.
var build BuildInfo

// Execute is the entry point called from main.
func Execute(b BuildInfo) {
	build = b
	rootCmd.Version = b.Version
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Command groups shown as sections in --help.
const (
	groupServer = "server"
	groupClient = "client"
	groupSetup  = "setup"
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "",
		"config file (default: ./config.yaml or /etc/pmox/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "",
		"output format: table | json | yaml")
	_ = rootCmd.RegisterFlagCompletionFunc("output",
		cobra.FixedCompletions([]string{"table", "json", "yaml"}, cobra.ShellCompDirectiveNoFileComp))

	rootCmd.AddGroup(
		&cobra.Group{ID: groupServer, Title: "Server commands:"},
		&cobra.Group{ID: groupClient, Title: "Client commands:"},
		&cobra.Group{ID: groupSetup, Title: "Setup:"},
	)

	// Assign each top-level command to a section.
	serveCmd.GroupID = groupServer
	licenseCmd.GroupID = groupServer
	serverCmd.GroupID = groupServer
	configCmd.GroupID = groupServer
	statusCmd.GroupID = groupServer
	offlineCmd.GroupID = groupServer

	clientCmd.GroupID = groupClient
	certCmd.GroupID = groupClient
	hostsCmd.GroupID = groupClient

	setupCmd.GroupID = groupSetup
}

// loadConfig populates the package-level settings from defaults, the config
// file and the environment. It runs in PersistentPreRunE before any command.
func loadConfig() error {
	s, used, err := config.Load(cfgFile)
	if err != nil {
		return err
	}
	settings = s
	cfgUsed = used
	return nil
}

// notImplemented is the handler for commands whose subsystem is not built yet.
// It fails (non-zero exit) rather than printing success, so scripts and CI never
// mistake an unimplemented command for a completed action.
func notImplemented(cmd *cobra.Command, _ []string) error {
	return fmt.Errorf("%q is not implemented yet", cmd.CommandPath())
}

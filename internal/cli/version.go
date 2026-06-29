package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	Short:   "Print version and build info",
	GroupID: groupServer,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("proxmox-license-proxy %s\n  commit: %s\n  built:  %s\n  go:     %s\n",
			build.Version, build.Commit, build.Date, runtime.Version())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

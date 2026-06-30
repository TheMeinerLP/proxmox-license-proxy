package cli

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"proxmox-license-proxy/internal/release"
)

var versionCheck bool

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version and build info",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("proxmox-license-proxy %s\n  commit: %s\n  built:  %s\n  go:     %s\n",
			build.Version, build.Commit, build.Date, runtime.Version())
		if versionCheck {
			return printUpdateStatus(cmd.Context())
		}
		return nil
	},
}

// printUpdateStatus queries GitHub for the latest release and reports whether a
// newer one exists. A failed check is reported but not fatal.
func printUpdateStatus(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	info, err := release.NewChecker().Latest(ctx)
	if err != nil {
		return fmt.Errorf("update check failed: %w", err)
	}
	if release.IsNewer(build.Version, info.Tag) {
		fmt.Printf("\nupdate available: %s (you have %s)\n  %s\n", info.Tag, build.Version, info.URL)
	} else {
		fmt.Printf("\nyou are on the latest release (%s)\n", info.Tag)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolVar(&versionCheck, "check", false, "check GitHub for a newer release")
}

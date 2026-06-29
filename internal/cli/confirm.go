package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// interactiveTTY reports whether both stdin and stdout are connected to a
// terminal, i.e. whether it is safe to show an interactive prompt. When false
// (piped, redirected, run from a script or systemd) callers fall back to flags
// and defaults instead of blocking on input.
func interactiveTTY() bool {
	return isCharDevice(os.Stdin) && isCharDevice(os.Stdout)
}

func isCharDevice(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// confirm asks for a y/N confirmation on the command's input. It returns nil
// (proceed) when assumeYes is set or the user answers yes, and an error
// (abort, non-zero exit) otherwise. Destructive commands gate on this so a
// stray invocation cannot silently delete data.
func confirm(cmd *cobra.Command, assumeYes bool, prompt string) error {
	if assumeYes {
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s [y/N]: ", prompt)
	line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	default:
		return fmt.Errorf("aborted")
	}
}

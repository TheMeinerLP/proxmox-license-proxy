package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

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

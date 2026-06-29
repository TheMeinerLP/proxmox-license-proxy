package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// menuQuit is the sentinel value for the "quit" entry. The NUL prefix keeps it
// from ever colliding with a real subcommand name.
const menuQuit = "\x00quit"

// runInteractiveMenu turns a parent command (e.g. `client`, `subscription`)
// into a small guided shell: it lists the available subcommands, runs the one
// the user picks, then loops so several actions can be done in a row. Picking
// "quit" or aborting (Ctrl-C/Esc) leaves the menu. A subcommand that fails does
// not tear down the shell - its error is printed and the menu returns.
func runInteractiveMenu(cmd *cobra.Command) error {
	for {
		var subs []*cobra.Command
		opts := make([]huh.Option[string], 0, len(cmd.Commands())+1)
		for _, c := range cmd.Commands() {
			if !c.IsAvailableCommand() || c.Name() == "help" {
				continue
			}
			subs = append(subs, c)
			label := c.Name()
			if c.Short != "" {
				label += "  -  " + c.Short
			}
			opts = append(opts, huh.NewOption(label, c.Name()))
		}
		opts = append(opts, huh.NewOption("quit", menuQuit))

		var choice string
		if err := promptSelect(cmd.Name()+": what do you want to do?", &choice, opts...); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			return err
		}
		if choice == menuQuit || choice == "" {
			return nil
		}

		for _, c := range subs {
			if c.Name() == choice {
				if err := runMenuChoice(c); err != nil {
					fmt.Fprintln(os.Stderr, "error:", err)
				}
				break
			}
		}
		fmt.Println()
	}
}

// runMenuChoice invokes a chosen subcommand with default flags. The persistent
// pre-run (config load) already executed for the parent, and the management
// subcommands prompt for whatever they need when run without arguments, so this
// is enough to drive them interactively.
func runMenuChoice(c *cobra.Command) error {
	switch {
	case c.RunE != nil:
		return c.RunE(c, nil)
	case c.Run != nil:
		c.Run(c, nil)
	}
	return nil
}

// menuOrHelp is the RunE a parent command uses so a bare invocation on a
// terminal opens the guided menu, while piped/scripted use still prints help.
func menuOrHelp(cmd *cobra.Command, args []string) error {
	if len(args) == 0 && interactiveTTY() {
		return runInteractiveMenu(cmd)
	}
	return cmd.Help()
}

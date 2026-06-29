package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"proxmox-license-proxy/internal/subscription"
)

var accountCmd = &cobra.Command{
	Use:     "account",
	Short:   "Manage ACME client accounts (approve who may self-issue)",
	Aliases: []string{"accounts"},
	RunE:    menuOrHelp,
}

var accountListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered ACME accounts",
	RunE: func(cmd *cobra.Command, args []string) error {
		accounts, err := store().ListAccounts()
		if err != nil {
			return err
		}
		return render(accounts, func() error {
			if len(accounts) == 0 {
				fmt.Println("no accounts yet - a Proxmox host creates one with `client enroll`")
				return nil
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "THUMBPRINT\tSERVERID\tSTATUS\tCREATED")
			for _, a := range accounts {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", a.Thumbprint, a.ServerID, a.Status, a.CreatedAt)
			}
			return tw.Flush()
		})
	},
}

// selectAccount interactively picks an account whose status is not exclude.
func selectAccount(exclude subscription.Status, verb string) (string, error) {
	accounts, err := store().ListAccounts()
	if err != nil {
		return "", err
	}
	opts := make([]huh.Option[string], 0, len(accounts))
	for _, a := range accounts {
		if a.Status == exclude {
			continue
		}
		label := a.Thumbprint
		if a.ServerID != "" {
			label += "  " + a.ServerID
		}
		label += fmt.Sprintf("  [%s]", a.Status)
		opts = append(opts, huh.NewOption(label, a.Thumbprint))
	}
	if len(opts) == 0 {
		fmt.Printf("no accounts to %s\n", verb)
		return "", nil
	}
	var sel string
	err = promptSelect("Select an account to "+verb, &sel, opts...)
	return sel, err
}

// accountStatusCmd builds an approve/block command that takes a thumbprint or,
// with no argument on a terminal, opens an interactive picker.
func accountStatusCmd(use, short string, status subscription.Status) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var thumb string
			switch {
			case len(args) == 1:
				thumb = args[0]
			case interactiveTTY():
				picked, err := selectAccount(status, verbForStatus(status))
				if err != nil || picked == "" {
					return err
				}
				thumb = picked
			default:
				return fmt.Errorf("no account given (pass a thumbprint, or run on a terminal to pick)")
			}
			found, err := store().SetAccountStatus(thumb, status)
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("account %q not found", thumb)
			}
			fmt.Printf("account %s -> %s\n", thumb, status)
			return nil
		},
	}
}

func verbForStatus(s subscription.Status) string {
	if s == subscription.Blocked {
		return "block"
	}
	return "approve"
}

func init() {
	rootCmd.AddCommand(accountCmd)
	accountCmd.AddCommand(
		accountListCmd,
		accountStatusCmd("approve [thumbprint]", "Approve an account so it may self-issue subscriptions", subscription.Approved),
		accountStatusCmd("block [thumbprint]", "Block an account from issuing", subscription.Blocked),
	)
}

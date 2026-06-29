package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"proxmox-license-proxy/internal/subscription"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage registered Proxmox hosts (approvals)",
}

func printServers(servers []subscription.Server) error {
	return render(servers, func() error {
		if len(servers) == 0 {
			fmt.Println("no registered hosts")
			return nil
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "SERVERID\tKEY\tPRODUCT\tSTATUS\tLAST SEEN")
		for _, srv := range servers {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", srv.ServerID, srv.Key, srv.Product, srv.Status, srv.LastSeen)
		}
		return tw.Flush()
	})
}

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered hosts",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := store().ListServers()
		if err != nil {
			return err
		}
		return printServers(servers)
	},
}

var serverPendingCmd = &cobra.Command{
	Use:   "pending",
	Short: "List hosts awaiting approval",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := store().ListServers()
		if err != nil {
			return err
		}
		var pending []subscription.Server
		for _, srv := range servers {
			if srv.Status == subscription.Pending {
				pending = append(pending, srv)
			}
		}
		return printServers(pending)
	},
}

var (
	serverStatusAll  bool
	serverStatusNote string
)

// applyServerStatus sets status (and an optional note) on the given hosts. With
// all=true it targets every pending host instead of the argument list.
func applyServerStatus(status subscription.Status, ids []string, all bool, note string) error {
	st := store()

	if all {
		servers, err := st.ListServers()
		if err != nil {
			return err
		}
		ids = ids[:0]
		for _, srv := range servers {
			if srv.Status == subscription.Pending {
				ids = append(ids, srv.ServerID)
			}
		}
		if len(ids) == 0 {
			fmt.Println("no pending hosts")
			return nil
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("no hosts given (pass one or more server ids, or --all)")
	}

	for _, id := range ids {
		found, err := st.SetServerStatus(id, status)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("host %q not found", id)
		}
		if note != "" {
			if _, err := st.SetServerNote(id, note); err != nil {
				return err
			}
		}
		fmt.Printf("host %s -> %s\n", id, status)
	}
	return nil
}

// approvalCmd builds an approve/reject command sharing the multi-id, --all and
// --note behaviour.
func approvalCmd(use, short string, status subscription.Status) *cobra.Command {
	c := &cobra.Command{
		Use:               use,
		Short:             short,
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeServerIDs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return applyServerStatus(status, args, serverStatusAll, serverStatusNote)
		},
	}
	c.Flags().BoolVar(&serverStatusAll, "all", false, "apply to all pending hosts")
	c.Flags().StringVar(&serverStatusNote, "note", "", "attach a note to each host")
	return c
}

var serverBlockCmd = &cobra.Command{
	Use:               "block <serverid>...",
	Short:             "Block one or more hosts",
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: completeServerIDs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return applyServerStatus(subscription.Blocked, args, false, serverStatusNote)
	},
}

var serverRemoveYes bool

var serverRemoveCmd = &cobra.Command{
	Use:               "rm <serverid>",
	Aliases:           []string{"remove", "delete"},
	Short:             "Remove a host registration",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeServerIDs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := confirm(cmd, serverRemoveYes, fmt.Sprintf("Remove host %q?", args[0])); err != nil {
			return err
		}
		removed, err := store().RemoveServer(args[0])
		if err != nil {
			return err
		}
		if !removed {
			return fmt.Errorf("host %q not found", args[0])
		}
		fmt.Printf("removed host %s\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.AddCommand(
		serverListCmd,
		serverPendingCmd,
		approvalCmd("approve <serverid>...", "Approve hosts so they receive an active subscription", subscription.Approved),
		approvalCmd("reject <serverid>...", "Reject hosts (deny a subscription)", subscription.Rejected),
		serverBlockCmd,
		serverRemoveCmd,
	)
	serverBlockCmd.Flags().StringVar(&serverStatusNote, "note", "", "attach a note to each host")
	serverRemoveCmd.Flags().BoolVarP(&serverRemoveYes, "yes", "y", false, "skip confirmation prompt")
}

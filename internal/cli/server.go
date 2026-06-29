package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"proxmox-license-proxy/internal/subscription"
)

// selectHosts interactively picks hosts for an action, offering every host whose
// status is not already `exclude` (the status the action would set), each labelled
// with its status/product. Returns the chosen server ids, or nil if there is
// nothing to act on or the user picked nothing.
func selectHosts(exclude subscription.Status, verb string, multi bool) ([]string, error) {
	servers, err := store().ListServers()
	if err != nil {
		return nil, err
	}
	opts := make([]huh.Option[string], 0, len(servers))
	for _, srv := range servers {
		if srv.Status == exclude {
			continue
		}
		label := fmt.Sprintf("%s  [%s]", srv.ServerID, srv.Status)
		if srv.Product != "" {
			label += " " + srv.Product
		}
		opts = append(opts, huh.NewOption(label, srv.ServerID))
	}
	if len(opts) == 0 {
		fmt.Printf("no hosts to %s\n", verb)
		return nil, nil
	}

	var sel []string
	if multi {
		err = huh.NewForm(huh.NewGroup(huh.NewMultiSelect[string]().
			Title("Select hosts to " + verb).Options(opts...).Value(&sel))).Run()
	} else {
		var one string
		err = huh.NewForm(huh.NewGroup(huh.NewSelect[string]().
			Title("Select a host to " + verb).Options(opts...).Value(&one))).Run()
		if one != "" {
			sel = []string{one}
		}
	}
	return sel, err
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage registered Proxmox hosts (approvals)",
}

func printServers(servers []subscription.Server, emptyMsg string) error {
	return render(servers, func() error {
		if len(servers) == 0 {
			fmt.Println(emptyMsg)
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
		return printServers(servers,
			"no registered hosts yet\nhosts appear here after they first contact the proxy - run `client install` on a Proxmox host")
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
		return printServers(pending,
			"no hosts awaiting approval\n(hosts register as pending on first contact; auto_approve may approve them immediately)")
	},
}

var (
	serverStatusAll  bool
	serverStatusNote string
)

// applyServerStatus sets status (and an optional note) on the given hosts. With
// all=true it targets every pending host; with no ids on a terminal it opens an
// interactive picker instead of erroring, so admins rarely need to copy ids.
func applyServerStatus(status subscription.Status, verb string, ids []string, all bool, note string) error {
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
	if len(ids) == 0 && interactiveTTY() {
		picked, err := selectHosts(status, verb, true)
		if err != nil {
			return err
		}
		ids = picked
	}
	if len(ids) == 0 {
		return fmt.Errorf("no hosts given (pass server ids, run on a terminal to pick interactively, or --all)")
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
			verb := "approve"
			if status == subscription.Rejected {
				verb = "reject"
			}
			return applyServerStatus(status, verb, args, serverStatusAll, serverStatusNote)
		},
	}
	c.Flags().BoolVar(&serverStatusAll, "all", false, "apply to all pending hosts")
	c.Flags().StringVar(&serverStatusNote, "note", "", "attach a note to each host")
	return c
}

var serverBlockCmd = &cobra.Command{
	Use:               "block [serverid...]",
	Short:             "Block one or more hosts",
	Args:              cobra.ArbitraryArgs,
	ValidArgsFunction: completeServerIDs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return applyServerStatus(subscription.Blocked, "block", args, false, serverStatusNote)
	},
}

var serverRemoveYes bool

var serverRemoveCmd = &cobra.Command{
	Use:               "rm [serverid]",
	Aliases:           []string{"remove", "delete"},
	Short:             "Remove a host registration",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeServerIDs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var id string
		if len(args) == 1 {
			id = args[0]
		} else if interactiveTTY() {
			// the empty status matches no real host, so every host is offered.
			picked, err := selectHosts(subscription.Status(""), "remove", false)
			if err != nil {
				return err
			}
			if len(picked) == 0 {
				return nil
			}
			id = picked[0]
		} else {
			return fmt.Errorf("no host given (pass a server id, or run on a terminal to pick interactively)")
		}
		if err := confirm(cmd, serverRemoveYes, fmt.Sprintf("Remove host %q?", id)); err != nil {
			return err
		}
		removed, err := store().RemoveServer(id)
		if err != nil {
			return err
		}
		if !removed {
			return fmt.Errorf("host %q not found", id)
		}
		fmt.Printf("removed host %s\n", id)
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

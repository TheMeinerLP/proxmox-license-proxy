package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// ensureSettings loads the config on demand. Shell completion does not run the
// PersistentPreRunE hook, so completion functions must load settings themselves.
func ensureSettings() {
	if settings == nil {
		_ = loadConfig()
	}
}

// completeLicenseKeys completes an argument with the keys in the registry.
func completeLicenseKeys(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ensureSettings()
	if settings == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	licenses, err := store().ListLicenses()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out := make([]string, 0, len(licenses))
	for _, l := range licenses {
		// "key\tdescription" - zsh/fish show the description next to each key.
		desc := string(l.Status)
		if l.Product != "" {
			desc = l.Product + " " + desc
		}
		out = append(out, fmt.Sprintf("%s\t%s", l.Key, desc))
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeServerIDs completes an argument with the registered host ids, each
// annotated with its status (and product) so the right host is easy to pick.
func completeServerIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ensureSettings()
	if settings == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	servers, err := store().ListServers()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out := make([]string, 0, len(servers))
	for _, srv := range servers {
		desc := string(srv.Status)
		if srv.Product != "" {
			desc += " " + srv.Product
		}
		out = append(out, fmt.Sprintf("%s\t%s", srv.ServerID, desc))
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

package cli

import "github.com/spf13/cobra"

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
		out = append(out, l.Key)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeServerIDs completes an argument with the registered host ids.
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
		out = append(out, srv.ServerID)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

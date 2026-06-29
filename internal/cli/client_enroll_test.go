package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunClientEnrollRejectsSchemelessServer(t *testing.T) {
	resetEnrollGlobals()
	enrollServer = "192.168.68.100" // no scheme
	cmd := newEnrollFlagCmd()
	// Mark it as explicitly set so applyEnrollEnvDefaults leaves it alone.
	if err := cmd.Flags().Set("server", "192.168.68.100"); err != nil {
		t.Fatal(err)
	}

	err := runClientEnroll(cmd, nil)
	if err == nil {
		t.Fatal("expected an error for a scheme-less server URL")
	}
	if !strings.Contains(err.Error(), "http://") {
		t.Errorf("error %q should mention the required scheme", err)
	}
}

// newEnrollFlagCmd binds the enroll flags to the package globals exactly like
// init() does, so applyEnrollEnvDefaults can be exercised against a fresh command.
func newEnrollFlagCmd() *cobra.Command {
	c := &cobra.Command{Use: "enroll"}
	c.Flags().StringVar(&enrollServer, "server", "", "")
	c.Flags().StringVar(&enrollKeyPath, "account-key", "/etc/pmox/account.key", "")
	c.Flags().StringVar(&enrollServerID, "serverid", "", "")
	c.Flags().StringSliceVar(&enrollProducts, "products", nil, "")
	c.Flags().StringVar(&enrollLevel, "level", "", "")
	c.Flags().StringVar(&enrollSockets, "sockets", "", "")
	c.Flags().StringVar(&enrollContact, "contact", "", "")
	c.Flags().BoolVar(&enrollNoSet, "no-set", false, "")
	c.Flags().BoolVar(&enrollNoRedirect, "no-redirect", false, "")
	c.Flags().StringVar(&enrollIP, "ip", "", "")
	return c
}

func resetEnrollGlobals() {
	enrollServer, enrollKeyPath, enrollServerID = "", "/etc/pmox/account.key", ""
	enrollProducts = nil
	enrollLevel, enrollSockets, enrollContact, enrollIP = "", "", "", ""
	enrollNoSet, enrollNoRedirect = false, false
}

func TestApplyEnrollEnvDefaults(t *testing.T) {
	resetEnrollGlobals()
	t.Setenv("PMOX_ENROLL_SERVER", "https://proxy.lab")
	t.Setenv("PMOX_ENROLL_PRODUCTS", " pve , pbs ")
	t.Setenv("PMOX_ENROLL_LEVEL", "b")
	t.Setenv("PMOX_ENROLL_NO_SET", "yes")

	applyEnrollEnvDefaults(newEnrollFlagCmd())

	if enrollServer != "https://proxy.lab" {
		t.Errorf("server = %q, want https://proxy.lab", enrollServer)
	}
	if got := enrollProducts; len(got) != 2 || got[0] != "pve" || got[1] != "pbs" {
		t.Errorf("products = %v, want [pve pbs] (trimmed)", got)
	}
	if enrollLevel != "b" {
		t.Errorf("level = %q, want b", enrollLevel)
	}
	if !enrollNoSet {
		t.Errorf("no-set = false, want true (env 'yes')")
	}
	if enrollNoRedirect {
		t.Errorf("no-redirect = true, want false (env unset)")
	}
}

func TestApplyEnrollEnvDefaultsFlagWins(t *testing.T) {
	resetEnrollGlobals()
	t.Setenv("PMOX_ENROLL_SERVER", "https://env.lab")
	t.Setenv("PMOX_ENROLL_NO_SET", "1")

	cmd := newEnrollFlagCmd()
	// Simulate the user passing flags explicitly; these must beat the env.
	if err := cmd.Flags().Set("server", "https://flag.lab"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("no-set", "false"); err != nil {
		t.Fatal(err)
	}

	applyEnrollEnvDefaults(cmd)

	if enrollServer != "https://flag.lab" {
		t.Errorf("server = %q, want flag value https://flag.lab", enrollServer)
	}
	if enrollNoSet {
		t.Errorf("no-set = true, want false (explicit flag beats env)")
	}
}

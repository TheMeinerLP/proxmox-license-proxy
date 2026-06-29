package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// execRoot runs the real root command with the given args, capturing stdout. It
// drives the actual cobra wiring (persistent pre-run, flag parsing, RunE) the
// same way the binary does, so these are genuine end-to-end CLI tests.
func execRoot(t *testing.T, args ...string) string {
	t.Helper()
	rootCmd.SetArgs(args)
	var err error
	out := captureStdout(t, func() { err = rootCmd.Execute() })
	if err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
	return out
}

// useTempRegistry points the CLI at a throwaway registry file via the same env
// var users would set, so loadConfig (the persistent pre-run) wires it up.
func useTempRegistry(t *testing.T) string {
	t.Helper()
	reg := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv("PMOX_REGISTRY_FILE", reg)
	return reg
}

var keyRe = regexp.MustCompile(`pbsc-[0-9a-f]{10}`)

func TestSubscriptionLifecycle(t *testing.T) {
	useTempRegistry(t)

	// Mint a lab key non-interactively (flags set, --yes skips confirmation).
	gen := execRoot(t, "subscription", "generate", "--product", "pbs", "--level", "c", "--yes")
	key := keyRe.FindString(gen)
	if key == "" {
		t.Fatalf("generate did not print a pbs key:\n%s", gen)
	}
	if !strings.Contains(gen, "1ab") {
		t.Errorf("generated key is missing the lab signature:\n%s", gen)
	}

	// It should now show up in the list...
	if list := execRoot(t, "subscription", "list"); !strings.Contains(list, key) {
		t.Errorf("list does not contain %s:\n%s", key, list)
	}

	// ...and be removable, after which the list reports the empty state.
	if rm := execRoot(t, "subscription", "rm", key, "--yes"); !strings.Contains(rm, "removed "+key) {
		t.Errorf("rm did not confirm removal:\n%s", rm)
	}
	if list := execRoot(t, "subscription", "list"); !strings.Contains(list, "no subscriptions") {
		t.Errorf("empty list should hint at `subscription generate`:\n%s", list)
	}
}

func TestStatusEmptyStateHint(t *testing.T) {
	useTempRegistry(t)
	out := execRoot(t, "status")
	if !strings.Contains(out, "next: mint a lab key") {
		t.Errorf("empty status should suggest generating a key:\n%s", out)
	}
}

func TestServerEmptyStateHint(t *testing.T) {
	useTempRegistry(t)
	out := execRoot(t, "server", "list")
	if !strings.Contains(out, "no registered hosts") {
		t.Errorf("server list empty state missing:\n%s", out)
	}
}

func TestConfigInitDefaults(t *testing.T) {
	out := filepath.Join(t.TempDir(), "config.yaml")
	execRoot(t, "config", "init", "--defaults", "--out", out)

	data := readFile(t, out)
	for _, want := range []string{"listen:", "tls:", "auto_approve:", "registry_file:"} {
		if !strings.Contains(data, want) {
			t.Errorf("scaffolded config missing %q:\n%s", want, data)
		}
	}
}

func TestVersionCommand(t *testing.T) {
	build = BuildInfo{Version: "9.9.9", Commit: "abc", Date: "today"}
	out := execRoot(t, "version")
	if !strings.Contains(out, "9.9.9") {
		t.Errorf("version output missing version: %s", out)
	}
}

package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRunMenuChoiceDispatchesRunE(t *testing.T) {
	called := false
	c := &cobra.Command{Use: "x", RunE: func(*cobra.Command, []string) error { called = true; return nil }}
	if err := runMenuChoice(c); err != nil {
		t.Fatalf("runMenuChoice: %v", err)
	}
	if !called {
		t.Error("RunE was not invoked")
	}
}

func TestRunMenuChoiceDispatchesRun(t *testing.T) {
	called := false
	c := &cobra.Command{Use: "x", Run: func(*cobra.Command, []string) { called = true }}
	if err := runMenuChoice(c); err != nil {
		t.Fatalf("runMenuChoice: %v", err)
	}
	if !called {
		t.Error("Run was not invoked")
	}
}

// In tests stdout is a pipe, so interactiveTTY() is false and menuOrHelp must
// fall back to help (returning nil) instead of trying to open a menu - i.e. it
// never blocks on input in a non-interactive context.
func TestMenuOrHelpNonInteractive(t *testing.T) {
	c := &cobra.Command{Use: "parent"}
	c.AddCommand(&cobra.Command{Use: "child", Short: "do a thing", RunE: func(*cobra.Command, []string) error { return nil }})
	if err := menuOrHelp(c, nil); err != nil {
		t.Fatalf("menuOrHelp non-interactive: %v", err)
	}
}

package cli

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestConfirm(t *testing.T) {
	tests := []struct {
		name      string
		assumeYes bool
		input     string
		wantErr   bool
	}{
		{"assume yes skips prompt", true, "", false},
		{"y proceeds", false, "y\n", false},
		{"yes proceeds", false, "yes\n", false},
		{"n aborts", false, "n\n", true},
		{"empty aborts", false, "\n", true},
		{"garbage aborts", false, "maybe\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.SetIn(strings.NewReader(tt.input))
			cmd.SetOut(io.Discard)
			err := confirm(cmd, tt.assumeYes, "Proceed?")
			if (err != nil) != tt.wantErr {
				t.Fatalf("confirm() err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

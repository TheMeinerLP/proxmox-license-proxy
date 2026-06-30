package cli

import (
	"regexp"
	"testing"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func TestFormatTableAlignsByPlainWidth(t *testing.T) {
	headers := []string{"KEY", "STATUS"}
	rows := [][]string{
		{"pve2c-1ab6b5b38d", "APPROVED"},
		{"pbsc-1ab35958ba", "REVOKED"},
	}
	// Color the status column so the output carries ANSI codes; alignment must
	// still be computed from the plain text.
	color := func(col int, val string) string {
		if col == 1 {
			return colorStatus(val)
		}
		return val
	}

	orig := useColor
	t.Cleanup(func() { useColor = orig })

	useColor = false
	plain := formatTable(headers, rows, color)

	useColor = true
	colored := formatTable(headers, rows, color)

	if !ansiRE.MatchString(colored) {
		t.Fatal("expected ANSI color codes in the colored output")
	}
	// The core invariant: coloring must not shift the layout. Stripping the ANSI
	// codes from the colored render must reproduce the plain render exactly.
	if stripANSI(colored) != plain {
		t.Errorf("color changed the layout:\nplain:\n%s\nstripped:\n%s", plain, stripANSI(colored))
	}
}

func TestIndentLines(t *testing.T) {
	in := "first\nsecond\nthird"
	want := "  first\n  second\n  third"
	if got := indentLines(in, "  "); got != want {
		t.Errorf("indentLines = %q, want %q", got, want)
	}
}

func TestColorStatusDisabled(t *testing.T) {
	orig := useColor
	t.Cleanup(func() { useColor = orig })
	useColor = false
	if got := colorStatus("APPROVED"); got != "APPROVED" {
		t.Errorf("with color off, colorStatus should be a no-op, got %q", got)
	}
}

package cli

import (
	"os"
	"strings"
)

// useColor reports whether human-facing output should be colorized. It is off
// when NO_COLOR is set, TERM is "dumb", or stdout is not a terminal (piped,
// redirected, systemd), so scripts and logs stay plain.
var useColor = colorEnabled()

func colorEnabled() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return isCharDevice(os.Stdout)
}

// SGR parameters for the 8 basic ANSI colors and attributes. Basic codes (not
// hex) are used deliberately so the palette follows the user's terminal theme
// and renders on any 16-color terminal.
const (
	sgrReset  = "\x1b[0m"
	sgrBold   = "1"
	sgrFaint  = "2"
	sgrRed    = "31"
	sgrGreen  = "32"
	sgrYellow = "33"
	sgrCyan   = "36"
)

// paint wraps text in the SGR code(s) when color is enabled, else returns it
// verbatim - so the same call sites work plain when piped or NO_COLOR is set.
func paint(code, text string) string {
	if !useColor {
		return text
	}
	return "\x1b[" + code + "m" + text + sgrReset
}

func colorOK(s string) string     { return paint(sgrGreen, s) }
func colorWarn(s string) string   { return paint(sgrYellow, s) }
func colorFail(s string) string   { return paint(sgrRed, s) }
func colorInfo(s string) string   { return paint(sgrCyan, s) }
func colorHeader(s string) string { return paint(sgrBold+";"+sgrCyan, s) }
func colorBold(s string) string   { return paint(sgrBold, s) }
func colorDim(s string) string    { return paint(sgrFaint, s) }

// indentLines prefixes every line of text with prefix, so a multi-line detail
// stays aligned under its heading instead of only the first line being indented.
func indentLines(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

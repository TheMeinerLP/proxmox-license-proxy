package cli

import (
	"fmt"
	"os"
	"strings"

	"proxmox-license-proxy/internal/subscription"
)

// printTable prints aligned columns with a colored header and optional per-cell
// coloring. Column widths are measured from the plain text, then color is
// applied - so ANSI escape codes never throw off the alignment the way they do
// inside text/tabwriter (which counts escape bytes as visible width).
//
// cell, when non-nil, returns the display string for a body cell given its
// column index and raw value (e.g. to color a status). It must not change the
// visible width.
func printTable(headers []string, rows [][]string, cell func(col int, val string) string) {
	fmt.Fprint(os.Stdout, formatTable(headers, rows, cell))
}

// formatTable builds the aligned, colored table as a string (testable; printTable
// writes it to stdout).
func formatTable(headers []string, rows [][]string, cell func(col int, val string) string) string {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if i < len(widths) && len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}

	var b strings.Builder
	writeRow := func(cols []string, color func(col int, val string) string) {
		for i, c := range cols {
			disp := c
			if color != nil {
				disp = color(i, c)
			}
			b.WriteString(disp)
			if i < len(cols)-1 { // pad (by plain width) + gap between columns
				if i < len(widths) {
					b.WriteString(strings.Repeat(" ", widths[i]-len(c)))
				}
				b.WriteString("  ")
			}
		}
		b.WriteByte('\n')
	}

	writeRow(headers, func(_ int, v string) string { return colorHeader(v) })
	for _, r := range rows {
		writeRow(r, cell)
	}
	return b.String()
}

// colorStatus colors a subscription/host/account status so state reads at a
// glance: active/approved green, pending/info cyan, revoked/blocked/failed red.
func colorStatus(status string) string {
	switch subscription.Status(strings.ToUpper(status)) {
	case subscription.Approved:
		return colorOK(status)
	case subscription.Pending, subscription.Registered:
		return colorWarn(status)
	case subscription.Revoked, subscription.Blocked, subscription.Rejected, subscription.Failed:
		return colorFail(status)
	default:
		return status
	}
}

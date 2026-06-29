// Package fileio centralizes file reads whose path comes from this tool's own
// configuration or CLI flags. Routing them through one cleaned, audited helper
// keeps the single gosec G304 ("file inclusion via variable") decision in one
// place - instead of a blanket linter exclude - and guarantees every such path
// is filepath.Clean'd before it is opened.
package fileio

import (
	"os"
	"path/filepath"
)

// ReadFile reads the named file after cleaning its path. The path is expected to
// originate from trusted operator input (config file, CLI flag), not from a
// remote/untrusted source; Clean strips any "../" noise before the read.
func ReadFile(path string) ([]byte, error) {
	//nolint:gosec // G304: path is trusted operator config/flag input, cleaned above
	return os.ReadFile(filepath.Clean(path))
}

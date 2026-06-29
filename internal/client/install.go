// Package client holds the client-side install/update logic, kept free of any
// CLI or UI dependency so it stays easy to test and reuse.
package client

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Action describes what InstallBinary did.
type Action string

const (
	ActionInstalled Action = "installed"
	ActionUpdated   Action = "updated"
	ActionUnchanged Action = "unchanged"
)

// Result is the outcome of an install.
type Result struct {
	Path   string
	Action Action
}

// CurrentBinary returns the absolute, symlink-resolved path of the running
// executable.
func CurrentBinary() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

// InstallBinary copies src to dest atomically (temp file + rename, mode 0755).
// It reports whether the destination was created, updated, or already current.
func InstallBinary(src, dest string) (Result, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return Result{}, fmt.Errorf("read source binary: %w", err)
	}

	action := ActionInstalled
	if existing, err := os.ReadFile(dest); err == nil {
		if bytes.Equal(existing, data) {
			return Result{Path: dest, Action: ActionUnchanged}, nil
		}
		action = ActionUpdated
	}

	if dir := filepath.Dir(dest); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Result{}, err
		}
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return Result{}, fmt.Errorf("write %s (need root?): %w", dest, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return Result{}, err
	}
	return Result{Path: dest, Action: action}, nil
}

// UninstallBinary removes the installed binary. The bool reports whether it
// existed (a running executable can remove its own file on Linux).
func UninstallBinary(dest string) (bool, error) {
	if err := os.Remove(dest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("remove %s (need root?): %w", dest, err)
	}
	return true, nil
}

// DownloadTo fetches url into a temp file and returns its path. TLS is verified
// (this downloads an executable, so trust matters).
func DownloadTo(url string) (string, error) {
	c := &http.Client{Timeout: 60 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}

	f, err := os.CreateTemp("", "plp-binary-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	return f.Name(), nil
}

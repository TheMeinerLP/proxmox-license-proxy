package client

import (
	"fmt"
	"os"
	"os/exec"
)

// Product is a Proxmox product detected on the host, together with the command
// that sets its subscription key. A single host may run several (e.g. PVE + PBS).
type Product struct {
	Code       string   // pve | pbs | pmg
	Name       string   // human label
	SetCommand []string // argv to set a key; the key is appended as the last arg
}

// knownProducts lists every supported Proxmox product, how to detect it (its
// CLI tool on PATH or its config directory) and how to set a subscription key.
var knownProducts = []struct {
	Code, Name string
	Bins, Dirs []string
	Set        []string
}{
	{"pve", "Proxmox VE", []string{"pvesubscription"}, []string{"/etc/pve"}, []string{"pvesubscription", "set"}},
	{"pbs", "Proxmox Backup Server", []string{"proxmox-backup-manager"}, []string{"/etc/proxmox-backup"}, []string{"proxmox-backup-manager", "subscription", "set"}},
	{"pmg", "Proxmox Mail Gateway", []string{"pmgsubscription"}, []string{"/etc/pmg"}, []string{"pmgsubscription", "set"}},
}

// DetectProducts returns the Proxmox products installed on this host, found via
// their management tool on PATH or their /etc config directory.
func DetectProducts() []Product {
	return detectProducts(exec.LookPath, dirExists)
}

// detectProducts is the injectable core, so detection is unit-testable without a
// real Proxmox install.
func detectProducts(look func(string) (string, error), dir func(string) bool) []Product {
	var out []Product
	for _, p := range knownProducts {
		if anyBin(p.Bins, look) || anyDir(p.Dirs, dir) {
			out = append(out, Product{Code: p.Code, Name: p.Name, SetCommand: p.Set})
		}
	}
	return out
}

func anyBin(bins []string, look func(string) (string, error)) bool {
	for _, b := range bins {
		if _, err := look(b); err == nil {
			return true
		}
	}
	return false
}

func anyDir(dirs []string, exists func(string) bool) bool {
	for _, d := range dirs {
		if exists(d) {
			return true
		}
	}
	return false
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// SetKey installs a subscription key for the product by running its management
// tool (e.g. `proxmox-backup-manager subscription set <key>`). Requires root.
func (p Product) SetKey(key string) error {
	if len(p.SetCommand) == 0 {
		return fmt.Errorf("no set command for %s", p.Code)
	}
	args := append(append([]string{}, p.SetCommand[1:]...), key)
	//nolint:gosec // G204: the binary comes from the fixed knownProducts table and
	// the key is a format-validated lab key, not arbitrary user input.
	out, err := exec.Command(p.SetCommand[0], args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s set key: %w: %s", p.Code, err, string(out))
	}
	return nil
}

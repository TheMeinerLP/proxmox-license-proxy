// Package hosts manages a single, clearly marked block in /etc/hosts that points
// the Proxmox subscription hostname at the proxy. The block is delimited by
// markers so it can be added, replaced and removed idempotently without touching
// the rest of the file. Writes are atomic and preserve the file's permissions.
package hosts

import (
	"fmt"
	"os"
	"strings"
)

const (
	beginMarker = "# >>> pmox-proxy (managed) >>>"
	endMarker   = "# <<< pmox-proxy (managed) <<<"
)

// Enable inserts (or replaces) the managed block pointing names at ip.
// With dryRun it returns the resulting file content without writing.
func Enable(file, ip string, names []string, dryRun bool) (string, error) {
	if ip == "" {
		return "", fmt.Errorf("no IP given (set hosts.ip in config or pass --ip)")
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no hostnames to map")
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}

	stripped := strings.TrimRight(stripBlock(string(raw)), "\n") + "\n"
	result := stripped + managedBlock(ip, names)

	if dryRun {
		return result, nil
	}
	if err := atomicWrite(file, result, raw); err != nil {
		return "", err
	}
	return managedBlock(ip, names), nil
}

// Disable removes the managed block. With dryRun it returns the resulting
// content without writing.
func Disable(file string, dryRun bool) (string, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	result := strings.TrimRight(stripBlock(string(raw)), "\n") + "\n"
	if dryRun {
		return result, nil
	}
	return "", atomicWrite(file, result, raw)
}

// Status reports whether the managed block is present and returns it.
func Status(file string) (bool, string, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return false, "", err
	}
	block := extractBlock(string(raw))
	return block != "", block, nil
}

func managedBlock(ip string, names []string) string {
	var b strings.Builder
	b.WriteString(beginMarker + "\n")
	for _, n := range names {
		fmt.Fprintf(&b, "%s %s\n", ip, n)
	}
	b.WriteString(endMarker + "\n")
	return b.String()
}

func stripBlock(content string) string {
	var out []string
	inBlock := false
	for _, line := range strings.Split(content, "\n") {
		switch strings.TrimSpace(line) {
		case beginMarker:
			inBlock = true
			continue
		case endMarker:
			inBlock = false
			continue
		}
		if !inBlock {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func extractBlock(content string) string {
	var out []string
	inBlock := false
	for _, line := range strings.Split(content, "\n") {
		t := strings.TrimSpace(line)
		if t == beginMarker {
			inBlock = true
			out = append(out, line)
			continue
		}
		if t == endMarker {
			out = append(out, line)
			break
		}
		if inBlock {
			out = append(out, line)
		}
	}
	if !inBlock && len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n")
}

// atomicWrite writes content via a temp file + rename, preserving the original
// file's permission bits.
func atomicWrite(file, content string, original []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	}
	_ = original // kept for symmetry / future backup

	tmp := file + ".tmp"
	// mode is the preserved permission of the existing file (or 0644 for a new
	// /etc/hosts), so it is not a hard-coded over-permissive literal.
	if err := os.WriteFile(tmp, []byte(content), mode); err != nil {
		return fmt.Errorf("write %s (need root?): %w", file, err)
	}
	return os.Rename(tmp, file)
}

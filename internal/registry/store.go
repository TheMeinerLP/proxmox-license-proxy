// Package registry is the persistence adapter for the registry.json file
// (licenses + auto-registered hosts). The domain types live in the
// subscription package; this package only knows how to read and write them.
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"

	"proxmox-license-proxy/internal/subscription"
)

// Store is the on-disk registry. It is the single source of truth shared by the
// server (read-through on every request) and the CLI (mutations). Cross-process
// safety is provided by an advisory file lock; writes are atomic (temp file +
// rename), so readers never see a partial file and therefore need no lock.
type Store struct {
	path string
}

// NewStore returns a store backed by the given file path.
func NewStore(path string) *Store { return &Store{path: path} }

// Load reads the registry. A missing file yields an empty registry.
func (s *Store) Load() (subscription.Registry, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return subscription.Registry{}, nil
	}
	if err != nil {
		return subscription.Registry{}, err
	}
	var reg subscription.Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return subscription.Registry{}, fmt.Errorf("registry %q: %w", s.path, err)
	}
	return reg, nil
}

// mutate takes the exclusive lock, loads the registry, applies fn and writes
// the result atomically.
func (s *Store) mutate(fn func(*subscription.Registry) error) error {
	if dir := filepath.Dir(s.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	lock := flock.New(s.path + ".lock")
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("lock registry: %w", err)
	}
	defer func() { _ = lock.Unlock() }()

	reg, err := s.Load()
	if err != nil {
		return err
	}
	if err := fn(&reg); err != nil {
		return err
	}
	return s.save(reg)
}

func (s *Store) save(reg subscription.Registry) error {
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	// Keep the last good copy so a bad edit can be recovered. Best-effort: a
	// missing file (first write) or backup failure must not block the write.
	if prev, rerr := os.ReadFile(s.path); rerr == nil {
		_ = os.WriteFile(s.path+".bak", prev, 0o640)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// ListLicenses returns all licenses.
func (s *Store) ListLicenses() ([]subscription.License, error) {
	reg, err := s.Load()
	if err != nil {
		return nil, err
	}
	return reg.Licenses, nil
}

// GetLicense looks up a license by key.
func (s *Store) GetLicense(key string) (subscription.License, bool, error) {
	reg, err := s.Load()
	if err != nil {
		return subscription.License{}, false, err
	}
	for _, l := range reg.Licenses {
		if l.Key == key {
			return l, true, nil
		}
	}
	return subscription.License{}, false, nil
}

// AddLicense inserts or replaces a license.
func (s *Store) AddLicense(l subscription.License) error {
	return s.mutate(func(reg *subscription.Registry) error {
		for i := range reg.Licenses {
			if reg.Licenses[i].Key == l.Key {
				reg.Licenses[i] = l
				return nil
			}
		}
		reg.Licenses = append(reg.Licenses, l)
		return nil
	})
}

// RemoveLicense deletes a license. The bool reports whether it existed.
func (s *Store) RemoveLicense(key string) (bool, error) {
	removed := false
	err := s.mutate(func(reg *subscription.Registry) error {
		out := reg.Licenses[:0]
		for _, l := range reg.Licenses {
			if l.Key == key {
				removed = true
				continue
			}
			out = append(out, l)
		}
		reg.Licenses = out
		return nil
	})
	return removed, err
}

// SetDue changes a license's expiry date. The bool reports whether it existed.
func (s *Store) SetDue(key, due string) (bool, error) {
	found := false
	err := s.mutate(func(reg *subscription.Registry) error {
		for i := range reg.Licenses {
			if reg.Licenses[i].Key == key {
				reg.Licenses[i].NextDueDate = due
				found = true
			}
		}
		return nil
	})
	return found, err
}

// UpsertServer records a host contact: new hosts are created as PENDING, known
// hosts get their LastSeen (and key/product) refreshed. It returns the current
// entry so the caller can read the host's status.
func (s *Store) UpsertServer(serverid, key, product string) (subscription.Server, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var current subscription.Server
	err := s.mutate(func(reg *subscription.Registry) error {
		for i := range reg.Servers {
			if reg.Servers[i].ServerID == serverid {
				reg.Servers[i].LastSeen = now
				if key != "" {
					reg.Servers[i].Key = key
				}
				if product != "" {
					reg.Servers[i].Product = product
				}
				current = reg.Servers[i]
				return nil
			}
		}
		current = subscription.Server{
			ServerID:  serverid,
			Key:       key,
			Product:   product,
			Status:    subscription.Pending,
			FirstSeen: now,
			LastSeen:  now,
		}
		reg.Servers = append(reg.Servers, current)
		return nil
	})
	return current, err
}

// ListServers returns all registered hosts.
func (s *Store) ListServers() ([]subscription.Server, error) {
	reg, err := s.Load()
	if err != nil {
		return nil, err
	}
	return reg.Servers, nil
}

// SetServerStatus changes a host's status. The bool reports whether it existed.
func (s *Store) SetServerStatus(serverid string, status subscription.Status) (bool, error) {
	found := false
	err := s.mutate(func(reg *subscription.Registry) error {
		for i := range reg.Servers {
			if reg.Servers[i].ServerID == serverid {
				reg.Servers[i].Status = status
				found = true
			}
		}
		return nil
	})
	return found, err
}

// SetServerNote sets a free-text note on a host. The bool reports whether it existed.
func (s *Store) SetServerNote(serverid, note string) (bool, error) {
	found := false
	err := s.mutate(func(reg *subscription.Registry) error {
		for i := range reg.Servers {
			if reg.Servers[i].ServerID == serverid {
				reg.Servers[i].Note = note
				found = true
			}
		}
		return nil
	})
	return found, err
}

// RemoveServer deletes a host registration. The bool reports whether it existed.
func (s *Store) RemoveServer(serverid string) (bool, error) {
	removed := false
	err := s.mutate(func(reg *subscription.Registry) error {
		out := reg.Servers[:0]
		for _, srv := range reg.Servers {
			if srv.ServerID == serverid {
				removed = true
				continue
			}
			out = append(out, srv)
		}
		reg.Servers = out
		return nil
	})
	return removed, err
}

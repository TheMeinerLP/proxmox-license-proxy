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

	"proxmox-license-proxy/internal/fileio"
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
	data, err := fileio.ReadFile(s.path)
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
		// 0750: the registry holds host/license state; no world access. The
		// package ships /etc/pmox as a setgid group dir, which this preserves.
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}

	lock := flock.New(s.path + ".lock")
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("lock registry: %w", err)
	}
	defer func() { _ = lock.Unlock() }()
	// The lock file may be created by either a root-run CLI or the pmox service;
	// keep it group-writable so both can take the lock on a shared registry.
	//nolint:gosec // G302: 0660 is required so the pmox group (daemon + root CLI) shares the lock
	_ = os.Chmod(s.path+".lock", 0o660)

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
	if prev, rerr := fileio.ReadFile(s.path); rerr == nil {
		//nolint:gosec // G306: 0660 so the pmox group (daemon + root CLI) shares the registry backup
		_ = os.WriteFile(s.path+".bak", prev, 0o660)
		//nolint:gosec // G302: keep the backup group-writable for the shared registry
		_ = os.Chmod(s.path+".bak", 0o660)
	}
	tmp := s.path + ".tmp"
	//nolint:gosec // G306: 0660 so the pmox group (daemon + root CLI) shares the registry
	if err := os.WriteFile(tmp, data, 0o660); err != nil {
		return err
	}
	// Pin 0660 regardless of umask so a registry written by root (CLI) stays
	// writable by the pmox service group, and vice versa, on a shared dir.
	//nolint:gosec // G302: see comment above - shared group write is required
	if err := os.Chmod(tmp, 0o660); err != nil {
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

// SetLicenseStatus changes a subscription's status (e.g. to REVOKED). The bool
// reports whether the key existed.
func (s *Store) SetLicenseStatus(key string, status subscription.Status) (bool, error) {
	found := false
	err := s.mutate(func(reg *subscription.Registry) error {
		for i := range reg.Licenses {
			if reg.Licenses[i].Key == key {
				reg.Licenses[i].Status = status
				found = true
			}
		}
		return nil
	})
	return found, err
}

// GetLicenseFor returns the subscription assigned to a host for a product, if any.
func (s *Store) GetLicenseFor(serverid, product string) (subscription.License, bool, error) {
	reg, err := s.Load()
	if err != nil {
		return subscription.License{}, false, err
	}
	for _, l := range reg.Licenses {
		if l.ServerID == serverid && l.Product == product {
			return l, true, nil
		}
	}
	return subscription.License{}, false, nil
}

// UpsertAccount inserts or updates an ACME account, keyed by its JWK thumbprint.
func (s *Store) UpsertAccount(acc subscription.Account) error {
	return s.mutate(func(reg *subscription.Registry) error {
		for i := range reg.Accounts {
			if reg.Accounts[i].Thumbprint == acc.Thumbprint {
				reg.Accounts[i] = acc
				return nil
			}
		}
		reg.Accounts = append(reg.Accounts, acc)
		return nil
	})
}

// GetAccount looks up an account by its thumbprint (the JWS "kid").
func (s *Store) GetAccount(thumbprint string) (subscription.Account, bool, error) {
	reg, err := s.Load()
	if err != nil {
		return subscription.Account{}, false, err
	}
	for _, a := range reg.Accounts {
		if a.Thumbprint == thumbprint {
			return a, true, nil
		}
	}
	return subscription.Account{}, false, nil
}

// ListAccounts returns all registered ACME accounts.
func (s *Store) ListAccounts() ([]subscription.Account, error) {
	reg, err := s.Load()
	if err != nil {
		return nil, err
	}
	return reg.Accounts, nil
}

// SetAccountStatus changes an account's status. The bool reports whether it existed.
func (s *Store) SetAccountStatus(thumbprint string, status subscription.Status) (bool, error) {
	found := false
	err := s.mutate(func(reg *subscription.Registry) error {
		for i := range reg.Accounts {
			if reg.Accounts[i].Thumbprint == thumbprint {
				reg.Accounts[i].Status = status
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
//
// When autoApprove is set (the host contacted from a trusted network), a new
// host starts APPROVED and a host still PENDING is upgraded to APPROVED. An
// explicit BLOCKED/REJECTED/APPROVED status is never changed - an operator's
// decision always wins over auto-approval.
func (s *Store) UpsertServer(serverid, key, product string, autoApprove bool) (subscription.Server, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	initial := subscription.Pending
	if autoApprove {
		initial = subscription.Approved
	}
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
				if autoApprove && reg.Servers[i].Status == subscription.Pending {
					reg.Servers[i].Status = subscription.Approved
				}
				current = reg.Servers[i]
				return nil
			}
		}
		current = subscription.Server{
			ServerID:  serverid,
			Key:       key,
			Product:   product,
			Status:    initial,
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

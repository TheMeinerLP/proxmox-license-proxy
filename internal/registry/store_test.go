package registry

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"proxmox-license-proxy/internal/subscription"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "registry.json"))
}

func TestStoreLoadMissingFileIsEmpty(t *testing.T) {
	reg, err := newStore(t).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(reg.Licenses) != 0 || len(reg.Servers) != 0 {
		t.Fatalf("expected empty registry, got %+v", reg)
	}
}

func TestStoreLicenseCRUD(t *testing.T) {
	s := newStore(t)
	for _, k := range []string{"pbsc-1111111111", "pbsc-2222222222", "pbsc-3333333333"} {
		if err := s.AddLicense(subscription.License{Key: k, Status: subscription.Approved}); err != nil {
			t.Fatalf("AddLicense: %v", err)
		}
	}
	// remove the middle one -> the other two must survive intact
	removed, err := s.RemoveLicense("pbsc-2222222222")
	if err != nil || !removed {
		t.Fatalf("RemoveLicense = (%v,%v)", removed, err)
	}
	list, _ := s.ListLicenses()
	if len(list) != 2 {
		t.Fatalf("expected 2 licenses, got %d: %+v", len(list), list)
	}
	if _, ok, _ := s.GetLicense("pbsc-1111111111"); !ok {
		t.Error("first license lost")
	}
	if _, ok, _ := s.GetLicense("pbsc-3333333333"); !ok {
		t.Error("third license lost")
	}

	if found, _ := s.SetDue("pbsc-1111111111", "2031-01-01"); !found {
		t.Error("SetDue not found")
	}
	l, _, _ := s.GetLicense("pbsc-1111111111")
	if l.NextDueDate != "2031-01-01" {
		t.Errorf("SetDue not applied: %s", l.NextDueDate)
	}
}

func TestStoreUpsertServer(t *testing.T) {
	s := newStore(t)
	// first contact -> PENDING, timestamps set
	srv, err := s.UpsertServer("HW-1", "pbsc-1111111111", "pbs")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}
	if srv.Status != subscription.Pending || srv.FirstSeen == "" || srv.LastSeen == "" {
		t.Fatalf("first contact wrong: %+v", srv)
	}

	// approve, then re-contact -> status preserved, LastSeen refreshed
	if found, _ := s.SetServerStatus("HW-1", subscription.Approved); !found {
		t.Fatal("SetServerStatus not found")
	}
	again, _ := s.UpsertServer("HW-1", "pbsc-1111111111", "pbs")
	if again.Status != subscription.Approved {
		t.Errorf("status not preserved on re-contact: %s", again.Status)
	}
	if again.FirstSeen != srv.FirstSeen {
		t.Errorf("FirstSeen changed: %s -> %s", srv.FirstSeen, again.FirstSeen)
	}

	list, _ := s.ListServers()
	if len(list) != 1 {
		t.Fatalf("expected 1 server, got %d", len(list))
	}

	removed, _ := s.RemoveServer("HW-1")
	if !removed {
		t.Error("RemoveServer reported not found")
	}
}

func TestStoreConcurrentWrites(t *testing.T) {
	s := newStore(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = s.AddLicense(subscription.License{Key: keyN(n), Status: subscription.Approved})
		}(i)
	}
	wg.Wait()
	list, _ := s.ListLicenses()
	if len(list) != 20 {
		t.Fatalf("expected 20 licenses after concurrent writes, got %d", len(list))
	}
}

func TestStoreKeepsBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	s := NewStore(path)

	// First write: no prior file, so no backup yet.
	if err := s.AddLicense(subscription.License{Key: "pbsc-1111111111", Status: subscription.Approved}); err != nil {
		t.Fatalf("AddLicense: %v", err)
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Errorf("expected no .bak after first write, got err=%v", err)
	}

	// Second write: the previous good copy must be backed up.
	if err := s.AddLicense(subscription.License{Key: "pbsc-2222222222", Status: subscription.Approved}); err != nil {
		t.Fatalf("AddLicense: %v", err)
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("read .bak: %v", err)
	}
	if !strings.Contains(string(bak), "pbsc-1111111111") || strings.Contains(string(bak), "pbsc-2222222222") {
		t.Errorf(".bak should hold the pre-second-write state, got: %s", bak)
	}
}

func keyN(n int) string {
	const hex = "0123456789abcdef"
	return "pbsc-" + string([]byte{
		hex[(n>>4)&0xf], hex[n&0xf], '0', '0', '0', '0', '0', '0', '0', '0',
	})
}

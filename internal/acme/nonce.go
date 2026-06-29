package acme

import (
	"crypto/rand"
	"sync"
	"time"
)

// NonceStore hands out single-use replay nonces and validates them, so a signed
// request cannot be captured and replayed. It is in-memory and mutex-guarded,
// which suits a single-instance lab server (a restart simply invalidates
// outstanding nonces, and clients fetch a fresh one).
type NonceStore struct {
	mu     sync.Mutex
	issued map[string]time.Time
	ttl    time.Duration
}

// NewNonceStore returns a store whose nonces expire after ttl.
func NewNonceStore(ttl time.Duration) *NonceStore {
	return &NonceStore{issued: make(map[string]time.Time), ttl: ttl}
}

// Issue returns a fresh nonce and records it as valid until now+ttl.
func (n *NonceStore) Issue() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	nonce := b64.EncodeToString(buf)

	n.mu.Lock()
	defer n.mu.Unlock()
	n.gcLocked()
	n.issued[nonce] = time.Now().Add(n.ttl)
	return nonce, nil
}

// Use consumes a nonce: it returns true exactly once for a valid, unexpired
// nonce and false otherwise (unknown, already used or expired).
func (n *NonceStore) Use(nonce string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	exp, ok := n.issued[nonce]
	if !ok {
		return false
	}
	delete(n.issued, nonce)
	return time.Now().Before(exp)
}

// gcLocked drops expired nonces; callers must hold the lock. It keeps the map
// from growing without bound when clients fetch nonces they never use.
func (n *NonceStore) gcLocked() {
	now := time.Now()
	for k, exp := range n.issued {
		if now.After(exp) {
			delete(n.issued, k)
		}
	}
}

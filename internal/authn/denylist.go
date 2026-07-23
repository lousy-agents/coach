package authn

import (
	"context"
	"sync"
	"time"
)

// Denylist checks and records revoked JWT IDs (jti).
// IsRevoked must return a non-nil error on store failure (not a miss) so
// callers can fail closed with 503; a miss is (false, nil).
type Denylist interface {
	IsRevoked(ctx context.Context, jti string) (revoked bool, err error)
	Revoke(ctx context.Context, jti string, exp time.Time) error
}

// MemoryDenylist is an in-process Denylist safe for concurrent use.
type MemoryDenylist struct {
	mu   sync.RWMutex
	byID map[string]time.Time // jti → expiry (cleanup hint)
}

// NewMemoryDenylist returns an empty in-memory denylist.
func NewMemoryDenylist() *MemoryDenylist {
	return &MemoryDenylist{byID: make(map[string]time.Time)}
}

// IsRevoked reports whether jti is present in the denylist.
func (m *MemoryDenylist) IsRevoked(_ context.Context, jti string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.byID[jti]
	return ok, nil
}

// Revoke records jti until exp (retained for later store compaction).
func (m *MemoryDenylist) Revoke(_ context.Context, jti string, exp time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[jti] = exp
	return nil
}

package authn

import (
	"context"
	"sync"
	"time"
)

// OAuthStateStore holds short-lived OAuth CSRF state nonces.
// Consume must return a non-nil error on store failure (not a miss) so callers
// can fail closed; a miss or expired entry is (false, nil).
type OAuthStateStore interface {
	Save(ctx context.Context, state string, expiresAt time.Time) error
	// Consume deletes state if present and unexpired at now.
	Consume(ctx context.Context, state string, now time.Time) (ok bool, err error)
}

// MemoryOAuthState is an in-process OAuthStateStore safe for concurrent use.
type MemoryOAuthState struct {
	mu    sync.Mutex
	byKey map[string]time.Time // state → expiry
}

// NewMemoryOAuthState returns an empty in-memory OAuth state store.
func NewMemoryOAuthState() *MemoryOAuthState {
	return &MemoryOAuthState{byKey: make(map[string]time.Time)}
}

// Save records state until expiresAt.
func (m *MemoryOAuthState) Save(_ context.Context, state string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byKey[state] = expiresAt
	return nil
}

// Consume removes state when present and expiresAt is strictly after now.
func (m *MemoryOAuthState) Consume(_ context.Context, state string, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	exp, ok := m.byKey[state]
	if !ok {
		return false, nil
	}
	delete(m.byKey, state)
	if !exp.After(now) {
		return false, nil
	}
	return true, nil
}

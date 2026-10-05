package authn_test

import (
	"context"
	"sync"
	"time"

	"github.com/lousy-agents/coach/internal/authn"
)

// errDenylist always returns a store error from IsRevoked (fail-closed path).
type errDenylist struct {
	err error
	mu  sync.Mutex
}

func (e *errDenylist) IsRevoked(context.Context, string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return false, e.err
}

func (e *errDenylist) Revoke(context.Context, string, time.Time) error {
	return nil
}

// trackingDenylist wraps a Denylist and records IsRevoked outcomes so tests can
// prove the denylist path ran (not merely expiry).
type trackingDenylist struct {
	inner authn.Denylist
	mu    sync.Mutex
	calls int
	last  *bool
}

func (t *trackingDenylist) IsRevoked(ctx context.Context, jti string) (bool, error) {
	revoked, err := t.inner.IsRevoked(ctx, jti)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls++
	v := revoked
	t.last = &v
	return revoked, err
}

func (t *trackingDenylist) Revoke(ctx context.Context, jti string, exp time.Time) error {
	return t.inner.Revoke(ctx, jti, exp)
}

func (t *trackingDenylist) isRevokedCalls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calls
}

func (t *trackingDenylist) lastRevokedResult() (revoked bool, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.last == nil {
		return false, false
	}
	return *t.last, true
}

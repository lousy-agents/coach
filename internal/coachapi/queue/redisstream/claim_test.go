package redisstream

import (
	"context"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// TestExpiryReclaimInvalidatesOldToken proves reclaimExpired both hands the
// task back after ClaimAfter elapses and invalidates the superseded token,
// per Complete/Nack's stale-token contract.
func TestExpiryReclaimInvalidatesOldToken(t *testing.T) {
	start := time.Unix(0, 0)
	clock := acceptanceharness.NewFakeClock(start)
	q := newTestQueue(clock, time.Minute)

	q.seedClaim("task-1", "token-1", start)

	if _, ok := q.reclaimExpired(); ok {
		t.Fatalf("reclaimExpired() before ClaimAfter elapsed = ok=true, want ok=false")
	}

	clock.Advance(time.Minute)

	claim, ok := q.reclaimExpired()
	if !ok {
		t.Fatalf("reclaimExpired() after ClaimAfter elapsed = ok=false, want ok=true")
	}
	if claim.Token == "token-1" {
		t.Fatalf("reclaimExpired() returned the superseded token %q, want a new token", claim.Token)
	}

	if err := q.Complete(context.Background(), queue.Claim{TaskID: "task-1", Token: "token-1"}); err == nil {
		t.Fatalf("Complete() with the stale, superseded token = nil error, want an error")
	}
	if err := q.Nack(context.Background(), queue.Claim{TaskID: "task-1", Token: "token-1"}, false); err == nil {
		t.Fatalf("Nack() with the stale, superseded token = nil error, want an error")
	}
}

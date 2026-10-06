package redisstream

import (
	"context"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// TestRetryableNackMakesTaskImmediatelyReclaimable proves the fix for
// reviewer finding #1: a retryable Nack must leave the task reclaimable by
// the very next Claim, with Attempt incremented, and must not require a
// full fresh claimAfter window to elapse first. Before the fix, Nack set
// claimedAt to q.clock.Now() (a "freshly claimed" timestamp), so this
// assertion failed with ok=false.
func TestRetryableNackMakesTaskImmediatelyReclaimable(t *testing.T) {
	start := time.Unix(0, 0)
	clock := acceptanceharness.NewFakeClock(start)
	q := newTestQueue(clock, time.Minute)

	q.seedClaim("task-1", "token-1", start)

	if err := q.Nack(context.Background(), queue.Claim{TaskID: "task-1", Token: "token-1", Attempt: 0}, false); err != nil {
		t.Fatalf("Nack(retryable) = %v, want nil", err)
	}

	claim, ok := q.reclaimExpired()
	if !ok {
		t.Fatalf("reclaimExpired() after retryable Nack, with no clock advance = ok=false, want ok=true (task must be immediately claimable again)")
	}
	if claim.TaskID != "task-1" {
		t.Fatalf("reclaimExpired() TaskID = %q, want %q", claim.TaskID, "task-1")
	}
	if claim.Attempt != 1 {
		t.Fatalf("reclaimExpired() Attempt = %d, want 1", claim.Attempt)
	}
}

// TestCompleteAndNackFailOnStaleToken proves takePending rejects an
// already-consumed token (e.g. a claim already Complete'd), independent of
// reclaim.
func TestCompleteAndNackFailOnStaleToken(t *testing.T) {
	start := time.Unix(0, 0)
	clock := acceptanceharness.NewFakeClock(start)
	q := newTestQueue(clock, time.Minute)

	q.seedClaim("task-1", "token-1", start)

	if err := q.Complete(context.Background(), queue.Claim{TaskID: "task-1", Token: "token-1"}); err != nil {
		t.Fatalf("Complete() first call = %v, want nil", err)
	}

	if err := q.Complete(context.Background(), queue.Claim{TaskID: "task-1", Token: "token-1"}); err == nil {
		t.Fatalf("Complete() second call with an already-consumed token = nil error, want an error")
	}
	if err := q.Nack(context.Background(), queue.Claim{TaskID: "task-1", Token: "token-1"}, false); err == nil {
		t.Fatalf("Nack() with an already-consumed token = nil error, want an error")
	}
}

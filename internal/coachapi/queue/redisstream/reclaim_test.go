package redisstream

import (
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// newTestQueue builds a Queue whose state machine (pending map, clock,
// claimAfter) can be exercised directly, with no Redis client/publisher/
// subscriber involved -- Claim/Nack/Complete's reclaim and token
// invalidation logic never touches those fields.
func newTestQueue(clock acceptanceharness.Clock, claimAfter time.Duration) *Queue {
	return &Queue{
		claimAfter: claimAfter,
		clock:      clock,
		pending:    make(map[string]*pendingClaim),
	}
}

func seedClaim(q *Queue, taskID, token string, claimedAt time.Time) *pendingClaim {
	pc := &pendingClaim{
		taskID:    taskID,
		attempt:   0,
		token:     token,
		claimedAt: claimedAt,
		msg:       message.NewMessage(token, []byte("payload")),
	}
	q.pending[token] = pc
	return pc
}

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

	seedClaim(q, "task-1", "token-1", start)

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

// TestExpiryReclaimInvalidatesOldToken proves reclaimExpired both hands the
// task back after ClaimAfter elapses and invalidates the superseded token,
// per Complete/Nack's stale-token contract.
func TestExpiryReclaimInvalidatesOldToken(t *testing.T) {
	start := time.Unix(0, 0)
	clock := acceptanceharness.NewFakeClock(start)
	q := newTestQueue(clock, time.Minute)

	seedClaim(q, "task-1", "token-1", start)

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

// TestCompleteAndNackFailOnStaleToken proves takePending rejects an
// already-consumed token (e.g. a claim already Complete'd), independent of
// reclaim.
func TestCompleteAndNackFailOnStaleToken(t *testing.T) {
	start := time.Unix(0, 0)
	clock := acceptanceharness.NewFakeClock(start)
	q := newTestQueue(clock, time.Minute)

	seedClaim(q, "task-1", "token-1", start)

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

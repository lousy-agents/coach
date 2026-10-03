package redisstream

import (
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

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

func (q *Queue) seedClaim(taskID, token string, claimedAt time.Time) *pendingClaim {
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

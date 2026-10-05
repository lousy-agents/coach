package sqs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

func TestQueueCompleteWithStaleTokenFails(t *testing.T) {
	ctx := context.Background()
	api := newFakeSQS()
	api.queues["https://fake.local/queues/main"] = nil
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newTestQueue(t, api, clock)

	if err := q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("x")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	stale, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("first Claim: ok=%v err=%v", ok, err)
	}

	clock.Advance(24 * time.Hour)

	fresh, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("reclaim Claim: ok=%v err=%v", ok, err)
	}
	if fresh.Attempt != stale.Attempt+1 {
		t.Fatalf("reclaim Attempt = %d, want %d", fresh.Attempt, stale.Attempt+1)
	}

	if err := q.Complete(ctx, stale); !errors.Is(err, errStaleClaim) {
		t.Fatalf("Complete(stale) error = %v, want errStaleClaim", err)
	}
	if err := q.Complete(ctx, fresh); err != nil {
		t.Fatalf("Complete(fresh): %v", err)
	}
}

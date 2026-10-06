package sqs

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// TestQueueDuplicateTaskIDClaimsAreTrackedIndependently proves the SQS
// adapter tolerates duplicate delivery of the same Task.ID (ADR-006
// explicitly permits this), which happens when a task is enqueued more
// than once or SQS redelivers a message that also still has an earlier
// in-flight copy. Before keying q.inflight by receipt handle, the second
// Claim for the same TaskID silently overwrote the first claim's
// bookkeeping, making the first claim's token permanently stale.
func TestQueueDuplicateTaskIDClaimsAreTrackedIndependently(t *testing.T) {
	ctx := context.Background()
	api := newFakeSQS()
	api.queues["https://fake.local/queues/main"] = nil
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newTestQueue(t, api, clock)

	if err := q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("first")}); err != nil {
		t.Fatalf("Enqueue 1: %v", err)
	}
	if err := q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("second")}); err != nil {
		t.Fatalf("Enqueue 2: %v", err)
	}

	first, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("first Claim: ok=%v err=%v", ok, err)
	}
	second, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("second Claim: ok=%v err=%v", ok, err)
	}
	if first.TaskID != second.TaskID {
		t.Fatalf("want both claims for the same TaskID, got %q and %q", first.TaskID, second.TaskID)
	}
	if first.Token == second.Token {
		t.Fatalf("want distinct tokens for two independently delivered messages, got %q twice", first.Token)
	}

	if err := q.Complete(ctx, first); err != nil {
		t.Fatalf("Complete(first) = %v, want success: the first claim's receipt handle must still be tracked", err)
	}
	if err := q.Complete(ctx, second); err != nil {
		t.Fatalf("Complete(second) = %v, want success: completing the first claim must not evict the second", err)
	}
}

func TestQueueClaimWrapsUnderlyingError(t *testing.T) {
	ctx := context.Background()
	q := newTestQueue(t, erroringSQS{newFakeSQS()}, acceptanceharness.NewFakeClock(time.Unix(0, 0)))

	_, _, err := q.Claim(ctx)
	if err == nil {
		t.Fatal("Claim: want error, got nil")
	}
	if got := fmt.Sprint(err); got == "" {
		t.Fatalf("Claim error message is empty")
	}
}

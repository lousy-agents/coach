package queueconformance

import (
	"context"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

func runDuplicateDeliveryDoesNotCorruptState(t *testing.T, newQueue func(tb testing.TB, clock acceptanceharness.Clock) Queue) {
	// ADR-006's behavioral contract promises "possible duplicate
	// delivery" and requires workers to treat redelivery of the same
	// job id as at-least-once. This suite has no broker-level replay
	// mechanism to inject a duplicate directly, so it simulates the
	// closest observable equivalent through the portable Queue
	// surface: re-Enqueue the same task id after it has already been
	// completed (mirroring a broker redelivering a message the
	// original worker already acknowledged, e.g. due to a delayed ack
	// the broker didn't see in time) and assert the queue keeps
	// working -- the redelivered attempt can be claimed and completed
	// on its own terms, without erroring out or leaving the queue in
	// a state where the original completion's bookkeeping is
	// corrupted.
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newQueue(t, clock)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := q.Enqueue(ctx, Task{ID: "task-1", Payload: []byte("payload")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	claim, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("initial Claim: ok=%v err=%v", ok, err)
	}
	if err := q.Complete(ctx, claim); err != nil {
		t.Fatalf("initial Complete: %v", err)
	}

	if err := q.Enqueue(ctx, Task{ID: "task-1", Payload: []byte("payload")}); err != nil {
		t.Fatalf("duplicate Enqueue: %v", err)
	}

	duplicate, ok, err := q.Claim(ctx)
	if err != nil {
		t.Fatalf("duplicate Claim: %v", err)
	}
	if !ok {
		t.Fatalf("duplicate Claim: want the redelivered task-1 to be claimable")
	}
	if duplicate.TaskID != "task-1" {
		t.Fatalf("duplicate claim task id = %q, want %q", duplicate.TaskID, "task-1")
	}
	if err := q.Complete(ctx, duplicate); err != nil {
		t.Fatalf("duplicate Complete: %v", err)
	}
}

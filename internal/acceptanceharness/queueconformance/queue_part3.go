package queueconformance

import (
	"context"

	"github.com/lousy-agents/coach/internal/acceptanceharness"

	"testing"
	"time"
)

func runDuplicateDeliveryDoesNotCorruptState(t *testing.T, newQueue func(tb testing.TB, clock acceptanceharness.Clock) Queue) {

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

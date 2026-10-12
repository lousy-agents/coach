package queueconformance

import (
	"context"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

func runRetryableNackMakesTaskClaimableAgain(t *testing.T, newQueue func(tb testing.TB, clock acceptanceharness.Clock) Queue) {
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newQueue(t, clock)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := q.Enqueue(ctx, Task{ID: "task-1", Payload: []byte("payload")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	first, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("initial Claim: ok=%v err=%v", ok, err)
	}

	if err := q.Nack(ctx, first, false); err != nil {
		t.Fatalf("Nack(permanent=false): %v", err)
	}

	// A retryable Nack must invalidate the pre-Nack claim's token
	// immediately -- not lazily on the next Claim -- so a worker that
	// gets Nack'd and then races a Complete call must fail here, before
	// any other worker has reclaimed the task.
	if err := q.Complete(ctx, first); err == nil {
		t.Fatalf("Complete with stale (pre-Nack) claim immediately after Nack: want error, got nil")
	}

	second, ok, err := q.Claim(ctx)
	if err != nil {
		t.Fatalf("Claim after retryable Nack: %v", err)
	}
	if !ok {
		t.Fatalf("Claim after retryable Nack: want the task to be claimable again")
	}
	if second.TaskID != first.TaskID {
		t.Fatalf("reclaimed task id = %q, want %q", second.TaskID, first.TaskID)
	}
	if second.Attempt != first.Attempt+1 {
		t.Fatalf("reclaimed attempt = %d, want %d", second.Attempt, first.Attempt+1)
	}

	if err := q.Complete(ctx, second); err != nil {
		t.Fatalf("Complete after retryable Nack: %v", err)
	}

	// The pre-Nack claim must be just as stale as a pre-reclaim one.
	if err := q.Complete(ctx, first); err == nil {
		t.Fatalf("Complete with stale (pre-Nack) claim: want error, got nil")
	}
}

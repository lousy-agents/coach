package queueconformance

import (
	"context"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

func runKillMidAttemptEnablesReclaim(t *testing.T, newQueue func(tb testing.TB, clock acceptanceharness.Clock) Queue) {
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newQueue(t, clock)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := q.Enqueue(ctx, Task{ID: "task-1", Payload: []byte("payload")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	first, ok, err := q.Claim(ctx)
	if err != nil {
		t.Fatalf("first Claim: %v", err)
	}
	if !ok {
		t.Fatalf("want first Claim to succeed")
	}

	// Simulate a crash after partial persistence: the first worker
	// never calls Complete. Advancing past the visibility timeout is
	// what a real adapter's reclaim logic reacts to.
	clock.Advance(reclaimAdvance)

	second, ok, err := q.Claim(ctx)
	if err != nil {
		t.Fatalf("reclaim Claim: %v", err)
	}
	if !ok {
		t.Fatalf("want reclaim Claim to succeed once the visibility timeout has elapsed")
	}
	if second.TaskID != first.TaskID {
		t.Fatalf("reclaimed task id = %q, want %q", second.TaskID, first.TaskID)
	}
	if second.Attempt != first.Attempt+1 {
		t.Fatalf("reclaimed attempt = %d, want %d", second.Attempt, first.Attempt+1)
	}
}

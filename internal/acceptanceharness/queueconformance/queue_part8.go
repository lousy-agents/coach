package queueconformance

import (
	"context"

	"github.com/lousy-agents/coach/internal/acceptanceharness"

	"testing"
	"time"
)

func runPostReclaimSingleCompletion(t *testing.T, newQueue func(tb testing.TB, clock acceptanceharness.Clock) Queue) {
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newQueue(t, clock)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := q.Enqueue(ctx, Task{ID: "task-1", Payload: []byte("payload")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	stale, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("initial Claim: ok=%v err=%v", ok, err)
	}

	clock.Advance(reclaimAdvance)

	fresh, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("reclaim Claim: ok=%v err=%v", ok, err)
	}

	if err := q.Complete(ctx, stale); err == nil {
		t.Fatalf("Complete with stale (pre-reclaim) claim: want error, got nil")
	}

	if err := q.Complete(ctx, fresh); err != nil {
		t.Fatalf("Complete with fresh (post-reclaim) claim: %v", err)
	}
}

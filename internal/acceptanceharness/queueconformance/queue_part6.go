package queueconformance

import (
	"context"

	"github.com/lousy-agents/coach/internal/acceptanceharness"

	"testing"
	"time"
)

func runGracefulShutdownDoesNotLoseOrDuplicateClaim(t *testing.T, newQueue func(tb testing.TB, clock acceptanceharness.Clock) Queue) {

	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newQueue(t, clock)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := q.Enqueue(ctx, Task{ID: "task-1", Payload: []byte("payload")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	held, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("initial Claim: ok=%v err=%v", ok, err)
	}

	if _, ok, err := q.Claim(ctx); err != nil {
		t.Fatalf("premature Claim: %v", err)
	} else if ok {
		t.Fatalf("premature Claim: want no claim available before the visibility timeout elapses")
	}

	clock.Advance(reclaimAdvance)

	reclaimed, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("post-shutdown reclaim Claim: ok=%v err=%v", ok, err)
	}
	if err := q.Complete(ctx, reclaimed); err != nil {
		t.Fatalf("Complete after post-shutdown reclaim: %v", err)
	}
	if err := q.Complete(ctx, held); err == nil {
		t.Fatalf("Complete with the original held (pre-reclaim) claim: want error, got nil")
	}
}

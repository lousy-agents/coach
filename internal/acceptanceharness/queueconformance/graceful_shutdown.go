package queueconformance

import (
	"context"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

func runGracefulShutdownDoesNotLoseOrDuplicateClaim(t *testing.T, newQueue func(tb testing.TB, clock acceptanceharness.Clock) Queue) {
	// This is deliberately a thin composition check, not a restatement
	// of "kill-mid-attempt enables reclaim": it only confirms that a
	// worker holding an active claim and simply going quiet (the
	// "graceful shutdown" case, as opposed to a hard kill) does not
	// have its claim reclaimed before the visibility timeout elapses,
	// and that reclaim semantics still compose correctly once it
	// eventually does.
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

	// The shutting-down worker stops calling Claim, but the
	// visibility timeout has not elapsed yet: no other worker should
	// be able to claim task-1.
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

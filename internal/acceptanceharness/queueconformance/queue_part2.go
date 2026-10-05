package queueconformance

import (
	"context"

	"github.com/lousy-agents/coach/internal/acceptanceharness"

	"testing"
	"time"
)

func runPermanentFailureRoutesToPoisonTask(t *testing.T, newQueue func(tb testing.TB, clock acceptanceharness.Clock) Queue) {
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

	if err := q.Nack(ctx, claim, true); err != nil {
		t.Fatalf("Nack(permanent=true): %v", err)
	}

	poisoned, err := q.PoisonTasks(ctx)
	if err != nil {
		t.Fatalf("PoisonTasks: %v", err)
	}
	found := false
	for _, task := range poisoned {
		if task.ID == "task-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("PoisonTasks() = %v, want it to contain task-1", poisoned)
	}

	clock.Advance(reclaimAdvance)
	if _, ok, err := q.Claim(ctx); err != nil {
		t.Fatalf("Claim after permanent Nack: %v", err)
	} else if ok {
		t.Fatalf("Claim after permanent Nack: want no claimable task, got one")
	}
}

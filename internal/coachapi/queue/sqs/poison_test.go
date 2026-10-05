package sqs

import (
	"context"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

func TestQueueNackPermanentRoutesToPoisonQueue(t *testing.T) {
	ctx := context.Background()
	api := newFakeSQS()
	api.queues["https://fake.local/queues/main"] = nil
	api.queues["https://fake.local/queues/main-poison"] = nil
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newTestQueue(t, api, clock)

	if err := q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("payload")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	claim, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("Claim: ok=%v err=%v", ok, err)
	}

	if err := q.Nack(ctx, claim, true); err != nil {
		t.Fatalf("Nack(true): %v", err)
	}

	poisoned, err := q.PoisonTasks(ctx)
	if err != nil {
		t.Fatalf("PoisonTasks: %v", err)
	}
	if len(poisoned) != 1 || poisoned[0].ID != "task-1" || string(poisoned[0].Payload) != "payload" {
		t.Fatalf("PoisonTasks() = %+v, want one task-1/payload entry", poisoned)
	}

	clock.Advance(24 * time.Hour)
	if _, ok, err := q.Claim(ctx); err != nil || ok {
		t.Fatalf("Claim after permanent Nack: ok=%v err=%v, want ok=false", ok, err)
	}

	// PoisonTasks is a repeatable peek, not a drain.
	poisonedAgain, err := q.PoisonTasks(ctx)
	if err != nil {
		t.Fatalf("second PoisonTasks: %v", err)
	}
	if len(poisonedAgain) != 1 {
		t.Fatalf("second PoisonTasks() = %+v, want it to still report task-1", poisonedAgain)
	}
}

package sqs

import (
	"context"
	"errors"

	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

func TestQueueNackRetryableMakesTaskClaimableAgain(t *testing.T) {
	ctx := context.Background()
	api := newFakeSQS()
	api.queues["https://fake.local/queues/main"] = nil
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newTestQueue(t, api, clock)

	if err := q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("x")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	first, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("first Claim: ok=%v err=%v", ok, err)
	}

	if err := q.Nack(ctx, first, false); err != nil {
		t.Fatalf("Nack(false): %v", err)
	}

	second, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("Claim after Nack: ok=%v err=%v", ok, err)
	}
	if second.Attempt != first.Attempt+1 {
		t.Fatalf("Attempt after retryable Nack = %d, want %d", second.Attempt, first.Attempt+1)
	}

	if err := q.Nack(ctx, first, false); !errors.Is(err, errStaleClaim) {
		t.Fatalf("Nack with stale token error = %v, want errStaleClaim", err)
	}
}

func TestQueueEnqueueClaimComplete(t *testing.T) {
	ctx := context.Background()
	api := newFakeSQS()
	api.queues["https://fake.local/queues/main"] = nil
	api.queues["https://fake.local/queues/main-poison"] = nil
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newTestQueue(t, api, clock)

	if err := q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("hello")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	claim, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("Claim: ok=%v err=%v", ok, err)
	}
	if claim.TaskID != "task-1" || claim.Attempt != 0 {
		t.Fatalf("Claim = %+v, want TaskID=task-1 Attempt=0", claim)
	}

	if err := q.Complete(ctx, claim); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if _, ok, err := q.Claim(ctx); err != nil || ok {
		t.Fatalf("Claim after Complete: ok=%v err=%v, want ok=false", ok, err)
	}
}

func newFakeSQS() *fakeSQS {
	return &fakeSQS{
		queues:   make(map[string][]*fakeMessage),
		queueURL: make(map[string]string),
	}
}

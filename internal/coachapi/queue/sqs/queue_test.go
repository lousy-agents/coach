package sqs

import (
	"context"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

func newTestQueue(t *testing.T, api sqsAPI, clock acceptanceharness.Clock) *Queue {
	t.Helper()
	return &Queue{
		client:            api,
		queueURL:          "https://fake.local/queues/main",
		poisonQueueURL:    "https://fake.local/queues/main-poison",
		visibilityTimeout: time.Minute,
		clock:             clock,
		inflight:          make(map[string]*inflightClaim),
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

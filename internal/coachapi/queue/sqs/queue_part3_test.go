package sqs

import (
	"context"

	"fmt"

	"testing"
	"time"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// TestQueueClaimDoesNotHoldLockDuringReclaimNetworkCall proves the fix for
// a review finding on this method: reapExpired (called from within Claim)
// must snapshot expired claims, release q.mu, make its
// ChangeMessageVisibility calls unlocked, and only reacquire q.mu to delete
// them -- not hold q.mu for the whole reclaim loop. It enqueues two tasks,
// claims both (task-1's claim is then made stale via a clock advance),
// blocks ChangeMessageVisibility so a Claim call reclaiming task-1 hangs
// mid-reap, and asserts a concurrent Complete on task-2's still-valid claim
// finishes well before the blocked call is released -- which is only
// possible if reapExpired had already given up the lock.
func TestQueueClaimDoesNotHoldLockDuringReclaimNetworkCall(t *testing.T) {
	ctx := context.Background()
	unblock := make(chan struct{})
	api := &fakeSQS{queues: make(map[string][]*fakeMessage), queueURL: make(map[string]string)}
	blocking := blockingVisibilitySQS{fakeSQS: api, unblock: unblock}
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newTestQueue(t, blocking, clock)

	if err := q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("x")}); err != nil {
		t.Fatalf("Enqueue(task-1): %v", err)
	}
	if err := q.Enqueue(ctx, queue.Task{ID: "task-2", Payload: []byte("y")}); err != nil {
		t.Fatalf("Enqueue(task-2): %v", err)
	}

	first, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("first Claim (task-1): ok=%v err=%v", ok, err)
	}
	second, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("second Claim (task-2): ok=%v err=%v", ok, err)
	}
	if first.TaskID == second.TaskID {
		t.Fatalf("both claims returned the same task id %q, want task-1 and task-2", first.TaskID)
	}

	clock.Advance(24 * time.Hour)

	reclaimDone := make(chan error, 1)
	go func() {
		_, _, err := q.Claim(ctx)
		reclaimDone <- err
	}()

	time.Sleep(20 * time.Millisecond)

	completeDone := make(chan error, 1)
	go func() {
		completeDone <- q.Complete(ctx, second)
	}()

	select {
	case err := <-completeDone:
		if err != nil {
			t.Fatalf("Complete(task-2) while reclaim was blocked: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Complete(task-2) did not return while reapExpired's ChangeMessageVisibility call was blocked -- the lock is still held during the network call")
	case <-reclaimDone:
		t.Fatal("the blocked reclaim finished before Complete could even be observed, so this run could not prove anything -- test setup bug")
	}

	close(unblock)
	if err := <-reclaimDone; err != nil {
		t.Fatalf("reclaiming Claim call: %v", err)
	}
}

func TestQueueClaimWrapsUnderlyingError(t *testing.T) {
	ctx := context.Background()
	q := newTestQueue(t, erroringSQS{newFakeSQS()}, acceptanceharness.NewFakeClock(time.Unix(0, 0)))

	_, _, err := q.Claim(ctx)
	if err == nil {
		t.Fatal("Claim: want error, got nil")
	}
	if got := fmt.Sprint(err); got == "" {
		t.Fatalf("Claim error message is empty")
	}
}

func (f *fakeSQS) ChangeMessageVisibility(ctx context.Context, in *awssqs.ChangeMessageVisibilityInput, _ ...func(*awssqs.Options)) (*awssqs.ChangeMessageVisibilityOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	url := *in.QueueUrl
	msg := f.findByReceiptHandle(url, *in.ReceiptHandle)
	if msg == nil {
		return nil, fakeAPIError{code: "ReceiptHandleIsInvalid"}
	}
	msg.visible = in.VisibilityTimeout == 0
	return &awssqs.ChangeMessageVisibilityOutput{}, nil
}

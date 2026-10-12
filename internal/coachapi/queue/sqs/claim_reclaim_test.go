package sqs

import (
	"context"
	"testing"
	"time"

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

	// Both claims are made *before* advancing the clock, so neither Claim
	// call above went through reapExpired's blocked path. Advancing now
	// makes both claims' deadlines stale, so the next Claim call (below,
	// backgrounded) will try to reclaim one of them via a
	// ChangeMessageVisibility call that blocks on unblock -- and whichever
	// entry it picks, the *other* one (second, completed concurrently
	// below) is still present in q.inflight until that blocked call
	// resolves and reapExpired's delete phase runs.
	clock.Advance(24 * time.Hour)

	reclaimDone := make(chan error, 1)
	go func() {
		_, _, err := q.Claim(ctx) // triggers reapExpired, which blocks on ChangeMessageVisibility
		reclaimDone <- err
	}()

	// Give the reclaiming goroutine a moment to actually enter the blocked
	// call before racing Complete against it.
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

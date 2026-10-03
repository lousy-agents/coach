package queueconformance

import (
	"context"
	"fmt"
	"sync"
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

// runMultiWorkerScaling backs the "multi-worker scaling" subtest. Per
// Queue.Claim's own doc comment, ok=false means only "nothing claimable
// right now", not "the queue is fully drained" -- an adapter with any
// asynchronous delivery latency could hand a worker a spurious empty claim
// mid-drain while other workers still have completions to make. A worker
// that exited its loop on the first ok=false would treat those as
// equivalent and risk finishing early, so every worker instead keeps
// retrying (with a short bounded backoff) until a shared completion count
// reaches taskCount or ctx is done -- the latter remains the real safety
// net against a genuinely broken adapter.
func runMultiWorkerScaling(t *testing.T, newQueue func(tb testing.TB, clock acceptanceharness.Clock) Queue) {
	const taskCount = 20
	const workerCount = 4

	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newQueue(t, clock)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i := 0; i < taskCount; i++ {
		id := fmt.Sprintf("task-%d", i)
		if err := q.Enqueue(ctx, Task{ID: id, Payload: []byte("payload")}); err != nil {
			t.Fatalf("Enqueue(%s): %v", id, err)
		}
	}

	run := multiWorkerRun{completions: make(map[string]int), taskCount: taskCount}
	var wg sync.WaitGroup
	wg.Add(workerCount)
	for w := 0; w < workerCount; w++ {
		go func() {
			defer wg.Done()
			run.worker(ctx, q)
		}()
	}
	wg.Wait()
	run.report(t)
}

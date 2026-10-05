package queueconformance

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

func runDualWorkerExclusion(t *testing.T, newQueue func(tb testing.TB, clock acceptanceharness.Clock) Queue) {
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newQueue(t, clock)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := q.Enqueue(ctx, Task{ID: "task-1", Payload: []byte("payload")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	type result struct {
		claim Claim
		ok    bool
		err   error
	}
	results := make([]result, 2)

	var wg sync.WaitGroup
	wg.Add(len(results))
	for i := range results {
		i := i
		go func() {
			defer wg.Done()
			claim, ok, err := q.Claim(ctx)
			results[i] = result{claim: claim, ok: ok, err: err}
		}()
	}
	wg.Wait()

	successCount := 0
	for _, r := range results {
		if claimSucceeded(t, r.claim, r.ok, r.err) {
			successCount++
		}
	}
	if successCount != 1 {
		t.Fatalf("want exactly 1 successful concurrent claim, got %d", successCount)
	}
}

func claimSucceeded(t *testing.T, claim Claim, ok bool, err error) bool {
	t.Helper()
	if err != nil {
		t.Fatalf("Claim: %v", err)
		return false
	}
	if !ok {
		return false
	}
	if claim.TaskID != "task-1" {
		t.Fatalf("claimed unexpected task id %q, want %q", claim.TaskID, "task-1")
		return false
	}
	return true
}

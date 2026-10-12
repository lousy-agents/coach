package queueconformance

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

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

type multiWorkerRun struct {
	mu             sync.Mutex
	completions    map[string]int
	errs           []error
	totalCompleted int
	taskCount      int
}

func (r *multiWorkerRun) finished() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.totalCompleted >= r.taskCount
}

func (r *multiWorkerRun) fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs = append(r.errs, err)
}

func (r *multiWorkerRun) record(taskID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.completions[taskID]++
	r.totalCompleted++
}

func (r *multiWorkerRun) report(t *testing.T) {
	t.Helper()
	for _, err := range r.errs {
		t.Errorf("worker error: %v", err)
	}
	if len(r.completions) != r.taskCount {
		t.Fatalf("completed %d distinct tasks, want %d: %v", len(r.completions), r.taskCount, r.completions)
	}
	for id, count := range r.completions {
		if count != 1 {
			t.Errorf("task %s completed %d times, want exactly 1", id, count)
		}
	}
}

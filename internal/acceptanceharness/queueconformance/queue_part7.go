package queueconformance

import (
	"context"
	"fmt"

	"testing"
	"time"
)

func (r *multiWorkerRun) worker(ctx context.Context, q Queue) {
	for {
		if r.finished() {
			return
		}
		claim, ok, err := q.Claim(ctx)
		if err != nil {
			r.fail(fmt.Errorf("Claim: %w", err))
			return
		}
		if !ok {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Millisecond):
			}
			continue
		}
		if err := q.Complete(ctx, claim); err != nil {
			r.fail(fmt.Errorf("Complete(%s): %w", claim.TaskID, err))
			return
		}
		r.record(claim.TaskID)
	}
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

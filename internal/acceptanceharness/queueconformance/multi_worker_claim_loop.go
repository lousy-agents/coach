package queueconformance

import (
	"context"
	"fmt"
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

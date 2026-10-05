package worker_test

import (
	"context"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/store/memory"
	"github.com/lousy-agents/coach/internal/coachapi/worker"
)

// succeedAfterRelease signals entered, then waits for release (or ctx
// cancellation) before completing like successHandler.
func succeedAfterRelease(ctx context.Context, job coachapi.Job, w worker.JobWriter, handlerEntered chan struct{}, handlerRelease chan struct{}) (*coachapi.Completion, error) {
	close(handlerEntered)
	select {
	case <-handlerRelease:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return successHandler(ctx, job, w)
}

// heartbeatAdvancedPast reports whether job's stored heartbeat_at is now
// later than firstHB.
func heartbeatAdvancedPast(ctx context.Context, store *memory.Store, job coachapi.Job, firstHB time.Time) bool {
	j, err := store.GetJob(ctx, job.ID)
	if err != nil || j.HeartbeatAt == nil {
		return false
	}
	return j.HeartbeatAt.After(firstHB)
}

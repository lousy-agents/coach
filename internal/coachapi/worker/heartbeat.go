package worker

import (
	"context"
	"errors"

	"github.com/lousy-agents/coach/internal/coachapi"
)

func (w *Worker) heartbeatLoop(ctx context.Context, lease coachapi.ClaimLease, onFenceLost func()) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.clock.After(w.cfg.HeartbeatInterval):
			if w.beatOnce(ctx, lease, onFenceLost) {
				return
			}
		}
	}
}

// beatOnce sends one heartbeat. Returns true when the loop should exit.
func (w *Worker) beatOnce(ctx context.Context, lease coachapi.ClaimLease, onFenceLost func()) bool {
	err := w.store.Heartbeat(ctx, lease.JobID, lease.WorkerID, lease.Attempt, w.clock.Now())
	if err == nil {
		return false
	}
	if errors.Is(err, coachapi.ErrClaimLost) {
		onFenceLost()
		return true
	}
	// Shutdown/timeout from the heartbeat ctx is not fence loss.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// Transient store errors: keep trying until ctx cancels or fence loses.
	return false
}

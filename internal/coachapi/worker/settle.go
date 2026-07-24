package worker

import (
	"context"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// settleMode controls how non-terminal / non-duplicate statuses are handled.
type settleMode int

const (
	// settleForClaim: queued and stale-running fall through so ClaimJob can run.
	settleForClaim settleMode = iota
	// settleForDuplicate: leave the message unacked when not a live duplicate/terminal.
	settleForDuplicate
)

// settleQueueClaim applies terminal and live-duplicate queue disposition.
// settled=true means the queue claim was acked/nacked (or intentionally left
// alone for duplicate mode); settled=false means the caller should try ClaimJob.
func (w *Worker) settleQueueClaim(ctx context.Context, qclaim queue.Claim, job coachapi.Job, mode settleMode) (settled bool, err error) {
	now := w.clock.Now()
	switch job.Status {
	case coachapi.JobStatusCompleted:
		return true, w.queue.Complete(ctx, qclaim)
	case coachapi.JobStatusFailed:
		// Terminal failed may still need poison dispatch if a prior
		// Nack(true) failed after FailJob (ADR-006 permanent-failure).
		return true, w.queue.Nack(ctx, qclaim, true)
	case coachapi.JobStatusRunning:
		return w.settleRunning(ctx, qclaim, job, now, mode)
	case coachapi.JobStatusQueued:
		if mode == settleForDuplicate {
			return true, nil
		}
		return false, nil
	default:
		if mode == settleForClaim {
			return true, w.queue.Complete(ctx, qclaim)
		}
		return true, nil
	}
}

func (w *Worker) settleRunning(ctx context.Context, qclaim queue.Claim, job coachapi.Job, now time.Time, mode settleMode) (bool, error) {
	if hasLiveHeartbeat(job, now, w.cfg.StaleAfter) {
		return true, w.queue.Complete(ctx, qclaim)
	}
	if mode == settleForDuplicate {
		// Still not ours and not a live duplicate — leave unacked for redelivery.
		return true, nil
	}
	return false, nil
}

func hasLiveHeartbeat(job coachapi.Job, now time.Time, staleAfter time.Duration) bool {
	if job.HeartbeatAt == nil {
		return false
	}
	return now.Sub(*job.HeartbeatAt) < staleAfter
}

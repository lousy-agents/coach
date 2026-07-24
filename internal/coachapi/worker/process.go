package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// ProcessNext claims one queue message (if any) and runs disposition +
// handler. ok=false means the queue had nothing claimable.
func (w *Worker) ProcessNext(ctx context.Context) (ok bool, err error) {
	claim, ok, err := w.queue.Claim(ctx)
	if err != nil {
		return false, fmt.Errorf("worker: queue claim: %w", err)
	}
	if !ok {
		return false, nil
	}
	if err := w.processQueueClaim(ctx, claim); err != nil {
		return true, err
	}
	return true, nil
}

func (w *Worker) processQueueClaim(ctx context.Context, qclaim queue.Claim) error {
	job, err := w.loadJobForClaim(ctx, qclaim)
	if err != nil {
		return err
	}
	if job == nil {
		return nil // orphan acked inside loadJobForClaim
	}

	settled, err := w.settleQueueClaim(ctx, qclaim, *job, settleForClaim)
	if err != nil || settled {
		return err
	}
	return w.claimAndRun(ctx, qclaim, *job)
}

func (w *Worker) loadJobForClaim(ctx context.Context, qclaim queue.Claim) (*coachapi.Job, error) {
	job, err := w.store.GetJob(ctx, qclaim.TaskID)
	if err != nil {
		if errors.Is(err, coachapi.ErrJobNotFound) {
			// Unknown id: ack so a poison/orphan message does not loop forever.
			return nil, w.queue.Complete(ctx, qclaim)
		}
		return nil, fmt.Errorf("worker: get job %q: %w", qclaim.TaskID, err)
	}
	return &job, nil
}

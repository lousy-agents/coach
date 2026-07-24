package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

func (w *Worker) claimAndRun(ctx context.Context, qclaim queue.Claim, job coachapi.Job) error {
	lease, err := w.store.ClaimJob(ctx, job.ID, w.cfg.WorkerID, w.clock.Now(), w.cfg.StaleAfter)
	if err != nil {
		if errors.Is(err, coachapi.ErrNotClaimable) {
			// Lost the race or status changed; re-read and ack duplicates.
			return w.ackIfDuplicateOrTerminal(ctx, qclaim)
		}
		return fmt.Errorf("worker: claim job %q: %w", job.ID, err)
	}

	job, err = w.store.GetJob(ctx, lease.JobID)
	if err != nil {
		return fmt.Errorf("worker: reload job %q after claim: %w", lease.JobID, err)
	}
	return w.runClaimed(ctx, qclaim, job, lease)
}

func (w *Worker) ackIfDuplicateOrTerminal(ctx context.Context, qclaim queue.Claim) error {
	job, err := w.store.GetJob(ctx, qclaim.TaskID)
	if err != nil {
		if errors.Is(err, coachapi.ErrJobNotFound) {
			return w.queue.Complete(ctx, qclaim)
		}
		return err
	}
	_, err = w.settleQueueClaim(ctx, qclaim, job, settleForDuplicate)
	return err
}

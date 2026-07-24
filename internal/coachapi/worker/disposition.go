package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// dispositionHandlerError applies ADR-006 failure mapping:
// retryable below MaxAttempts → ReleaseClaim + Nack(false);
// permanent or exhausted → FailJob + Nack(true).
func (w *Worker) dispositionHandlerError(ctx context.Context, qclaim queue.Claim, lease coachapi.ClaimLease, handlerErr error) error {
	if IsRetryable(handlerErr) && lease.Attempt < w.cfg.MaxAttempts {
		return w.releaseForRetry(ctx, qclaim, lease)
	}
	return w.failPermanently(ctx, qclaim, lease, handlerErr)
}

func (w *Worker) releaseForRetry(ctx context.Context, qclaim queue.Claim, lease coachapi.ClaimLease) error {
	if err := w.store.ReleaseClaim(ctx, lease.JobID, lease.WorkerID, lease.Attempt); err != nil {
		if errors.Is(err, coachapi.ErrClaimLost) {
			return coachapi.ErrClaimLost
		}
		return fmt.Errorf("worker: release claim %q: %w", lease.JobID, err)
	}
	if err := w.queue.Nack(ctx, qclaim, false); err != nil {
		return fmt.Errorf("worker: nack retryable %q: %w", lease.JobID, err)
	}
	return nil
}

func (w *Worker) failPermanently(ctx context.Context, qclaim queue.Claim, lease coachapi.ClaimLease, handlerErr error) error {
	finishedAt := w.clock.Now()
	if err := w.store.FailJob(ctx, lease.JobID, lease.WorkerID, lease.Attempt, handlerErr.Error(), finishedAt); err != nil {
		if errors.Is(err, coachapi.ErrClaimLost) {
			return coachapi.ErrClaimLost
		}
		return fmt.Errorf("worker: fail job %q: %w", lease.JobID, err)
	}
	if err := w.queue.Nack(ctx, qclaim, true); err != nil {
		return fmt.Errorf("worker: nack permanent %q: %w", lease.JobID, err)
	}
	return nil
}

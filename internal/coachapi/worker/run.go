package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

func (w *Worker) runClaimed(ctx context.Context, qclaim queue.Claim, job coachapi.Job, lease coachapi.ClaimLease) error {
	hbCtx, hbCancel := context.WithCancel(ctx)
	defer hbCancel()

	var fenceOnce sync.Once
	fenceCh := make(chan struct{})
	signalFenceLost := func() {
		fenceOnce.Do(func() { close(fenceCh) })
	}

	go w.heartbeatLoop(hbCtx, lease, signalFenceLost)

	writer := &leaseWriter{store: w.store, lease: lease}
	handlerDone := make(chan handlerOutcome, 1)
	go func() {
		completion, err := w.handler(hbCtx, job, writer)
		handlerDone <- handlerOutcome{completion: completion, err: err}
	}()

	select {
	case <-ctx.Done():
		hbCancel()
		return ctx.Err()
	case <-fenceCh:
		hbCancel()
		// Abandon without ack so the queue can redeliver.
		return coachapi.ErrClaimLost
	case out := <-handlerDone:
		hbCancel()
		return w.finishHandlerOutcome(ctx, qclaim, lease, out)
	}
}

type handlerOutcome struct {
	completion *coachapi.Completion
	err        error
}

func (w *Worker) finishHandlerOutcome(ctx context.Context, qclaim queue.Claim, lease coachapi.ClaimLease, out handlerOutcome) error {
	if out.err != nil {
		if errors.Is(out.err, coachapi.ErrClaimLost) {
			return coachapi.ErrClaimLost
		}
		return w.dispositionHandlerError(ctx, qclaim, lease, out.err)
	}
	if out.completion == nil {
		return w.dispositionHandlerError(ctx, qclaim, lease, errors.New("handler returned nil completion"))
	}
	return w.completeClaimed(ctx, qclaim, lease, *out.completion)
}

func (w *Worker) completeClaimed(ctx context.Context, qclaim queue.Claim, lease coachapi.ClaimLease, completion coachapi.Completion) error {
	if completion.Attempt == 0 {
		completion.Attempt = lease.Attempt
	}
	if completion.FinishedAt.IsZero() {
		completion.FinishedAt = w.clock.Now()
	}
	if completion.GeneratedAt.IsZero() {
		completion.GeneratedAt = completion.FinishedAt
	}
	if err := w.store.CompleteJob(ctx, lease.JobID, lease.WorkerID, lease.Attempt, completion); err != nil {
		if errors.Is(err, coachapi.ErrClaimLost) {
			return coachapi.ErrClaimLost
		}
		return fmt.Errorf("worker: complete job %q: %w", lease.JobID, err)
	}
	return w.queue.Complete(ctx, qclaim)
}

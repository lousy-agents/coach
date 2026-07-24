package worker

import (
	"context"
	"fmt"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// StartReconciler runs the requeue reconciler until ctx is cancelled. It is
// safe to call once; subsequent calls are no-ops while a reconciler is live.
// After the reconciler exits (parent cancel or StopReconciler), StartReconciler
// may be called again.
func (w *Worker) StartReconciler(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopReconcile != nil {
		return
	}
	rctx, cancel := context.WithCancel(ctx)
	w.reconcileGen++
	gen := w.reconcileGen
	w.stopReconcile = cancel
	go func() {
		defer w.clearReconcilerSlot(gen)
		w.reconcileLoop(rctx)
	}()
}

func (w *Worker) clearReconcilerSlot(gen uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Clear only if we still own the slot (StopReconciler / a newer
	// StartReconciler may have replaced it under a new generation).
	if w.reconcileGen == gen {
		w.stopReconcile = nil
	}
}

// StopReconciler cancels a reconciler started by StartReconciler.
func (w *Worker) StopReconciler() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopReconcile == nil {
		return
	}
	w.stopReconcile()
	w.stopReconcile = nil
	w.reconcileGen++
}

func (w *Worker) reconcileLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.clock.After(w.cfg.ReconcileInterval):
			_ = w.ReconcileOnce(ctx) //nolint:errcheck // best-effort; next tick retries
		}
	}
}

// ReconcileOnce releases stale running jobs to queued and re-enqueues queued
// rows older than QueuedAgeThreshold. It never runs job handlers — only
// TaskQueue.Enqueue.
func (w *Worker) ReconcileOnce(ctx context.Context) error {
	now := w.clock.Now()
	if _, err := w.store.ReleaseStaleRunning(ctx, now, w.cfg.StaleAfter); err != nil {
		return fmt.Errorf("worker: release stale running: %w", err)
	}
	olderThan := now.Add(-w.cfg.QueuedAgeThreshold)
	jobs, err := w.store.ListQueuedOlderThan(ctx, olderThan)
	if err != nil {
		return fmt.Errorf("worker: list queued: %w", err)
	}
	return w.reenqueueQueued(ctx, jobs)
}

func (w *Worker) reenqueueQueued(ctx context.Context, jobs []coachapi.Job) error {
	var firstErr error
	for _, job := range jobs {
		if err := w.enqueueJob(ctx, job.ID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (w *Worker) enqueueJob(ctx context.Context, jobID string) error {
	payload, err := coachapi.MarshalTaskPayload(jobID)
	if err != nil {
		return fmt.Errorf("worker: marshal task payload for %q: %w", jobID, err)
	}
	if err := w.queue.Enqueue(ctx, queue.Task{ID: jobID, Payload: payload}); err != nil {
		return err
	}
	return nil
}

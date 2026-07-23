package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

const (
	defaultHeartbeatInterval  = 15 * time.Second
	defaultStaleAfter         = 60 * time.Second
	defaultReconcileInterval  = 30 * time.Second
	defaultQueuedAgeThreshold = 30 * time.Second
	defaultMaxAttempts        = 5
)

// Config holds injectable worker timings and identity. Zero durations take
// the package defaults (15s heartbeat, 60s stale, 30s reconcile/queued age,
// 5 max attempts). StaleAfter must be at least 3× HeartbeatInterval after
// defaults are applied. MaxAttempts must be >= 1 after defaults.
type Config struct {
	WorkerID           string
	HeartbeatInterval  time.Duration
	StaleAfter         time.Duration
	ReconcileInterval  time.Duration
	QueuedAgeThreshold time.Duration
	// MaxAttempts is the maximum jobs.attempt value that may run a handler
	// before a retryable failure is treated as exhausted (terminal + poison).
	// Permanent handler errors are terminal on any attempt.
	MaxAttempts int
}

func (c Config) withDefaults() (Config, error) {
	if c.WorkerID == "" {
		return Config{}, errors.New("worker: Config.WorkerID is required")
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = defaultHeartbeatInterval
	}
	if c.StaleAfter <= 0 {
		c.StaleAfter = defaultStaleAfter
	}
	if c.ReconcileInterval <= 0 {
		c.ReconcileInterval = defaultReconcileInterval
	}
	if c.QueuedAgeThreshold <= 0 {
		c.QueuedAgeThreshold = defaultQueuedAgeThreshold
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = defaultMaxAttempts
	}
	if c.StaleAfter < 3*c.HeartbeatInterval {
		return Config{}, fmt.Errorf(
			"worker: StaleAfter (%s) must be >= 3× HeartbeatInterval (%s)",
			c.StaleAfter, c.HeartbeatInterval,
		)
	}
	return c, nil
}

// Retryable marks err as a transient handler failure eligible for bounded
// queue redelivery (Nack permanent=false) while lease.Attempt < MaxAttempts.
// Plain errors and errors that do not unwrap to a Retryable marker are
// permanent (FailJob + Nack permanent=true).
func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return retryableError{err: err}
}

// IsRetryable reports whether err (or any error in its unwrap chain) was
// produced by Retryable.
func IsRetryable(err error) bool {
	var target retryableError
	return errors.As(err, &target)
}

type retryableError struct {
	err error
}

func (e retryableError) Error() string { return e.err.Error() }
func (e retryableError) Unwrap() error { return e.err }

// JobWriter is the fenced persistence surface a JobHandler uses while holding
// a claim. Every method is conditional on the handler's ClaimLease.
type JobWriter interface {
	Lease() coachapi.ClaimLease
	InsertFindings(ctx context.Context, findings []coachapi.JobFinding) error
	InsertDiagnostics(ctx context.Context, diagnostics []coachapi.JobDiagnostic) error
}

// JobHandler runs one claimed job attempt. On success it returns a non-nil
// Completion (findings may already have been written via JobWriter). On
// failure it returns a non-nil error:
//   - Retryable(err) below MaxAttempts → ReleaseClaim + Nack(false)
//   - permanent error, or retryable at/above MaxAttempts → FailJob + Nack(true)
//   - errors.Is(err, coachapi.ErrClaimLost) → abandon without ack/nack
type JobHandler func(ctx context.Context, job coachapi.Job, w JobWriter) (*coachapi.Completion, error)

// Worker consumes jobs only through queue.TaskQueue and persists claim
// lifecycle through coachapi.WorkerJobStore.
type Worker struct {
	store   coachapi.WorkerJobStore
	queue   queue.TaskQueue
	clock   acceptanceharness.Clock
	handler JobHandler
	cfg     Config

	mu            sync.Mutex
	stopReconcile context.CancelFunc
}

// New constructs a Worker. clock may be nil (RealClock). handler is required.
func New(store coachapi.WorkerJobStore, q queue.TaskQueue, clock acceptanceharness.Clock, handler JobHandler, cfg Config) (*Worker, error) {
	if store == nil {
		return nil, errors.New("worker: store is required")
	}
	if q == nil {
		return nil, errors.New("worker: queue is required")
	}
	if handler == nil {
		return nil, errors.New("worker: handler is required")
	}
	cfg, err := cfg.withDefaults()
	if err != nil {
		return nil, err
	}
	if clock == nil {
		clock = acceptanceharness.RealClock{}
	}
	return &Worker{store: store, queue: q, clock: clock, handler: handler, cfg: cfg}, nil
}

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
	job, err := w.store.GetJob(ctx, qclaim.TaskID)
	if err != nil {
		if errors.Is(err, coachapi.ErrJobNotFound) {
			// Unknown id: ack so a poison/orphan message does not loop forever.
			return w.queue.Complete(ctx, qclaim)
		}
		return fmt.Errorf("worker: get job %q: %w", qclaim.TaskID, err)
	}

	now := w.clock.Now()
	switch job.Status {
	case coachapi.JobStatusCompleted:
		return w.queue.Complete(ctx, qclaim)
	case coachapi.JobStatusFailed:
		// Terminal failed may still need poison dispatch if a prior
		// Nack(true) failed after FailJob (ADR-006 permanent-failure).
		return w.queue.Nack(ctx, qclaim, true)
	case coachapi.JobStatusRunning:
		if hasLiveHeartbeat(job, now, w.cfg.StaleAfter) {
			return w.queue.Complete(ctx, qclaim)
		}
		// Stale running: fall through and attempt reclaim via ClaimJob.
	case coachapi.JobStatusQueued:
		// Fall through to ClaimJob.
	default:
		return w.queue.Complete(ctx, qclaim)
	}

	lease, err := w.store.ClaimJob(ctx, job.ID, w.cfg.WorkerID, now, w.cfg.StaleAfter)
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
	now := w.clock.Now()
	switch job.Status {
	case coachapi.JobStatusCompleted:
		return w.queue.Complete(ctx, qclaim)
	case coachapi.JobStatusFailed:
		return w.queue.Nack(ctx, qclaim, true)
	case coachapi.JobStatusRunning:
		if hasLiveHeartbeat(job, now, w.cfg.StaleAfter) {
			return w.queue.Complete(ctx, qclaim)
		}
	}
	// Still not ours and not a live duplicate — leave unacked for redelivery.
	return nil
}

func hasLiveHeartbeat(job coachapi.Job, now time.Time, staleAfter time.Duration) bool {
	if job.HeartbeatAt == nil {
		return false
	}
	return now.Sub(*job.HeartbeatAt) < staleAfter
}

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
		if out.err != nil {
			if errors.Is(out.err, coachapi.ErrClaimLost) {
				return coachapi.ErrClaimLost
			}
			return w.dispositionHandlerError(ctx, qclaim, lease, out.err)
		}
		if out.completion == nil {
			return w.dispositionHandlerError(ctx, qclaim, lease, errors.New("handler returned nil completion"))
		}
		completion := *out.completion
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
}

type handlerOutcome struct {
	completion *coachapi.Completion
	err        error
}

// dispositionHandlerError applies ADR-006 failure mapping:
// retryable below MaxAttempts → ReleaseClaim + Nack(false);
// permanent or exhausted → FailJob + Nack(true).
func (w *Worker) dispositionHandlerError(ctx context.Context, qclaim queue.Claim, lease coachapi.ClaimLease, handlerErr error) error {
	retryable := IsRetryable(handlerErr) && lease.Attempt < w.cfg.MaxAttempts
	if retryable {
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

func (w *Worker) heartbeatLoop(ctx context.Context, lease coachapi.ClaimLease, onFenceLost func()) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.clock.After(w.cfg.HeartbeatInterval):
			now := w.clock.Now()
			if err := w.store.Heartbeat(ctx, lease.JobID, lease.WorkerID, lease.Attempt, now); err != nil {
				if errors.Is(err, coachapi.ErrClaimLost) || errors.Is(err, context.Canceled) {
					onFenceLost()
					return
				}
				// Transient store errors: keep trying until ctx cancels or fence loses.
				continue
			}
		}
	}
}

// StartReconciler runs the requeue reconciler until ctx is cancelled. It is
// safe to call once; subsequent calls are no-ops while a reconciler is live.
func (w *Worker) StartReconciler(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopReconcile != nil {
		return
	}
	rctx, cancel := context.WithCancel(ctx)
	w.stopReconcile = cancel
	go w.reconcileLoop(rctx)
}

// StopReconciler cancels a reconciler started by StartReconciler.
func (w *Worker) StopReconciler() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopReconcile != nil {
		w.stopReconcile()
		w.stopReconcile = nil
	}
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
	var firstErr error
	for _, job := range jobs {
		if err := w.queue.Enqueue(ctx, queue.Task{ID: job.ID, Payload: []byte(job.ID)}); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
	}
	return firstErr
}

type leaseWriter struct {
	store coachapi.WorkerJobStore
	lease coachapi.ClaimLease
}

func (w *leaseWriter) Lease() coachapi.ClaimLease { return w.lease }

func (w *leaseWriter) InsertFindings(ctx context.Context, findings []coachapi.JobFinding) error {
	stamped := append([]coachapi.JobFinding(nil), findings...)
	for i := range stamped {
		stamped[i].JobID = w.lease.JobID
		stamped[i].Attempt = w.lease.Attempt
	}
	return w.store.InsertFindings(ctx, w.lease.JobID, w.lease.WorkerID, w.lease.Attempt, stamped)
}

func (w *leaseWriter) InsertDiagnostics(ctx context.Context, diagnostics []coachapi.JobDiagnostic) error {
	stamped := append([]coachapi.JobDiagnostic(nil), diagnostics...)
	for i := range stamped {
		stamped[i].JobID = w.lease.JobID
		stamped[i].Attempt = w.lease.Attempt
	}
	return w.store.InsertDiagnostics(ctx, w.lease.JobID, w.lease.WorkerID, w.lease.Attempt, stamped)
}

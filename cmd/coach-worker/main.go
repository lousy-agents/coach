// Command coach-worker is the composition-root job consumer for the coach
// platform (Task 3 / GitHub issue #104, epic #97): it claims work only through
// queue.TaskQueue, persists claim/heartbeat/fenced writes via
// coachapi.WorkerJobStore, and runs an injectable job handler (stub until
// Task 8 baseline scan lands).
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/worker"
)

func main() {
	cfg, err := loadConfigFromEnv()
	if err != nil {
		log.Fatalf("coach-worker: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	deps, err := buildDependencies(ctx, cfg)
	if err != nil {
		log.Fatalf("coach-worker: %v", err)
	}
	defer deps.Close()

	w, err := worker.New(deps.Store, deps.Queue, acceptanceharness.RealClock{}, stubJobHandler, worker.Config{
		WorkerID:           cfg.WorkerID,
		HeartbeatInterval:  cfg.HeartbeatInterval,
		StaleAfter:         cfg.StaleAfter,
		ReconcileInterval:  cfg.ReconcileInterval,
		QueuedAgeThreshold: cfg.QueuedAgeThreshold,
		MaxAttempts:        cfg.MaxAttempts,
	})
	if err != nil {
		log.Fatalf("coach-worker: %v", err)
	}

	w.StartReconciler(ctx)
	defer w.StopReconciler()

	log.Printf(
		"coach-worker: running (worker_id=%s heartbeat=%s stale=%s reconcile=%s queued_age=%s postgres=%t)",
		cfg.WorkerID, cfg.HeartbeatInterval, cfg.StaleAfter, cfg.ReconcileInterval, cfg.QueuedAgeThreshold,
		cfg.PostgresDSN != "",
	)

	if err := runLoop(ctx, w, cfg.IdlePollInterval); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("coach-worker: %v", err)
	}
}

func runLoop(ctx context.Context, w *worker.Worker, idlePoll time.Duration) error {
	if idlePoll <= 0 {
		idlePoll = time.Second
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		ok, err := w.ProcessNext(ctx)
		if err != nil {
			if errors.Is(err, coachapi.ErrClaimLost) {
				// Another worker reclaimed; queue message left unacked for redelivery.
				log.Printf("coach-worker: claim lost; abandoning without ack")
				continue
			}
			if errors.Is(err, context.Canceled) {
				return err
			}
			log.Printf("coach-worker: process error: %v", err)
		}
		if ok {
			continue
		}
		// Nothing claimable: brief idle wait (real clock; tests drive Worker directly).
		timer := time.NewTimer(idlePoll)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// stubJobHandler is the Task 3 placeholder until Task 8 wires repo_baseline_scan.
// It completes successfully with an empty findings set so the worker lifecycle
// can be exercised end-to-end without analysis dependencies.
func stubJobHandler(_ context.Context, job coachapi.Job, w worker.JobWriter) (*coachapi.Completion, error) {
	lease := w.Lease()
	now := time.Now().UTC()
	return &coachapi.Completion{
		Attempt:     lease.Attempt,
		CommitSHA:   "",
		Versions:    coachapi.ReportVersions{Analyzer: "stub@0"},
		FinishedAt:  now,
		GeneratedAt: now,
	}, nil
}

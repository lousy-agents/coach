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

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi/worker"
)

func main() {
	cfg, err := loadConfigFromEnv()
	if err != nil {
		log.Fatalf("coach-worker: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("coach-worker: %v", err)
	}
}

func run(ctx context.Context, cfg Config) error {
	deps, err := buildDependencies(ctx, cfg)
	if err != nil {
		return err
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
		return err
	}

	w.StartReconciler(ctx)
	defer w.StopReconciler()

	log.Printf(
		"coach-worker: running (worker_id=%s heartbeat=%s stale=%s reconcile=%s queued_age=%s postgres=%t)",
		cfg.WorkerID, cfg.HeartbeatInterval, cfg.StaleAfter, cfg.ReconcileInterval, cfg.QueuedAgeThreshold,
		cfg.PostgresDSN != "",
	)

	return runLoop(ctx, w, cfg.IdlePollInterval)
}

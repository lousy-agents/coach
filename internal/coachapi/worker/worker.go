package worker

import (
	"context"
	"errors"
	"sync"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

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
	reconcileGen  uint64
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

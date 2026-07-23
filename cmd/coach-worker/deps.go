package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
	"github.com/lousy-agents/coach/internal/coachapi/queue/redisstream"
)

// Dependencies are the live collaborators main wires into worker.New.
type Dependencies struct {
	Store coachapi.WorkerJobStore
	Queue queue.TaskQueue

	closers []io.Closer
}

// Close releases queue and pool resources owned by Dependencies.
func (d Dependencies) Close() {
	for i := len(d.closers) - 1; i >= 0; i-- {
		_ = d.closers[i].Close() //nolint:errcheck // best-effort shutdown
	}
}

func buildDependencies(ctx context.Context, cfg Config) (Dependencies, error) {
	taskQueue, err := redisstream.NewQueue(redisstream.Config{
		Address:       cfg.RedisAddr,
		Password:      cfg.RedisPassword,
		DB:            cfg.RedisDB,
		Stream:        cfg.RedisStream,
		ConsumerGroup: cfg.RedisConsumerGroup,
		Consumer:      cfg.RedisConsumer,
		ClaimAfter:    cfg.RedisClaimAfter,
	}, acceptanceharness.RealClock{})
	if err != nil {
		return Dependencies{}, fmt.Errorf("coach-worker: constructing Redis Streams queue: %w", err)
	}

	deps := Dependencies{Queue: taskQueue, closers: []io.Closer{taskQueue}}

	if cfg.PostgresDSN != "" {
		pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
		if err != nil {
			deps.Close()
			return Dependencies{}, fmt.Errorf("coach-worker: constructing Postgres pool: %w", err)
		}
		deps.closers = append(deps.closers, closerFunc(pool.Close))
		deps.Store = coachapi.NewPostgresStore(pool)
	} else {
		deps.Store = coachapi.NewMemoryStore()
	}

	return deps, nil
}

type closerFunc func()

func (f closerFunc) Close() error {
	f()
	return nil
}

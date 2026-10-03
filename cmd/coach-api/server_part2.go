package main

import (
	"context"

	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lousy-agents/coach/internal/acceptanceharness"

	"github.com/lousy-agents/coach/internal/coachapi"

	"github.com/lousy-agents/coach/internal/coachapi/queue/redisstream"
)

// buildDependencies constructs the real Dependencies described by cfg: a
// GitHub-App-authenticated authz.RepoAuthorizer (optionally wrapped in the
// credential-free-smoke BypassAuthorizer), a Redis Streams queue.TaskQueue,
// and either a PostgresStore (cfg.PostgresDSN set) or a MemoryStore. When App
// credentials are absent and the full authz bypass pair is set,
// buildAuthorizer uses a fail-closed deny-all inner instead of a live GitHub
// CredentialResolver (credential-free compose smoke).
func buildDependencies(ctx context.Context, cfg InfraConfig) (Dependencies, error) {
	authorizer, err := buildAuthorizer(cfg)
	if err != nil {
		return Dependencies{}, err
	}

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
		return Dependencies{}, fmt.Errorf("coach-api: constructing Redis Streams queue: %w", err)
	}

	var store coachapi.JobStore
	if cfg.PostgresDSN != "" {
		pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
		if err != nil {
			_ = taskQueue.Close()
			return Dependencies{}, fmt.Errorf("coach-api: constructing Postgres pool: %w", err)
		}
		store = coachapi.NewPostgresStore(pool)
	} else {
		store = coachapi.NewMemoryStore()
	}

	return Dependencies{Store: store, Authorizer: authorizer, Queue: taskQueue}, nil
}

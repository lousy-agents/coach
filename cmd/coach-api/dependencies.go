package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/authz"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
	"github.com/lousy-agents/coach/internal/coachapi/queue/redisstream"
	"github.com/lousy-agents/coach/internal/coachapi/store/memory"
	"github.com/lousy-agents/coach/internal/coachapi/store/postgres"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

// Dependencies are the live collaborators buildHandler composes into the
// coach-api HTTP surface. main() constructs the real ones (Postgres/memory
// store, live GitHub-backed authz.RepoAuthorizer, Redis Streams queue) from
// environment configuration via buildDependencies; tests substitute stubs so
// the composed handler can be exercised end-to-end without a real Redis,
// Postgres, or GitHub App.
type Dependencies struct {
	Store      coachapi.JobStore
	Authorizer authz.RepoAuthorizer
	Queue      queue.TaskQueue
}

// buildDependencies constructs the real Dependencies described by cfg: a
// GitHub-App-authenticated authz.RepoAuthorizer (optionally wrapped in the
// credential-free-smoke BypassAuthorizer), a Redis Streams queue.TaskQueue,
// and either a postgres.Store (cfg.PostgresDSN set) or a memory.Store. When App
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
			_ = taskQueue.Close() //nolint:errcheck // best-effort cleanup; we already have the error to report
			return Dependencies{}, fmt.Errorf("coach-api: constructing Postgres pool: %w", err)
		}
		store = postgres.NewStore(pool)
	} else {
		store = memory.NewStore()
	}

	return Dependencies{Store: store, Authorizer: authorizer, Queue: taskQueue}, nil
}

// buildAuthorizer constructs the submit-time authz.RepoAuthorizer for cfg.
// With GitHub App credentials it builds the live GitHub authorizer (optionally
// bypass-wrapped). With no App credentials it requires the full bypass pair
// and wraps a fail-closed deny-all inner — never a live CredentialResolver.
func buildAuthorizer(cfg InfraConfig) (authz.RepoAuthorizer, error) {
	hasApp := cfg.GitHubAppID > 0 && len(cfg.GitHubAppPrivateKey) > 0
	hasBypass := cfg.AuthzBypassOwner != "" && cfg.AuthzBypassRepo != ""

	if !hasApp {
		if !hasBypass {
			return nil, errors.New("coach-api: GitHub App credentials required unless both COACH_AUTHZ_BYPASS_OWNER and COACH_AUTHZ_BYPASS_REPO are set")
		}
		return wrapAuthorizerForBypass(denyAllRepoAuthorizer{}, cfg), nil
	}

	credentials, err := githubingest.NewCredentialResolver(githubingest.CredentialResolverConfig{
		AppID:      cfg.GitHubAppID,
		PrivateKey: cfg.GitHubAppPrivateKey,
	})
	if err != nil {
		return nil, fmt.Errorf("coach-api: constructing GitHub credential resolver: %w", err)
	}
	authorizer, err := authz.NewGitHubRepoAuthorizer(authz.GitHubRepoAuthorizerConfig{Credentials: credentials})
	if err != nil {
		return nil, fmt.Errorf("coach-api: constructing repo authorizer: %w", err)
	}
	return wrapAuthorizerForBypass(authorizer, cfg), nil
}

// denyAllRepoAuthorizer is the fail-closed inner used when coach-api runs
// without GitHub App credentials (credential-free smoke). Only a surrounding
// BypassAuthorizer can authorize, and only for its exact configured pair.
type denyAllRepoAuthorizer struct{}

func (denyAllRepoAuthorizer) Authorize(context.Context, string, string, string) error {
	return authz.ErrNotAuthorized
}

// wrapAuthorizerForBypass wraps authorizer in authz.NewBypassAuthorizer only
// when both cfg.AuthzBypassOwner and cfg.AuthzBypassRepo are set
// (credential-free-smoke exception). A single one set alone must not
// partially enable the bypass -- authorizer is returned unwrapped in that
// case, so it still fails closed.
func wrapAuthorizerForBypass(authorizer authz.RepoAuthorizer, cfg InfraConfig) authz.RepoAuthorizer {
	if cfg.AuthzBypassOwner != "" && cfg.AuthzBypassRepo != "" {
		return authz.NewBypassAuthorizer(authorizer, cfg.AuthzBypassOwner, cfg.AuthzBypassRepo)
	}
	return authorizer
}

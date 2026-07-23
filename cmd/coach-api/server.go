package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/authn"
	"github.com/lousy-agents/coach/internal/authz"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
	"github.com/lousy-agents/coach/internal/coachapi/queue/redisstream"
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
// Story 3 credential-free-smoke BypassAuthorizer), a Redis Streams
// queue.TaskQueue, and either a PostgresStore (cfg.PostgresDSN set) or a
// MemoryStore.
func buildDependencies(ctx context.Context, cfg InfraConfig) (Dependencies, error) {
	credentials, err := githubingest.NewCredentialResolver(githubingest.CredentialResolverConfig{
		AppID:      cfg.GitHubAppID,
		PrivateKey: cfg.GitHubAppPrivateKey,
	})
	if err != nil {
		return Dependencies{}, fmt.Errorf("coach-api: constructing GitHub credential resolver: %w", err)
	}

	var authorizer authz.RepoAuthorizer
	authorizer, err = authz.NewGitHubRepoAuthorizer(authz.GitHubRepoAuthorizerConfig{Credentials: credentials})
	if err != nil {
		return Dependencies{}, fmt.Errorf("coach-api: constructing repo authorizer: %w", err)
	}
	authorizer = wrapAuthorizerForBypass(authorizer, cfg)

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
		store = coachapi.NewPostgresStore(pool)
	} else {
		store = coachapi.NewMemoryStore()
	}

	return Dependencies{Store: store, Authorizer: authorizer, Queue: taskQueue}, nil
}

// wrapAuthorizerForBypass wraps authorizer in authz.NewBypassAuthorizer only
// when both cfg.AuthzBypassOwner and cfg.AuthzBypassRepo are set (Story 3's
// credential-free-smoke exception). A single one set alone must not
// partially enable the bypass -- authorizer is returned unwrapped in that
// case, so it still fails closed.
func wrapAuthorizerForBypass(authorizer authz.RepoAuthorizer, cfg InfraConfig) authz.RepoAuthorizer {
	if cfg.AuthzBypassOwner != "" && cfg.AuthzBypassRepo != "" {
		return authz.NewBypassAuthorizer(authorizer, cfg.AuthzBypassOwner, cfg.AuthzBypassRepo)
	}
	return authorizer
}

// buildHandler composes internal/authn and internal/coachapi into one HTTP
// surface: authnSvc.Handler() serves /oauth/..., /v1/me, and
// /v1/auth/test-mint; authnSvc.Middleware wraps coachapi.Server.Handler()
// for /v1/jobs and its subpaths, since coachapi.Server does not self-guard
// (see internal/coachapi/server.go's Handler doc comment). A request whose
// path matches none of those registers on the "/" catch-all below, which
// returns the same stable not_found envelope every other unmatched route in
// this API returns (epic #97 Story 1: "All unmatched routes ... return the
// envelope with code not_found").
func buildHandler(cfg Config, deps Dependencies) (http.Handler, error) {
	if deps.Store == nil {
		return nil, errors.New("coach-api: Dependencies.Store is required")
	}
	if deps.Authorizer == nil {
		return nil, errors.New("coach-api: Dependencies.Authorizer is required")
	}
	if deps.Queue == nil {
		return nil, errors.New("coach-api: Dependencies.Queue is required")
	}

	authnSvc, err := authn.New(authn.Options{
		SigningKey:      cfg.JWTSigningKey,
		Issuer:          cfg.JWTIssuer,
		TokenTTL:        cfg.JWTTokenTTL,
		TestMintEnabled: cfg.AuthTestMintEnabled,
		GitHubOAuth:     cfg.GitHubOAuth,
	})
	if err != nil {
		return nil, fmt.Errorf("coach-api: constructing authn service: %w", err)
	}

	coachSrv, err := coachapi.NewServer(coachapi.ServerConfig{
		Store:      deps.Store,
		Authorizer: deps.Authorizer,
		Queue:      deps.Queue,
	})
	if err != nil {
		return nil, fmt.Errorf("coach-api: constructing coachapi server: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/oauth/", authnSvc.Handler())
	mux.Handle("/v1/me", authnSvc.Handler())
	mux.Handle("/v1/auth/test-mint", authnSvc.Handler())

	jobsHandler := authnSvc.Middleware(coachSrv.Handler())
	mux.Handle("/v1/jobs", jobsHandler)
	mux.Handle("/v1/jobs/", jobsHandler)

	mux.HandleFunc("/", writeNotFoundEnvelope)

	return mux, nil
}

// writeNotFoundEnvelope answers any path not claimed by a more specific
// pattern on the composed mux with the same stable not_found envelope every
// other unmatched route in this API returns.
func writeNotFoundEnvelope(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(coachapi.ErrorEnvelope{
		Error: coachapi.APIError{Code: coachapi.ErrorCodeNotFound, Message: "not found"},
	})
}

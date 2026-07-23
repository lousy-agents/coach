// Command coach-api is the composition-root HTTP service for the coach
// platform (Task 2 / GitHub issue #103, epic #97): it wires internal/authn
// (Coach JWT auth, optional GitHub OAuth login, config-gated test mint) and
// internal/coachapi (POST /v1/jobs, GET /v1/jobs/{id}, GET
// /v1/jobs/{id}/report) behind a live authz.RepoAuthorizer and a Redis
// Streams queue.TaskQueue into one runnable server.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"
)

// serverReadHeaderTimeout bounds how long ListenAndServe waits to read a
// request's headers, per AGENTS.md's inbound-HTTP timeout policy (avoids
// slowloris-style hangs on an otherwise idle listener).
const serverReadHeaderTimeout = 10 * time.Second

func main() {
	cfg, err := loadConfigFromEnv()
	if err != nil {
		log.Fatalf("coach-api: %v", err)
	}
	infraCfg, err := loadInfraConfigFromEnv()
	if err != nil {
		log.Fatalf("coach-api: %v", err)
	}

	deps, err := buildDependencies(context.Background(), infraCfg)
	if err != nil {
		log.Fatalf("coach-api: %v", err)
	}

	handler, err := buildHandler(cfg, deps)
	if err != nil {
		log.Fatalf("coach-api: %v", err)
	}

	log.Printf(
		"coach-api: listening on %s (github_oauth=%t test_mint=%t postgres=%t authz_bypass=%t)",
		cfg.HTTPAddr,
		cfg.GitHubOAuth != nil,
		cfg.AuthTestMintEnabled,
		infraCfg.PostgresDSN != "",
		infraCfg.AuthzBypassOwner != "" && infraCfg.AuthzBypassRepo != "",
	)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: serverReadHeaderTimeout,
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("coach-api: server error: %v", err)
	}
}

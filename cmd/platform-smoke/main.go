// Command platform-smoke is the end-to-end credential-free smoke for the local
// platform compose stack (Baseline Scan Story 4 / Task 10): mint → submit
// repo_baseline_scan → poll → assert provenance-tagged report. Exits non-zero
// on any failure. Distinct from Feature Zero's thinproof suite.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "platform-smoke: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("platform-smoke: ok")
}

func run(ctx context.Context) error {
	baseURL := strings.TrimRight(envOr("COACH_PLATFORM_SMOKE_BASE_URL", defaultBaseURL), "/")
	owner := envOr("COACH_SMOKE_REPO_OWNER", defaultOwner)
	repo := envOr("COACH_SMOKE_REPO_NAME", defaultRepo)
	timeout := envDuration("COACH_PLATFORM_SMOKE_TIMEOUT", defaultTimeout)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := &http.Client{Timeout: httpClientTO}

	token, err := mintToken(ctx, client, baseURL)
	if err != nil {
		return err
	}

	jobID, err := submitBaseline(ctx, client, baseURL, token, owner, repo)
	if err != nil {
		return err
	}
	fmt.Printf("platform-smoke: submitted job_id=%s owner=%s repo=%s\n", jobID, owner, repo)

	if err := pollUntilDone(ctx, client, baseURL, token, jobID); err != nil {
		return err
	}

	return assertReport(ctx, client, baseURL, token, jobID)
}

const (
	defaultBaseURL   = "http://127.0.0.1:8080"
	defaultOwner     = "coach-smoke"
	defaultRepo      = "fixture-repo"
	defaultPollEvery = 500 * time.Millisecond
	defaultTimeout   = 2 * time.Minute
	httpClientTO     = 15 * time.Second
)

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

func truncate(b []byte) string {
	const max = 512
	s := string(b)
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

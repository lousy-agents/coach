package main

import (
	"context"

	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func assertReport(ctx context.Context, client *http.Client, baseURL, token, jobID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/jobs/"+jobID+"/report", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("get report: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get report status %d: %s", resp.StatusCode, truncate(raw))
	}
	if err := validateReportBody(raw, jobID); err != nil {
		return err
	}
	fmt.Printf("platform-smoke: report ok job_id=%s\n", jobID)
	return nil
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
func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "platform-smoke: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("platform-smoke: ok")
}

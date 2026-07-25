// Command platform-smoke is the end-to-end credential-free smoke for the local
// platform compose stack (Baseline Scan Story 4 / Task 10): mint → submit
// repo_baseline_scan → poll → assert provenance-tagged report. Exits non-zero
// on any failure. Distinct from Feature Zero's thinproof suite.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultBaseURL   = "http://127.0.0.1:8080"
	defaultOwner     = "coach-smoke"
	defaultRepo      = "fixture-repo"
	defaultPollEvery = 500 * time.Millisecond
	defaultTimeout   = 2 * time.Minute
	httpClientTO     = 15 * time.Second
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

func mintToken(ctx context.Context, client *http.Client, baseURL string) (string, error) {
	body := []byte(`{"subject":"1","login":"platform-smoke"}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/auth/test-mint", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("test-mint request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("test-mint status %d: %s", resp.StatusCode, truncate(raw))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Token == "" {
		return "", fmt.Errorf("test-mint decode: %w body=%s", err, truncate(raw))
	}
	return out.Token, nil
}

func submitBaseline(ctx context.Context, client *http.Client, baseURL, token, owner, repo string) (string, error) {
	payload := map[string]any{
		"kind": "repo_baseline_scan",
		"params": map[string]string{
			"repo_owner": owner,
			"repo_name":  repo,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/jobs", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("submit job: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("submit job status %d: %s", resp.StatusCode, truncate(raw))
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.ID == "" {
		return "", fmt.Errorf("submit job decode: %w body=%s", err, truncate(raw))
	}
	return out.ID, nil
}

func pollUntilDone(ctx context.Context, client *http.Client, baseURL, token, jobID string) error {
	ticker := time.NewTicker(defaultPollEvery)
	defer ticker.Stop()
	for {
		status, errMsg, err := getJobStatus(ctx, client, baseURL, token, jobID)
		if err != nil {
			return err
		}
		switch status {
		case "completed":
			return nil
		case "failed":
			if errMsg == "" {
				errMsg = "(no error message)"
			}
			return fmt.Errorf("job %s failed: %s", jobID, errMsg)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for job %s (last status=%s): %w", jobID, status, ctx.Err())
		case <-ticker.C:
		}
	}
}

func getJobStatus(ctx context.Context, client *http.Client, baseURL, token, jobID string) (status, errMsg string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/jobs/"+jobID, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("get job: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("get job status %d: %s", resp.StatusCode, truncate(raw))
	}
	var out struct {
		Status string  `json:"status"`
		Error  *string `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", "", fmt.Errorf("get job decode: %w body=%s", err, truncate(raw))
	}
	if out.Error != nil {
		errMsg = *out.Error
	}
	return out.Status, errMsg, nil
}

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

	var report struct {
		ReportVersion string `json:"report_version"`
		JobID         string `json:"job_id"`
		Kind          string `json:"kind"`
		Findings      []struct {
			Source string `json:"source"`
		} `json:"findings"`
		Error *string `json:"error"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		return fmt.Errorf("report decode: %w body=%s", err, truncate(raw))
	}
	if report.ReportVersion != "1" {
		return fmt.Errorf("report_version=%q want %q", report.ReportVersion, "1")
	}
	if report.JobID != jobID {
		return fmt.Errorf("report job_id=%q want %q", report.JobID, jobID)
	}
	if report.Kind != "repo_baseline_scan" {
		return fmt.Errorf("report kind=%q want repo_baseline_scan", report.Kind)
	}
	if report.Error != nil {
		return fmt.Errorf("report error set: %s", *report.Error)
	}

	var hasDeterministic, hasAgent bool
	for _, f := range report.Findings {
		switch f.Source {
		case "deterministic":
			hasDeterministic = true
		case "agent":
			hasAgent = true
		default:
			return fmt.Errorf("finding with unexpected source=%q", f.Source)
		}
	}
	if !hasDeterministic && !hasAgent {
		return fmt.Errorf("report has no provenance-tagged findings (need source=deterministic and/or source=agent); body=%s", truncate(raw))
	}
	if !hasDeterministic {
		// Fixture is chosen to produce deterministic mutates_input signals;
		// require at least one deterministic finding so smoke is not a no-op.
		return fmt.Errorf("report missing source=deterministic findings; body=%s", truncate(raw))
	}
	fmt.Printf("platform-smoke: report ok findings=%d deterministic=%t agent=%t\n",
		len(report.Findings), hasDeterministic, hasAgent)
	return nil
}

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

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
	if err := checkSubmitStatus(resp.StatusCode, raw); err != nil {
		return "", err
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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// checkSubmitStatus enforces POST /v1/jobs → 202 Accepted (durable submit contract).
func checkSubmitStatus(statusCode int, body []byte) error {
	if statusCode != http.StatusAccepted {
		return fmt.Errorf("submit job status %d (want 202): %s", statusCode, truncate(body))
	}
	return nil
}

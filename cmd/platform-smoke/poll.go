package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

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

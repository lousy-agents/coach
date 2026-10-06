package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
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

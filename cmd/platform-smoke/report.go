package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// checkSubmitStatus enforces POST /v1/jobs → 202 Accepted (durable submit contract).
func checkSubmitStatus(statusCode int, body []byte) error {
	if statusCode != http.StatusAccepted {
		return fmt.Errorf("submit job status %d (want 202): %s", statusCode, truncate(body))
	}
	return nil
}

// validateReportBody asserts the smoke report contract: report_version 1,
// matching job id/kind, no error, and both deterministic and agent provenance.
func validateReportBody(raw []byte, wantJobID string) error {
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
	if report.JobID != wantJobID {
		return fmt.Errorf("report job_id=%q want %q", report.JobID, wantJobID)
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
	if !hasDeterministic {
		return fmt.Errorf("report missing source=deterministic findings; body=%s", truncate(raw))
	}
	if !hasAgent {
		return fmt.Errorf("report missing source=agent findings (stub/gateway path not proven); body=%s", truncate(raw))
	}
	return nil
}

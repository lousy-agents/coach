package main

import (
	"context"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/worker"
)

// stubJobHandler is the Task 3 placeholder until Task 8 wires repo_baseline_scan.
// It completes successfully with an empty findings set so the worker lifecycle
// can be exercised end-to-end without analysis dependencies.
func stubJobHandler(_ context.Context, _ coachapi.Job, w worker.JobWriter) (*coachapi.Completion, error) {
	lease := w.Lease()
	now := time.Now().UTC()
	return &coachapi.Completion{
		Attempt:     lease.Attempt,
		CommitSHA:   "",
		Versions:    coachapi.ReportVersions{Analyzer: "stub@0"},
		FinishedAt:  now,
		GeneratedAt: now,
	}, nil
}

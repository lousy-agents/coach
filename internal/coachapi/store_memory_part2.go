package coachapi

import (
	"context"

	"fmt"

	"time"
)

// summarizeFindings groups findings by source then rule id (deterministic,
// parsed from the finding payload's rule_id field) or rubric id (agent,
// from the finding's own RubricID) so counts cannot collide across sources.
func summarizeFindings(findings []JobFinding) ReportSummary {
	counts := map[string]map[string]int{}
	for _, f := range findings {
		key := findingSummaryKey(f)
		if key == "" {
			continue
		}
		source := string(f.Source)
		if counts[source] == nil {
			counts[source] = map[string]int{}
		}
		counts[source][key]++
	}
	return ReportSummary{FindingCounts: counts}
}

func findingSummaryKey(f JobFinding) string {
	if f.Source == FindingSourceAgent {
		if f.RubricID != nil {
			return *f.RubricID
		}
		return ""
	}
	return findingRuleID(f.Payload)
}

// ClaimJob implements WorkerJobStore.
func (m *MemoryStore) ClaimJob(ctx context.Context, jobID, workerID string, now time.Time, staleAfter time.Duration) (ClaimLease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return ClaimLease{}, fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}

	switch record.job.Status {
	case JobStatusQueued:

	case JobStatusRunning:

		if record.job.HeartbeatAt != nil && now.Sub(*record.job.HeartbeatAt) < staleAfter {
			return ClaimLease{}, fmt.Errorf("coachapi: job %q: %w", jobID, ErrNotClaimable)
		}
	default:
		return ClaimLease{}, fmt.Errorf("coachapi: job %q: %w", jobID, ErrNotClaimable)
	}

	record.job.Attempt++
	record.job.Status = JobStatusRunning
	wb := workerID
	record.job.ClaimedBy = &wb
	hb := now
	record.job.HeartbeatAt = &hb
	st := now
	record.job.StartedAt = &st
	record.job.FinishedAt = nil
	record.job.Error = nil
	record.findings = nil
	record.diagnostics = nil
	record.completed = false
	record.commitSHA = ""
	record.versions = ReportVersions{}
	record.generatedAt = time.Time{}

	return ClaimLease{
		JobID:     jobID,
		WorkerID:  workerID,
		Attempt:   record.job.Attempt,
		StartedAt: now,
	}, nil
}

// ReleaseClaim implements WorkerJobStore.
func (m *MemoryStore) ReleaseClaim(ctx context.Context, jobID, workerID string, attempt int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}
	if !fenceMatches(record.job, workerID, attempt) {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrClaimLost)
	}

	record.job.Status = JobStatusQueued
	record.job.ClaimedBy = nil
	record.job.HeartbeatAt = nil
	return nil
}

package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// ClaimJob implements WorkerJobStore.
func (m *Store) ClaimJob(ctx context.Context, jobID, workerID string, now time.Time, staleAfter time.Duration) (coachapi.ClaimLease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return coachapi.ClaimLease{}, fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrJobNotFound)
	}

	switch record.job.Status {
	case coachapi.JobStatusQueued:
		// claimable
	case coachapi.JobStatusRunning:
		// Reclaim only when heartbeat is missing or older than staleAfter.
		if record.job.HeartbeatAt != nil && now.Sub(*record.job.HeartbeatAt) < staleAfter {
			return coachapi.ClaimLease{}, fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrNotClaimable)
		}
	default:
		return coachapi.ClaimLease{}, fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrNotClaimable)
	}

	record.job.Attempt++
	record.job.Status = coachapi.JobStatusRunning
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
	record.versions = coachapi.ReportVersions{}
	record.generatedAt = time.Time{}

	return coachapi.ClaimLease{
		JobID:     jobID,
		WorkerID:  workerID,
		Attempt:   record.job.Attempt,
		StartedAt: now,
	}, nil
}

// Heartbeat implements WorkerJobStore.
func (m *Store) Heartbeat(ctx context.Context, jobID, workerID string, attempt int, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrJobNotFound)
	}
	if !fenceMatches(record.job, workerID, attempt) {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrClaimLost)
	}
	hb := now
	record.job.HeartbeatAt = &hb
	return nil
}

// ReleaseClaim implements WorkerJobStore.
func (m *Store) ReleaseClaim(ctx context.Context, jobID, workerID string, attempt int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrJobNotFound)
	}
	if !fenceMatches(record.job, workerID, attempt) {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrClaimLost)
	}

	record.job.Status = coachapi.JobStatusQueued
	record.job.ClaimedBy = nil
	record.job.HeartbeatAt = nil
	return nil
}

func fenceMatches(job coachapi.Job, workerID string, attempt int) bool {
	return job.Status == coachapi.JobStatusRunning &&
		job.ClaimedBy != nil &&
		*job.ClaimedBy == workerID &&
		job.Attempt == attempt
}

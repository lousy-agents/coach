package coachapi

import (
	"context"

	"fmt"

	"time"
)

// ReleaseStaleRunning implements WorkerJobStore.
func (m *MemoryStore) ReleaseStaleRunning(ctx context.Context, now time.Time, staleAfter time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	released := 0
	for _, record := range m.jobs {
		if record.job.Status != JobStatusRunning {
			continue
		}
		if record.job.HeartbeatAt != nil && now.Sub(*record.job.HeartbeatAt) < staleAfter {
			continue
		}
		record.job.Status = JobStatusQueued
		record.job.ClaimedBy = nil
		record.job.HeartbeatAt = nil
		released++
	}
	return released, nil
}

// InsertFindings implements WorkerJobStore.
func (m *MemoryStore) InsertFindings(ctx context.Context, jobID, workerID string, attempt int, findings []JobFinding) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}
	if !fenceMatches(record.job, workerID, attempt) {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrClaimLost)
	}
	stamped := cloneJobFindings(findings)
	for i := range stamped {
		stamped[i].JobID = jobID
		stamped[i].Attempt = attempt
	}
	record.findings = append(record.findings, stamped...)
	return nil
}

// ListQueuedOlderThan implements WorkerJobStore.
func (m *MemoryStore) ListQueuedOlderThan(ctx context.Context, olderThan time.Time) ([]Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []Job
	for _, record := range m.jobs {
		if record.job.Status == JobStatusQueued && record.job.CreatedAt.Before(olderThan) {
			out = append(out, cloneJob(record.job))
		}
	}
	return out, nil
}

// Heartbeat implements WorkerJobStore.
func (m *MemoryStore) Heartbeat(ctx context.Context, jobID, workerID string, attempt int, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}
	if !fenceMatches(record.job, workerID, attempt) {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrClaimLost)
	}
	hb := now
	record.job.HeartbeatAt = &hb
	return nil
}

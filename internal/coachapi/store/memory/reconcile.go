package memory

import (
	"context"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// ListQueuedOlderThan implements WorkerJobStore.
func (m *Store) ListQueuedOlderThan(ctx context.Context, olderThan time.Time) ([]coachapi.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []coachapi.Job
	for _, record := range m.jobs {
		if record.job.Status == coachapi.JobStatusQueued && record.job.CreatedAt.Before(olderThan) {
			out = append(out, cloneJob(record.job))
		}
	}
	return out, nil
}

// ReleaseStaleRunning implements WorkerJobStore.
func (m *Store) ReleaseStaleRunning(ctx context.Context, now time.Time, staleAfter time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	released := 0
	for _, record := range m.jobs {
		if record.job.Status != coachapi.JobStatusRunning {
			continue
		}
		if record.job.HeartbeatAt != nil && now.Sub(*record.job.HeartbeatAt) < staleAfter {
			continue
		}
		record.job.Status = coachapi.JobStatusQueued
		record.job.ClaimedBy = nil
		record.job.HeartbeatAt = nil
		released++
	}
	return released, nil
}

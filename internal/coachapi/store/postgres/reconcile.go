package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// ListQueuedOlderThan implements WorkerJobStore.
func (s *Store) ListQueuedOlderThan(ctx context.Context, olderThan time.Time) ([]coachapi.Job, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, kind, params, status, error, created_at, started_at,
			finished_at, claimed_by, heartbeat_at, attempt,
			created_by_provider, created_by_subject, created_by_login
		FROM jobs
		WHERE status = $1 AND created_at < $2
		ORDER BY created_at ASC`,
		string(coachapi.JobStatusQueued), olderThan,
	)
	if err != nil {
		return nil, fmt.Errorf("coachapi: list queued older than: %w", err)
	}
	defer rows.Close()

	var out []coachapi.Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("coachapi: list queued older than: %w", err)
	}
	return out, nil
}

// ReleaseStaleRunning implements WorkerJobStore.
func (s *Store) ReleaseStaleRunning(ctx context.Context, now time.Time, staleAfter time.Duration) (int, error) {
	staleBefore := now.Add(-staleAfter)
	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs
		SET status = $1, claimed_by = NULL, heartbeat_at = NULL
		WHERE status = $2 AND (heartbeat_at IS NULL OR heartbeat_at <= $3)`,
		string(coachapi.JobStatusQueued), string(coachapi.JobStatusRunning), staleBefore,
	)
	if err != nil {
		return 0, fmt.Errorf("coachapi: release stale running: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

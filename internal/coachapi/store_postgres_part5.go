package coachapi

import (
	"context"

	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"time"
)

// ClaimJob implements WorkerJobStore: claims queued rows or reclaims running
// rows whose heartbeat is older than staleAfter, incrementing attempt and
// deleting prior findings/diagnostics in one transaction.
func (s *PostgresStore) ClaimJob(ctx context.Context, jobID, workerID string, now time.Time, staleAfter time.Duration) (ClaimLease, error) {
	staleBefore := now.Add(-staleAfter)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ClaimLease{}, fmt.Errorf("coachapi: claim job: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var attempt int
	err = tx.QueryRow(ctx, `
		UPDATE jobs
		SET status = $1,
			claimed_by = $2,
			heartbeat_at = $3,
			started_at = $3,
			finished_at = NULL,
			error = NULL,
			attempt = attempt + 1,
			commit_sha = NULL,
			report_versions = NULL,
			generated_at = NULL
		WHERE id = $4
		  AND (
			status = $5
			OR (
				status = $6
				AND (heartbeat_at IS NULL OR heartbeat_at <= $7)
			)
		  )
		RETURNING attempt`,
		string(JobStatusRunning), workerID, now, jobID,
		string(JobStatusQueued), string(JobStatusRunning), staleBefore,
	).Scan(&attempt)
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		// Distinguish not-found from not-claimable.
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jobs WHERE id = $1)`, jobID).Scan(&exists); err != nil {
			return ClaimLease{}, fmt.Errorf("coachapi: claim job %q: %w", jobID, err)
		}
		if !exists {
			return ClaimLease{}, fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
		}
		return ClaimLease{}, fmt.Errorf("coachapi: job %q: %w", jobID, ErrNotClaimable)
	}
	if err != nil {
		return ClaimLease{}, fmt.Errorf("coachapi: claim job %q: %w", jobID, err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM job_findings WHERE job_id = $1`, jobID); err != nil {
		return ClaimLease{}, fmt.Errorf("coachapi: claim job %q: delete findings: %w", jobID, err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM job_diagnostics WHERE job_id = $1`, jobID); err != nil {
		return ClaimLease{}, fmt.Errorf("coachapi: claim job %q: delete diagnostics: %w", jobID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return ClaimLease{}, fmt.Errorf("coachapi: claim job %q: commit: %w", jobID, err)
	}
	return ClaimLease{JobID: jobID, WorkerID: workerID, Attempt: attempt, StartedAt: now}, nil
}

// FailJob implements WorkerJobStore.
func (s *PostgresStore) FailJob(ctx context.Context, jobID, workerID string, attempt int, errMsg string, finishedAt time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs SET status = $1, error = $2, finished_at = $3
		WHERE id = $4 AND status = $5 AND claimed_by = $6 AND attempt = $7`,
		string(JobStatusFailed), errMsg, finishedAt, jobID,
		string(JobStatusRunning), workerID, attempt,
	)
	if err != nil {
		return fmt.Errorf("coachapi: fail job %q: %w", jobID, err)
	}
	if tag.RowsAffected() == 0 {
		return s.fenceFailure(ctx, jobID)
	}
	return nil
}

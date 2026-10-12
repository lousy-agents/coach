package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// ClaimJob implements WorkerJobStore: claims queued rows or reclaims running
// rows whose heartbeat is older than staleAfter, incrementing attempt and
// deleting prior findings/diagnostics in one transaction.
func (s *Store) ClaimJob(ctx context.Context, jobID, workerID string, now time.Time, staleAfter time.Duration) (coachapi.ClaimLease, error) {
	staleBefore := now.Add(-staleAfter)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return coachapi.ClaimLease{}, fmt.Errorf("coachapi: claim job: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

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
		string(coachapi.JobStatusRunning), workerID, now, jobID,
		string(coachapi.JobStatusQueued), string(coachapi.JobStatusRunning), staleBefore,
	).Scan(&attempt)
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		// Distinguish not-found from not-claimable.
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jobs WHERE id = $1)`, jobID).Scan(&exists); err != nil {
			return coachapi.ClaimLease{}, fmt.Errorf("coachapi: claim job %q: %w", jobID, err)
		}
		if !exists {
			return coachapi.ClaimLease{}, fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrJobNotFound)
		}
		return coachapi.ClaimLease{}, fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrNotClaimable)
	}
	if err != nil {
		return coachapi.ClaimLease{}, fmt.Errorf("coachapi: claim job %q: %w", jobID, err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM job_findings WHERE job_id = $1`, jobID); err != nil {
		return coachapi.ClaimLease{}, fmt.Errorf("coachapi: claim job %q: delete findings: %w", jobID, err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM job_diagnostics WHERE job_id = $1`, jobID); err != nil {
		return coachapi.ClaimLease{}, fmt.Errorf("coachapi: claim job %q: delete diagnostics: %w", jobID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return coachapi.ClaimLease{}, fmt.Errorf("coachapi: claim job %q: commit: %w", jobID, err)
	}
	return coachapi.ClaimLease{JobID: jobID, WorkerID: workerID, Attempt: attempt, StartedAt: now}, nil
}

// ReleaseClaim implements WorkerJobStore.
func (s *Store) ReleaseClaim(ctx context.Context, jobID, workerID string, attempt int) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs
		SET status = $1, claimed_by = NULL, heartbeat_at = NULL
		WHERE id = $2 AND status = $3 AND claimed_by = $4 AND attempt = $5`,
		string(coachapi.JobStatusQueued), jobID, string(coachapi.JobStatusRunning), workerID, attempt,
	)
	if err != nil {
		return fmt.Errorf("coachapi: release claim %q: %w", jobID, err)
	}
	if tag.RowsAffected() == 0 {
		return s.fenceFailure(ctx, jobID)
	}
	return nil
}

package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// Heartbeat implements WorkerJobStore.
func (s *Store) Heartbeat(ctx context.Context, jobID, workerID string, attempt int, now time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs SET heartbeat_at = $1
		WHERE id = $2 AND status = $3 AND claimed_by = $4 AND attempt = $5`,
		now, jobID, string(coachapi.JobStatusRunning), workerID, attempt,
	)
	if err != nil {
		return fmt.Errorf("coachapi: heartbeat job %q: %w", jobID, err)
	}
	if tag.RowsAffected() == 0 {
		return s.fenceFailure(ctx, jobID)
	}
	return nil
}

func (s *Store) fenceFailure(ctx context.Context, jobID string) error {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jobs WHERE id = $1)`, jobID).Scan(&exists); err != nil {
		return fmt.Errorf("coachapi: fence check job %q: %w", jobID, err)
	}
	if !exists {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrJobNotFound)
	}
	return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrClaimLost)
}

func (s *Store) fenceFailureTx(ctx context.Context, tx pgx.Tx, jobID string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jobs WHERE id = $1)`, jobID).Scan(&exists); err != nil {
		return fmt.Errorf("coachapi: fence check job %q: %w", jobID, err)
	}
	if !exists {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrJobNotFound)
	}
	return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrClaimLost)
}

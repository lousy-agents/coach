package coachapi

import (
	"context"

	"fmt"

	"github.com/jackc/pgx/v5"

	"time"
)

func (s *PostgresStore) RecordFailure(ctx context.Context, jobID string, errMsg string, finishedAt time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs SET status = $1, error = $2, finished_at = $3 WHERE id = $4`,
		string(JobStatusFailed), errMsg, finishedAt, jobID,
	)
	if err != nil {
		return fmt.Errorf("coachapi: record failure: updating job %q: %w", jobID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}
	return nil
}
func (s *PostgresStore) fenceFailure(ctx context.Context, jobID string) error {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jobs WHERE id = $1)`, jobID).Scan(&exists); err != nil {
		return fmt.Errorf("coachapi: fence check job %q: %w", jobID, err)
	}
	if !exists {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}
	return fmt.Errorf("coachapi: job %q: %w", jobID, ErrClaimLost)
}
func (s *PostgresStore) fenceFailureTx(ctx context.Context, tx pgx.Tx, jobID string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jobs WHERE id = $1)`, jobID).Scan(&exists); err != nil {
		return fmt.Errorf("coachapi: fence check job %q: %w", jobID, err)
	}
	if !exists {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}
	return fmt.Errorf("coachapi: job %q: %w", jobID, ErrClaimLost)
}

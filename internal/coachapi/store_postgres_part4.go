package coachapi

import (
	"context"

	"fmt"

	"time"
)

// InsertDiagnostics implements WorkerJobStore (same atomic fence/stamping as InsertFindings).
func (s *PostgresStore) InsertDiagnostics(ctx context.Context, jobID, workerID string, attempt int, diagnostics []JobDiagnostic) error {
	if len(diagnostics) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("coachapi: insert diagnostics: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if fenceHoldForTest != nil {
		fenceHoldForTest()
	}
	for _, d := range diagnostics {
		createdAt := d.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO job_diagnostics (id, job_id, attempt, scope, message, created_at)
			SELECT $1,$2,$3,$4,$5,$6
			WHERE EXISTS (
				SELECT 1 FROM jobs
				WHERE id = $2 AND status = $7 AND claimed_by = $8 AND attempt = $3
			)`,
			d.ID, jobID, attempt, d.Scope, d.Message, createdAt,
			string(JobStatusRunning), workerID,
		)
		if err != nil {
			return fmt.Errorf("coachapi: insert diagnostics: inserting %q: %w", d.ID, err)
		}
		if tag.RowsAffected() == 0 {
			return s.fenceFailureTx(ctx, tx, jobID)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("coachapi: insert diagnostics: commit: %w", err)
	}
	return nil
}

// Heartbeat implements WorkerJobStore.
func (s *PostgresStore) Heartbeat(ctx context.Context, jobID, workerID string, attempt int, now time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs SET heartbeat_at = $1
		WHERE id = $2 AND status = $3 AND claimed_by = $4 AND attempt = $5`,
		now, jobID, string(JobStatusRunning), workerID, attempt,
	)
	if err != nil {
		return fmt.Errorf("coachapi: heartbeat job %q: %w", jobID, err)
	}
	if tag.RowsAffected() == 0 {
		return s.fenceFailure(ctx, jobID)
	}
	return nil
}

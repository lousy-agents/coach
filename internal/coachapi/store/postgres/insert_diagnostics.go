package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// InsertDiagnostics implements WorkerJobStore (same atomic fence/stamping as InsertFindings).
func (s *Store) InsertDiagnostics(ctx context.Context, jobID, workerID string, attempt int, diagnostics []coachapi.JobDiagnostic) error {
	if len(diagnostics) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("coachapi: insert diagnostics: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

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
			string(coachapi.JobStatusRunning), workerID,
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

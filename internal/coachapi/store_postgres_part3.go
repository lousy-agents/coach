package coachapi

import (
	"context"

	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"time"
)

// InsertFindings implements WorkerJobStore. Each row uses INSERT…SELECT gated
// on the lease fence (atomic; no check-then-act), and job_id/attempt are taken
// from the lease args rather than client-supplied finding fields.
func (s *PostgresStore) InsertFindings(ctx context.Context, jobID, workerID string, attempt int, findings []JobFinding) error {
	if len(findings) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("coachapi: insert findings: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if fenceHoldForTest != nil {
		fenceHoldForTest()
	}
	for _, f := range findings {
		createdAt := f.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO job_findings (
				id, job_id, attempt, source, rubric_id, rubric_version,
				model_identity, payload, payload_hash, created_at
			)
			SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10
			WHERE EXISTS (
				SELECT 1 FROM jobs
				WHERE id = $2 AND status = $11 AND claimed_by = $12 AND attempt = $3
			)`,
			f.ID, jobID, attempt, string(f.Source), f.RubricID,
			f.RubricVersion, f.ModelIdentity, f.Payload, f.PayloadHash, createdAt,
			string(JobStatusRunning), workerID,
		)
		if err != nil {
			return fmt.Errorf("coachapi: insert findings: inserting %q: %w", f.ID, err)
		}
		if tag.RowsAffected() == 0 {
			return s.fenceFailureTx(ctx, tx, jobID)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("coachapi: insert findings: commit: %w", err)
	}
	return nil
}
func scanJob(row pgx.Row) (Job, error) {
	var (
		job    Job
		kind   string
		status string
	)
	err := row.Scan(
		&job.ID, &kind, &job.Params, &status, &job.Error, &job.CreatedAt,
		&job.StartedAt, &job.FinishedAt, &job.ClaimedBy, &job.HeartbeatAt,
		&job.Attempt, &job.CreatedByProvider, &job.CreatedBySubject,
		&job.CreatedByLogin,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, fmt.Errorf("coachapi: job: %w", ErrJobNotFound)
		}
		return Job{}, fmt.Errorf("coachapi: get job: %w", err)
	}
	job.Kind = JobKind(kind)
	job.Status = JobStatus(status)
	return job, nil
}

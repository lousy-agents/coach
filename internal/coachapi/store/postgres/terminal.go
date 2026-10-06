package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// CompleteJob implements WorkerJobStore.
func (s *Store) CompleteJob(ctx context.Context, jobID, workerID string, attempt int, completion coachapi.Completion) error {
	versionsRaw, err := json.Marshal(completion.Versions)
	if err != nil {
		return fmt.Errorf("coachapi: complete job: encoding versions: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("coachapi: complete job: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	tag, err := tx.Exec(ctx, `
		UPDATE jobs
		SET status = $1, attempt = $2, finished_at = $3, error = NULL,
			commit_sha = $4, report_versions = $5, generated_at = $6
		WHERE id = $7 AND status = $8 AND claimed_by = $9 AND attempt = $10`,
		string(coachapi.JobStatusCompleted), attempt, completion.FinishedAt,
		completion.CommitSHA, versionsRaw, completion.GeneratedAt, jobID,
		string(coachapi.JobStatusRunning), workerID, attempt,
	)
	if err != nil {
		return fmt.Errorf("coachapi: complete job %q: %w", jobID, err)
	}
	if tag.RowsAffected() == 0 {
		return s.fenceFailureTx(ctx, tx, jobID)
	}

	seq := 0
	nextCreatedAt := func() time.Time {
		t := completion.GeneratedAt.Add(time.Duration(seq) * time.Nanosecond)
		seq++
		return t
	}
	for _, f := range completion.Findings {
		if _, err := tx.Exec(ctx, `
			INSERT INTO job_findings (
				id, job_id, attempt, source, rubric_id, rubric_version,
				model_identity, payload, payload_hash, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			f.ID, jobID, attempt, string(f.Source), f.RubricID,
			f.RubricVersion, f.ModelIdentity, f.Payload, f.PayloadHash,
			nextCreatedAt(),
		); err != nil {
			return fmt.Errorf("coachapi: complete job: inserting finding %q: %w", f.ID, err)
		}
	}
	for _, d := range completion.Diagnostics {
		if _, err := tx.Exec(ctx, `
			INSERT INTO job_diagnostics (id, job_id, attempt, scope, message, created_at)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			d.ID, jobID, attempt, d.Scope, d.Message, nextCreatedAt(),
		); err != nil {
			return fmt.Errorf("coachapi: complete job: inserting diagnostic %q: %w", d.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("coachapi: complete job: commit: %w", err)
	}
	return nil
}

// FailJob implements WorkerJobStore.
func (s *Store) FailJob(ctx context.Context, jobID, workerID string, attempt int, errMsg string, finishedAt time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs SET status = $1, error = $2, finished_at = $3
		WHERE id = $4 AND status = $5 AND claimed_by = $6 AND attempt = $7`,
		string(coachapi.JobStatusFailed), errMsg, finishedAt, jobID,
		string(coachapi.JobStatusRunning), workerID, attempt,
	)
	if err != nil {
		return fmt.Errorf("coachapi: fail job %q: %w", jobID, err)
	}
	if tag.RowsAffected() == 0 {
		return s.fenceFailure(ctx, jobID)
	}
	return nil
}

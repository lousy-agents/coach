package coachapi

import (
	"context"
	"encoding/json"

	"fmt"

	"time"
)

// CompleteJob implements WorkerJobStore.
func (s *PostgresStore) CompleteJob(ctx context.Context, jobID, workerID string, attempt int, completion Completion) error {
	versionsRaw, err := json.Marshal(completion.Versions)
	if err != nil {
		return fmt.Errorf("coachapi: complete job: encoding versions: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("coachapi: complete job: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE jobs
		SET status = $1, attempt = $2, finished_at = $3, error = NULL,
			commit_sha = $4, report_versions = $5, generated_at = $6
		WHERE id = $7 AND status = $8 AND claimed_by = $9 AND attempt = $10`,
		string(JobStatusCompleted), attempt, completion.FinishedAt,
		completion.CommitSHA, versionsRaw, completion.GeneratedAt, jobID,
		string(JobStatusRunning), workerID, attempt,
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

// ReleaseStaleRunning implements WorkerJobStore.
func (s *PostgresStore) ReleaseStaleRunning(ctx context.Context, now time.Time, staleAfter time.Duration) (int, error) {
	staleBefore := now.Add(-staleAfter)
	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs
		SET status = $1, claimed_by = NULL, heartbeat_at = NULL
		WHERE status = $2 AND (heartbeat_at IS NULL OR heartbeat_at <= $3)`,
		string(JobStatusQueued), string(JobStatusRunning), staleBefore,
	)
	if err != nil {
		return 0, fmt.Errorf("coachapi: release stale running: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// RecordCompletion finalizes one attempt atomically: the jobs row update and
// every finding/diagnostic insert happen in a single transaction, so a
// caller never observes a completed job with a partially-written report.
// created_at for inserted findings/diagnostics is derived from
// completion.GeneratedAt plus a strictly increasing per-row microsecond
// offset (rather than left to Postgres's now(), which returns one fixed
// value for the whole transaction) so report assembly's created_at ordering
// matches completion.Findings/Diagnostics input order deterministically.
// The offset unit must be microseconds, not nanoseconds: Postgres's
// TIMESTAMPTZ only stores microsecond precision, so a nanosecond offset is
// silently truncated on write and rows can collide onto the same stored
// value, breaking the intended ordering.
func (s *Store) RecordCompletion(ctx context.Context, jobID string, completion coachapi.Completion) error {
	versionsRaw, err := json.Marshal(completion.Versions)
	if err != nil {
		return fmt.Errorf("coachapi: record completion: encoding versions: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("coachapi: record completion: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	tag, err := tx.Exec(ctx, `
		UPDATE jobs
		SET status = $1, attempt = $2, finished_at = $3, error = NULL,
			commit_sha = $4, report_versions = $5, generated_at = $6
		WHERE id = $7`,
		string(coachapi.JobStatusCompleted), completion.Attempt, completion.FinishedAt,
		completion.CommitSHA, versionsRaw, completion.GeneratedAt, jobID,
	)
	if err != nil {
		return fmt.Errorf("coachapi: record completion: updating job %q: %w", jobID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrJobNotFound)
	}

	seq := 0
	nextCreatedAt := func() time.Time {
		t := completion.GeneratedAt.Add(time.Duration(seq) * time.Microsecond)
		seq++
		return t
	}

	for _, f := range completion.Findings {
		if _, err := tx.Exec(ctx, `
			INSERT INTO job_findings (
				id, job_id, attempt, source, rubric_id, rubric_version,
				model_identity, payload, payload_hash, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			f.ID, f.JobID, f.Attempt, string(f.Source), f.RubricID,
			f.RubricVersion, f.ModelIdentity, f.Payload, f.PayloadHash,
			nextCreatedAt(),
		); err != nil {
			return fmt.Errorf("coachapi: record completion: inserting finding %q: %w", f.ID, err)
		}
	}

	for _, d := range completion.Diagnostics {
		if _, err := tx.Exec(ctx, `
			INSERT INTO job_diagnostics (id, job_id, attempt, scope, message, created_at)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			d.ID, d.JobID, d.Attempt, d.Scope, d.Message, nextCreatedAt(),
		); err != nil {
			return fmt.Errorf("coachapi: record completion: inserting diagnostic %q: %w", d.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("coachapi: record completion: commit: %w", err)
	}
	return nil
}

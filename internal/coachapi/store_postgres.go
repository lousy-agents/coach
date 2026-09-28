package coachapi

import (
	"context"
	"encoding/json"

	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore is the durable JobStore backed by Postgres (schema in
// internal/coachapi/migrations). It uses the native pgx/v5 pool rather than
// database/sql: scanning nullable columns straight into Job's existing
// *string/*time.Time fields via double-pointer targets, and jsonb columns
// straight into json.RawMessage, are both pgx-native conveniences that
// database/sql's driver.Value interface does not offer as directly.
type PostgresStore struct {
	pool *pgxpool.Pool
}

var _ JobStore = (*PostgresStore)(nil)

// NewPostgresStore returns a JobStore backed by pool. The caller owns pool's
// lifecycle (construction and Close); PostgresStore never closes it.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) CreateJob(ctx context.Context, job Job) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO jobs (
			id, kind, params, status, error, created_at, started_at,
			finished_at, claimed_by, heartbeat_at, attempt,
			created_by_provider, created_by_subject, created_by_login
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		job.ID, string(job.Kind), job.Params, string(job.Status), job.Error,
		job.CreatedAt, job.StartedAt, job.FinishedAt, job.ClaimedBy,
		job.HeartbeatAt, job.Attempt, job.CreatedByProvider,
		job.CreatedBySubject, job.CreatedByLogin,
	)
	if err != nil {
		return fmt.Errorf("coachapi: create job %q: %w", job.ID, err)
	}
	return nil
}

func (s *PostgresStore) GetJob(ctx context.Context, id string) (Job, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, kind, params, status, error, created_at, started_at,
			finished_at, claimed_by, heartbeat_at, attempt,
			created_by_provider, created_by_subject, created_by_login
		FROM jobs WHERE id = $1`, id)
	return scanJob(row)
}

// GetReport returns ErrJobNotFound-wrapped both when id does not exist and
// when it exists but has never had a successful RecordCompletion (detected
// via generated_at IS NULL, which RecordCompletion sets atomically with
// every other report-assembly column), since normal callers check
// Job.Status via GetJob before calling GetReport.

// findingsForReport returns the findings for job id's final recorded
// attempt (jobs.attempt, as set by the completing RecordCompletion call),
// ordered by created_at so report assembly is deterministic. Rows from a
// discarded, non-final attempt are intentionally excluded.

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
func (s *PostgresStore) RecordCompletion(ctx context.Context, jobID string, completion Completion) error {
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
		string(JobStatusCompleted), completion.Attempt, completion.FinishedAt,
		completion.CommitSHA, versionsRaw, completion.GeneratedAt, jobID,
	)
	if err != nil {
		return fmt.Errorf("coachapi: record completion: updating job %q: %w", jobID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
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

// ClaimJob implements WorkerJobStore: claims queued rows or reclaims running
// rows whose heartbeat is older than staleAfter, incrementing attempt and
// deleting prior findings/diagnostics in one transaction.

//nolint:errcheck // no-op once committed

// Distinguish not-found from not-claimable.

// Heartbeat implements WorkerJobStore.

// InsertFindings implements WorkerJobStore. Each row uses INSERT…SELECT gated
// on the lease fence (atomic; no check-then-act), and job_id/attempt are taken
// from the lease args rather than client-supplied finding fields.

//nolint:errcheck // no-op once committed

// InsertDiagnostics implements WorkerJobStore (same atomic fence/stamping as InsertFindings).

//nolint:errcheck // no-op once committed

// CompleteJob implements WorkerJobStore.

//nolint:errcheck // no-op once committed

// FailJob implements WorkerJobStore.

// ReleaseClaim implements WorkerJobStore.

// ListQueuedOlderThan implements WorkerJobStore.

// ReleaseStaleRunning implements WorkerJobStore.

// fenceHoldForTest, when non-nil, runs before atomic fenced inserts so
// acceptance tests can interleave ClaimJob reclaim. nil outside tests.
var fenceHoldForTest func()

var _ WorkerJobStore = (*PostgresStore)(nil)

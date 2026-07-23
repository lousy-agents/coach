package coachapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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

// GetReport returns ErrJobNotFound-wrapped both when id does not exist and
// when it exists but has never had a successful RecordCompletion (detected
// via generated_at IS NULL, which RecordCompletion sets atomically with
// every other report-assembly column), since normal callers check
// Job.Status via GetJob before calling GetReport.
func (s *PostgresStore) GetReport(ctx context.Context, id string) (Report, error) {
	var (
		report      Report
		kind        string
		versionsRaw json.RawMessage
	)
	err := s.pool.QueryRow(ctx, `
		SELECT kind, params, commit_sha, error, report_versions, generated_at
		FROM jobs
		WHERE id = $1 AND generated_at IS NOT NULL`, id,
	).Scan(&kind, &report.Params, &report.CommitSHA, &report.Error, &versionsRaw, &report.GeneratedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Report{}, fmt.Errorf("coachapi: report for job %q: %w", id, ErrJobNotFound)
		}
		return Report{}, fmt.Errorf("coachapi: get report: %w", err)
	}
	report.ReportVersion = ReportVersion1
	report.JobID = id
	report.Kind = JobKind(kind)
	if err := json.Unmarshal(versionsRaw, &report.Versions); err != nil {
		return Report{}, fmt.Errorf("coachapi: get report: decoding report_versions: %w", err)
	}

	findings, err := s.findingsForReport(ctx, id)
	if err != nil {
		return Report{}, err
	}
	diagnostics, err := s.diagnosticsForReport(ctx, id)
	if err != nil {
		return Report{}, err
	}
	report.Summary = summarizeFindings(findings)
	report.Findings = toReportFindings(findings)
	report.Diagnostics = toReportDiagnostics(diagnostics)

	return report, nil
}

// findingsForReport returns the findings for job id's final recorded
// attempt (jobs.attempt, as set by the completing RecordCompletion call),
// ordered by created_at so report assembly is deterministic. Rows from a
// discarded, non-final attempt are intentionally excluded.
func (s *PostgresStore) findingsForReport(ctx context.Context, jobID string) ([]JobFinding, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT f.id, f.job_id, f.attempt, f.source, f.rubric_id,
			f.rubric_version, f.model_identity, f.payload, f.payload_hash,
			f.created_at
		FROM job_findings f
		JOIN jobs j ON j.id = f.job_id
		WHERE f.job_id = $1 AND f.attempt = j.attempt
		ORDER BY f.created_at ASC, f.id ASC`, jobID)
	if err != nil {
		return nil, fmt.Errorf("coachapi: get report: querying findings: %w", err)
	}
	defer rows.Close()

	var findings []JobFinding
	for rows.Next() {
		var (
			f      JobFinding
			source string
		)
		if err := rows.Scan(
			&f.ID, &f.JobID, &f.Attempt, &source, &f.RubricID,
			&f.RubricVersion, &f.ModelIdentity, &f.Payload, &f.PayloadHash,
			&f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("coachapi: get report: scanning finding: %w", err)
		}
		f.Source = FindingSource(source)
		findings = append(findings, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("coachapi: get report: iterating findings: %w", err)
	}
	return findings, nil
}

func (s *PostgresStore) diagnosticsForReport(ctx context.Context, jobID string) ([]JobDiagnostic, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id, d.job_id, d.attempt, d.scope, d.message, d.created_at
		FROM job_diagnostics d
		JOIN jobs j ON j.id = d.job_id
		WHERE d.job_id = $1 AND d.attempt = j.attempt
		ORDER BY d.created_at ASC, d.id ASC`, jobID)
	if err != nil {
		return nil, fmt.Errorf("coachapi: get report: querying diagnostics: %w", err)
	}
	defer rows.Close()

	var diagnostics []JobDiagnostic
	for rows.Next() {
		var d JobDiagnostic
		if err := rows.Scan(&d.ID, &d.JobID, &d.Attempt, &d.Scope, &d.Message, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("coachapi: get report: scanning diagnostic: %w", err)
		}
		diagnostics = append(diagnostics, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("coachapi: get report: iterating diagnostics: %w", err)
	}
	return diagnostics, nil
}

// RecordCompletion finalizes one attempt atomically: the jobs row update and
// every finding/diagnostic insert happen in a single transaction, so a
// caller never observes a completed job with a partially-written report.
// created_at for inserted findings/diagnostics is derived from
// completion.GeneratedAt plus a strictly increasing per-row offset (rather
// than left to Postgres's now(), which returns one fixed value for the
// whole transaction) so report assembly's created_at ordering matches
// completion.Findings/Diagnostics input order deterministically.
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

// ClaimJob implements WorkerJobStore: claims queued rows or reclaims running
// rows whose heartbeat is older than staleAfter, incrementing attempt and
// deleting prior findings/diagnostics in one transaction.
func (s *PostgresStore) ClaimJob(ctx context.Context, jobID, workerID string, now time.Time, staleAfter time.Duration) (ClaimLease, error) {
	staleBefore := now.Add(-staleAfter)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ClaimLease{}, fmt.Errorf("coachapi: claim job: begin tx: %w", err)
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
		string(JobStatusRunning), workerID, now, jobID,
		string(JobStatusQueued), string(JobStatusRunning), staleBefore,
	).Scan(&attempt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Distinguish not-found from not-claimable.
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jobs WHERE id = $1)`, jobID).Scan(&exists); err != nil {
				return ClaimLease{}, fmt.Errorf("coachapi: claim job %q: %w", jobID, err)
			}
			if !exists {
				return ClaimLease{}, fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
			}
			return ClaimLease{}, fmt.Errorf("coachapi: job %q: %w", jobID, ErrNotClaimable)
		}
		return ClaimLease{}, fmt.Errorf("coachapi: claim job %q: %w", jobID, err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM job_findings WHERE job_id = $1`, jobID); err != nil {
		return ClaimLease{}, fmt.Errorf("coachapi: claim job %q: delete findings: %w", jobID, err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM job_diagnostics WHERE job_id = $1`, jobID); err != nil {
		return ClaimLease{}, fmt.Errorf("coachapi: claim job %q: delete diagnostics: %w", jobID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return ClaimLease{}, fmt.Errorf("coachapi: claim job %q: commit: %w", jobID, err)
	}
	return ClaimLease{JobID: jobID, WorkerID: workerID, Attempt: attempt, StartedAt: now}, nil
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
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

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

// InsertDiagnostics implements WorkerJobStore (same atomic fence/stamping as InsertFindings).
func (s *PostgresStore) InsertDiagnostics(ctx context.Context, jobID, workerID string, attempt int, diagnostics []JobDiagnostic) error {
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
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

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

// FailJob implements WorkerJobStore.
func (s *PostgresStore) FailJob(ctx context.Context, jobID, workerID string, attempt int, errMsg string, finishedAt time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs SET status = $1, error = $2, finished_at = $3
		WHERE id = $4 AND status = $5 AND claimed_by = $6 AND attempt = $7`,
		string(JobStatusFailed), errMsg, finishedAt, jobID,
		string(JobStatusRunning), workerID, attempt,
	)
	if err != nil {
		return fmt.Errorf("coachapi: fail job %q: %w", jobID, err)
	}
	if tag.RowsAffected() == 0 {
		return s.fenceFailure(ctx, jobID)
	}
	return nil
}

// ReleaseClaim implements WorkerJobStore.
func (s *PostgresStore) ReleaseClaim(ctx context.Context, jobID, workerID string, attempt int) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs
		SET status = $1, claimed_by = NULL, heartbeat_at = NULL
		WHERE id = $2 AND status = $3 AND claimed_by = $4 AND attempt = $5`,
		string(JobStatusQueued), jobID, string(JobStatusRunning), workerID, attempt,
	)
	if err != nil {
		return fmt.Errorf("coachapi: release claim %q: %w", jobID, err)
	}
	if tag.RowsAffected() == 0 {
		return s.fenceFailure(ctx, jobID)
	}
	return nil
}

// ListQueuedOlderThan implements WorkerJobStore.
func (s *PostgresStore) ListQueuedOlderThan(ctx context.Context, olderThan time.Time) ([]Job, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, kind, params, status, error, created_at, started_at,
			finished_at, claimed_by, heartbeat_at, attempt,
			created_by_provider, created_by_subject, created_by_login
		FROM jobs
		WHERE status = $1 AND created_at < $2
		ORDER BY created_at ASC`,
		string(JobStatusQueued), olderThan,
	)
	if err != nil {
		return nil, fmt.Errorf("coachapi: list queued older than: %w", err)
	}
	defer rows.Close()

	var out []Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("coachapi: list queued older than: %w", err)
	}
	return out, nil
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

// fenceHoldForTest, when non-nil, runs before atomic fenced inserts so
// acceptance tests can interleave ClaimJob reclaim. nil outside tests.
var fenceHoldForTest func()

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

var _ WorkerJobStore = (*PostgresStore)(nil)

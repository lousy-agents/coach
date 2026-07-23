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

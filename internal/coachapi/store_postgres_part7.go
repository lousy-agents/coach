package coachapi

import (
	"context"

	"fmt"

	"time"
)

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

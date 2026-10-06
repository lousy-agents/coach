package postgres

import (
	"context"
	"fmt"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// findingsForReport returns the findings for job id's final recorded
// attempt (jobs.attempt, as set by the completing RecordCompletion call),
// ordered by created_at so report assembly is deterministic. Rows from a
// discarded, non-final attempt are intentionally excluded.
func (s *Store) findingsForReport(ctx context.Context, jobID string) ([]coachapi.JobFinding, error) {
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

	var findings []coachapi.JobFinding
	for rows.Next() {
		var (
			f      coachapi.JobFinding
			source string
		)
		if err := rows.Scan(
			&f.ID, &f.JobID, &f.Attempt, &source, &f.RubricID,
			&f.RubricVersion, &f.ModelIdentity, &f.Payload, &f.PayloadHash,
			&f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("coachapi: get report: scanning finding: %w", err)
		}
		f.Source = coachapi.FindingSource(source)
		findings = append(findings, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("coachapi: get report: iterating findings: %w", err)
	}
	return findings, nil
}

func (s *Store) diagnosticsForReport(ctx context.Context, jobID string) ([]coachapi.JobDiagnostic, error) {
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

	var diagnostics []coachapi.JobDiagnostic
	for rows.Next() {
		var d coachapi.JobDiagnostic
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

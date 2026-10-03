package coachapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

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

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// GetReport returns ErrJobNotFound-wrapped both when id does not exist and
// when it exists but has never had a successful RecordCompletion (detected
// via generated_at IS NULL, which RecordCompletion sets atomically with
// every other report-assembly column), since normal callers check
// Job.Status via GetJob before calling GetReport.
func (s *Store) GetReport(ctx context.Context, id string) (coachapi.Report, error) {
	var (
		report      coachapi.Report
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
			return coachapi.Report{}, fmt.Errorf("coachapi: report for job %q: %w", id, coachapi.ErrJobNotFound)
		}
		return coachapi.Report{}, fmt.Errorf("coachapi: get report: %w", err)
	}
	report.ReportVersion = coachapi.ReportVersion1
	report.JobID = id
	report.Kind = coachapi.JobKind(kind)
	if err := json.Unmarshal(versionsRaw, &report.Versions); err != nil {
		return coachapi.Report{}, fmt.Errorf("coachapi: get report: decoding report_versions: %w", err)
	}

	findings, err := s.findingsForReport(ctx, id)
	if err != nil {
		return coachapi.Report{}, err
	}
	diagnostics, err := s.diagnosticsForReport(ctx, id)
	if err != nil {
		return coachapi.Report{}, err
	}
	report.Summary = coachapi.SummarizeFindings(findings)
	report.Findings = coachapi.ReportFindings(findings)
	report.Diagnostics = coachapi.ReportDiagnostics(diagnostics)

	return report, nil
}

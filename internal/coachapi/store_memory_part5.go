package coachapi

import (
	"context"
	"encoding/json"
	"fmt"
)

func (m *MemoryStore) CreateJob(ctx context.Context, job Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.jobs[job.ID]; exists {
		return fmt.Errorf("coachapi: job %q already exists", job.ID)
	}
	m.jobs[job.ID] = &memoryJobRecord{job: cloneJob(job)}
	return nil
}
func (m *MemoryStore) GetJob(ctx context.Context, id string) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[id]
	if !ok {
		return Job{}, fmt.Errorf("coachapi: job %q: %w", id, ErrJobNotFound)
	}
	return cloneJob(record.job), nil
}
func findingRuleID(payload json.RawMessage) string {
	var decoded struct {
		RuleID string `json:"rule_id"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return ""
	}
	return decoded.RuleID
}
func toReportFindings(jobFindings []JobFinding) []Finding {
	out := make([]Finding, len(jobFindings))
	for i, f := range jobFindings {
		out[i] = Finding{
			Source:        f.Source,
			RubricID:      clonePtrString(f.RubricID),
			RubricVersion: clonePtrString(f.RubricVersion),
			ModelIdentity: clonePtrString(f.ModelIdentity),
			Payload:       cloneRawMessage(f.Payload),
		}
	}
	return out
}
func toReportDiagnostics(jobDiagnostics []JobDiagnostic) []Diagnostic {
	out := make([]Diagnostic, len(jobDiagnostics))
	for i, d := range jobDiagnostics {
		out[i] = Diagnostic{Scope: d.Scope, Message: d.Message}
	}
	return out
}

// GetReport returns ErrJobNotFound-wrapped both when id does not exist and
// when it exists but has never had a successful RecordCompletion/CompleteJob,
// since normal callers check Job.Status via GetJob before calling GetReport.
// Findings/diagnostics are those tagged with the job's final attempt only.
func (m *MemoryStore) GetReport(ctx context.Context, id string) (Report, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[id]
	if !ok || !record.completed {
		return Report{}, fmt.Errorf("coachapi: report for job %q: %w", id, ErrJobNotFound)
	}

	findings := findingsForAttempt(record.findings, record.job.Attempt)
	diagnostics := diagnosticsForAttempt(record.diagnostics, record.job.Attempt)

	return Report{
		ReportVersion: ReportVersion1,
		JobID:         record.job.ID,
		Kind:          record.job.Kind,
		Params:        cloneRawMessage(record.job.Params),
		CommitSHA:     record.commitSHA,
		Summary:       summarizeFindings(findings),
		Findings:      toReportFindings(findings),
		Diagnostics:   toReportDiagnostics(diagnostics),
		Error:         clonePtrString(record.job.Error),
		Versions:      cloneVersions(record.versions),
		GeneratedAt:   record.generatedAt,
	}, nil
}
func cloneJobDiagnostics(diagnostics []JobDiagnostic) []JobDiagnostic {
	if diagnostics == nil {
		return nil
	}
	out := make([]JobDiagnostic, len(diagnostics))
	copy(out, diagnostics)
	return out
}
func (m *MemoryStore) RecordCompletion(ctx context.Context, jobID string, completion Completion) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}

	finishedAt := completion.FinishedAt
	record.job.Status = JobStatusCompleted
	record.job.Attempt = completion.Attempt
	record.job.FinishedAt = &finishedAt
	record.job.Error = nil

	record.commitSHA = completion.CommitSHA
	record.versions = cloneVersions(completion.Versions)
	record.generatedAt = completion.GeneratedAt
	record.findings = cloneJobFindings(completion.Findings)
	record.diagnostics = cloneJobDiagnostics(completion.Diagnostics)
	record.completed = true

	return nil
}
func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	out := make(json.RawMessage, len(raw))
	copy(out, raw)
	return out
}
func clonePtrString(p *string) *string {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

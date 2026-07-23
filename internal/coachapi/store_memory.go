package coachapi

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// MemoryStore is an in-process, non-durable JobStore for local development
// and tests. It is safe for concurrent use.
type MemoryStore struct {
	mu   sync.Mutex
	jobs map[string]*memoryJobRecord
}

var _ JobStore = (*MemoryStore)(nil)

// memoryJobRecord holds a job row plus the attempt data needed to assemble
// its Report once RecordCompletion has run.
type memoryJobRecord struct {
	job         Job
	completed   bool
	commitSHA   string
	findings    []JobFinding
	diagnostics []JobDiagnostic
	versions    ReportVersions
	generatedAt time.Time
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{jobs: make(map[string]*memoryJobRecord)}
}

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

// GetReport returns ErrJobNotFound-wrapped both when id does not exist and
// when it exists but has never had a successful RecordCompletion, since
// normal callers check Job.Status via GetJob before calling GetReport.
func (m *MemoryStore) GetReport(ctx context.Context, id string) (Report, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[id]
	if !ok || !record.completed {
		return Report{}, fmt.Errorf("coachapi: report for job %q: %w", id, ErrJobNotFound)
	}

	return Report{
		ReportVersion: ReportVersion1,
		JobID:         record.job.ID,
		Kind:          record.job.Kind,
		Params:        cloneRawMessage(record.job.Params),
		CommitSHA:     record.commitSHA,
		Summary:       summarizeFindings(record.findings),
		Findings:      toReportFindings(record.findings),
		Diagnostics:   toReportDiagnostics(record.diagnostics),
		Error:         clonePtrString(record.job.Error),
		Versions:      cloneVersions(record.versions),
		GeneratedAt:   record.generatedAt,
	}, nil
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

func (m *MemoryStore) RecordFailure(ctx context.Context, jobID string, errMsg string, finishedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}

	record.job.Status = JobStatusFailed
	record.job.Error = &errMsg
	finished := finishedAt
	record.job.FinishedAt = &finished

	return nil
}

// summarizeFindings groups findings by source then rule id (deterministic,
// parsed from the finding payload's rule_id field) or rubric id (agent,
// from the finding's own RubricID) so counts cannot collide across sources.
func summarizeFindings(findings []JobFinding) ReportSummary {
	counts := map[string]map[string]int{}
	for _, f := range findings {
		var key string
		if f.Source == FindingSourceAgent {
			if f.RubricID != nil {
				key = *f.RubricID
			}
		} else {
			key = findingRuleID(f.Payload)
		}
		if key == "" {
			continue
		}
		source := string(f.Source)
		if counts[source] == nil {
			counts[source] = map[string]int{}
		}
		counts[source][key]++
	}
	return ReportSummary{FindingCounts: counts}
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

func cloneJob(j Job) Job {
	out := j
	out.Params = cloneRawMessage(j.Params)
	out.Error = clonePtrString(j.Error)
	out.StartedAt = cloneTimePtr(j.StartedAt)
	out.FinishedAt = cloneTimePtr(j.FinishedAt)
	out.ClaimedBy = clonePtrString(j.ClaimedBy)
	out.HeartbeatAt = cloneTimePtr(j.HeartbeatAt)
	return out
}

func cloneJobFindings(findings []JobFinding) []JobFinding {
	if findings == nil {
		return nil
	}
	out := make([]JobFinding, len(findings))
	for i, f := range findings {
		out[i] = f
		out[i].RubricID = clonePtrString(f.RubricID)
		out[i].RubricVersion = clonePtrString(f.RubricVersion)
		out[i].ModelIdentity = clonePtrString(f.ModelIdentity)
		out[i].Payload = cloneRawMessage(f.Payload)
	}
	return out
}

func cloneJobDiagnostics(diagnostics []JobDiagnostic) []JobDiagnostic {
	if diagnostics == nil {
		return nil
	}
	out := make([]JobDiagnostic, len(diagnostics))
	copy(out, diagnostics)
	return out
}

func cloneVersions(v ReportVersions) ReportVersions {
	out := v
	if v.Rubrics != nil {
		out.Rubrics = make(map[string]string, len(v.Rubrics))
		for k, val := range v.Rubrics {
			out.Rubrics[k] = val
		}
	}
	return out
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

func cloneTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := *t
	return &v
}

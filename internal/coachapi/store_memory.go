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

// ClaimJob implements WorkerJobStore.
func (m *MemoryStore) ClaimJob(ctx context.Context, jobID, workerID string, now time.Time, staleAfter time.Duration) (ClaimLease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return ClaimLease{}, fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}

	switch record.job.Status {
	case JobStatusQueued:
		// claimable
	case JobStatusRunning:
		// Reclaim only when heartbeat is missing or older than staleAfter.
		if record.job.HeartbeatAt != nil && now.Sub(*record.job.HeartbeatAt) < staleAfter {
			return ClaimLease{}, fmt.Errorf("coachapi: job %q: %w", jobID, ErrNotClaimable)
		}
	default:
		return ClaimLease{}, fmt.Errorf("coachapi: job %q: %w", jobID, ErrNotClaimable)
	}

	record.job.Attempt++
	record.job.Status = JobStatusRunning
	wb := workerID
	record.job.ClaimedBy = &wb
	hb := now
	record.job.HeartbeatAt = &hb
	st := now
	record.job.StartedAt = &st
	record.job.FinishedAt = nil
	record.job.Error = nil
	record.findings = nil
	record.diagnostics = nil
	record.completed = false
	record.commitSHA = ""
	record.versions = ReportVersions{}
	record.generatedAt = time.Time{}

	return ClaimLease{
		JobID:     jobID,
		WorkerID:  workerID,
		Attempt:   record.job.Attempt,
		StartedAt: now,
	}, nil
}

// Heartbeat implements WorkerJobStore.
func (m *MemoryStore) Heartbeat(ctx context.Context, jobID, workerID string, attempt int, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}
	if !fenceMatches(record.job, workerID, attempt) {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrClaimLost)
	}
	hb := now
	record.job.HeartbeatAt = &hb
	return nil
}

// InsertFindings implements WorkerJobStore.
func (m *MemoryStore) InsertFindings(ctx context.Context, jobID, workerID string, attempt int, findings []JobFinding) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}
	if !fenceMatches(record.job, workerID, attempt) {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrClaimLost)
	}
	stamped := cloneJobFindings(findings)
	for i := range stamped {
		stamped[i].JobID = jobID
		stamped[i].Attempt = attempt
	}
	record.findings = append(record.findings, stamped...)
	return nil
}

// InsertDiagnostics implements WorkerJobStore.
func (m *MemoryStore) InsertDiagnostics(ctx context.Context, jobID, workerID string, attempt int, diagnostics []JobDiagnostic) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}
	if !fenceMatches(record.job, workerID, attempt) {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrClaimLost)
	}
	stamped := cloneJobDiagnostics(diagnostics)
	for i := range stamped {
		stamped[i].JobID = jobID
		stamped[i].Attempt = attempt
	}
	record.diagnostics = append(record.diagnostics, stamped...)
	return nil
}

// CompleteJob implements WorkerJobStore.
func (m *MemoryStore) CompleteJob(ctx context.Context, jobID, workerID string, attempt int, completion Completion) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}
	if !fenceMatches(record.job, workerID, attempt) {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrClaimLost)
	}

	finishedAt := completion.FinishedAt
	record.job.Status = JobStatusCompleted
	record.job.Attempt = attempt
	record.job.FinishedAt = &finishedAt
	record.job.Error = nil
	record.commitSHA = completion.CommitSHA
	record.versions = cloneVersions(completion.Versions)
	record.generatedAt = completion.GeneratedAt
	if len(completion.Findings) > 0 {
		stamped := cloneJobFindings(completion.Findings)
		for i := range stamped {
			stamped[i].JobID = jobID
			stamped[i].Attempt = attempt
		}
		record.findings = append(record.findings, stamped...)
	}
	if len(completion.Diagnostics) > 0 {
		stamped := cloneJobDiagnostics(completion.Diagnostics)
		for i := range stamped {
			stamped[i].JobID = jobID
			stamped[i].Attempt = attempt
		}
		record.diagnostics = append(record.diagnostics, stamped...)
	}
	record.completed = true
	return nil
}

// FailJob implements WorkerJobStore.
func (m *MemoryStore) FailJob(ctx context.Context, jobID, workerID string, attempt int, errMsg string, finishedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}
	if !fenceMatches(record.job, workerID, attempt) {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrClaimLost)
	}

	record.job.Status = JobStatusFailed
	record.job.Error = &errMsg
	finished := finishedAt
	record.job.FinishedAt = &finished
	return nil
}

// ReleaseClaim implements WorkerJobStore.
func (m *MemoryStore) ReleaseClaim(ctx context.Context, jobID, workerID string, attempt int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrJobNotFound)
	}
	if !fenceMatches(record.job, workerID, attempt) {
		return fmt.Errorf("coachapi: job %q: %w", jobID, ErrClaimLost)
	}

	record.job.Status = JobStatusQueued
	record.job.ClaimedBy = nil
	record.job.HeartbeatAt = nil
	return nil
}

// ListQueuedOlderThan implements WorkerJobStore.
func (m *MemoryStore) ListQueuedOlderThan(ctx context.Context, olderThan time.Time) ([]Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []Job
	for _, record := range m.jobs {
		if record.job.Status == JobStatusQueued && record.job.CreatedAt.Before(olderThan) {
			out = append(out, cloneJob(record.job))
		}
	}
	return out, nil
}

// ReleaseStaleRunning implements WorkerJobStore.
func (m *MemoryStore) ReleaseStaleRunning(ctx context.Context, now time.Time, staleAfter time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	released := 0
	for _, record := range m.jobs {
		if record.job.Status != JobStatusRunning {
			continue
		}
		if record.job.HeartbeatAt != nil && now.Sub(*record.job.HeartbeatAt) < staleAfter {
			continue
		}
		record.job.Status = JobStatusQueued
		record.job.ClaimedBy = nil
		record.job.HeartbeatAt = nil
		released++
	}
	return released, nil
}

func fenceMatches(job Job, workerID string, attempt int) bool {
	return job.Status == JobStatusRunning &&
		job.ClaimedBy != nil &&
		*job.ClaimedBy == workerID &&
		job.Attempt == attempt
}

func findingsForAttempt(findings []JobFinding, attempt int) []JobFinding {
	var out []JobFinding
	for _, f := range findings {
		if f.Attempt == attempt {
			out = append(out, f)
		}
	}
	return out
}

func diagnosticsForAttempt(diagnostics []JobDiagnostic, attempt int) []JobDiagnostic {
	var out []JobDiagnostic
	for _, d := range diagnostics {
		if d.Attempt == attempt {
			out = append(out, d)
		}
	}
	return out
}

var _ WorkerJobStore = (*MemoryStore)(nil)

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

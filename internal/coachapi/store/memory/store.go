// Package memory is the in-process coachapi.WorkerJobStore used for local
// development and tests.
package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// Store is an in-process, non-durable JobStore for local development
// and tests. It is safe for concurrent use.
type Store struct {
	mu   sync.Mutex
	jobs map[string]*jobRecord
}

var _ coachapi.JobStore = (*Store)(nil)

var _ coachapi.WorkerJobStore = (*Store)(nil)

// jobRecord holds a job row plus the attempt data needed to assemble
// its Report once RecordCompletion has run.
type jobRecord struct {
	job         coachapi.Job
	completed   bool
	commitSHA   string
	findings    []coachapi.JobFinding
	diagnostics []coachapi.JobDiagnostic
	versions    coachapi.ReportVersions
	generatedAt time.Time
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{jobs: make(map[string]*jobRecord)}
}

func (m *Store) CreateJob(ctx context.Context, job coachapi.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.jobs[job.ID]; exists {
		return fmt.Errorf("coachapi: job %q already exists", job.ID)
	}
	m.jobs[job.ID] = &jobRecord{job: cloneJob(job)}
	return nil
}

func (m *Store) GetJob(ctx context.Context, id string) (coachapi.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[id]
	if !ok {
		return coachapi.Job{}, fmt.Errorf("coachapi: job %q: %w", id, coachapi.ErrJobNotFound)
	}
	return cloneJob(record.job), nil
}

// GetReport returns ErrJobNotFound-wrapped both when id does not exist and
// when it exists but has never had a successful RecordCompletion/CompleteJob,
// since normal callers check Job.Status via GetJob before calling GetReport.
// Findings/diagnostics are those tagged with the job's final attempt only.
func (m *Store) GetReport(ctx context.Context, id string) (coachapi.Report, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[id]
	if !ok || !record.completed {
		return coachapi.Report{}, fmt.Errorf("coachapi: report for job %q: %w", id, coachapi.ErrJobNotFound)
	}

	findings := findingsForAttempt(record.findings, record.job.Attempt)
	diagnostics := diagnosticsForAttempt(record.diagnostics, record.job.Attempt)

	return coachapi.Report{
		ReportVersion: coachapi.ReportVersion1,
		JobID:         record.job.ID,
		Kind:          record.job.Kind,
		Params:        cloneRawMessage(record.job.Params),
		CommitSHA:     record.commitSHA,
		Summary:       coachapi.SummarizeFindings(findings),
		Findings:      coachapi.ReportFindings(findings),
		Diagnostics:   coachapi.ReportDiagnostics(diagnostics),
		Error:         cloneOptional(record.job.Error),
		Versions:      cloneVersions(record.versions),
		GeneratedAt:   record.generatedAt,
	}, nil
}

func (m *Store) RecordCompletion(ctx context.Context, jobID string, completion coachapi.Completion) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrJobNotFound)
	}

	finishedAt := completion.FinishedAt
	record.job.Status = coachapi.JobStatusCompleted
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

func (m *Store) RecordFailure(ctx context.Context, jobID string, errMsg string, finishedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrJobNotFound)
	}

	record.job.Status = coachapi.JobStatusFailed
	record.job.Error = &errMsg
	finished := finishedAt
	record.job.FinishedAt = &finished

	return nil
}

func findingsForAttempt(findings []coachapi.JobFinding, attempt int) []coachapi.JobFinding {
	var out []coachapi.JobFinding
	for _, f := range findings {
		if f.Attempt == attempt {
			out = append(out, f)
		}
	}
	return out
}

func diagnosticsForAttempt(diagnostics []coachapi.JobDiagnostic, attempt int) []coachapi.JobDiagnostic {
	var out []coachapi.JobDiagnostic
	for _, d := range diagnostics {
		if d.Attempt == attempt {
			out = append(out, d)
		}
	}
	return out
}

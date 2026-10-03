package coachapi

import (
	"context"

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

// GetReport returns ErrJobNotFound-wrapped both when id does not exist and
// when it exists but has never had a successful RecordCompletion/CompleteJob,
// since normal callers check Job.Status via GetJob before calling GetReport.
// Findings/diagnostics are those tagged with the job's final attempt only.

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

// claimable

// Reclaim only when heartbeat is missing or older than staleAfter.

// Heartbeat implements WorkerJobStore.

// InsertFindings implements WorkerJobStore.

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

// ReleaseClaim implements WorkerJobStore.

// ListQueuedOlderThan implements WorkerJobStore.

// ReleaseStaleRunning implements WorkerJobStore.

func fenceMatches(job Job, workerID string, attempt int) bool {
	return job.Status == JobStatusRunning &&
		job.ClaimedBy != nil &&
		*job.ClaimedBy == workerID &&
		job.Attempt == attempt
}

var _ WorkerJobStore = (*MemoryStore)(nil)

// summarizeFindings groups findings by source then rule id (deterministic,
// parsed from the finding payload's rule_id field) or rubric id (agent,
// from the finding's own RubricID) so counts cannot collide across sources.

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

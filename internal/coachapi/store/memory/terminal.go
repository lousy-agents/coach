package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// CompleteJob implements WorkerJobStore.
func (m *Store) CompleteJob(ctx context.Context, jobID, workerID string, attempt int, completion coachapi.Completion) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrJobNotFound)
	}
	if !fenceMatches(record.job, workerID, attempt) {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrClaimLost)
	}

	finishedAt := completion.FinishedAt
	record.job.Status = coachapi.JobStatusCompleted
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
func (m *Store) FailJob(ctx context.Context, jobID, workerID string, attempt int, errMsg string, finishedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrJobNotFound)
	}
	if !fenceMatches(record.job, workerID, attempt) {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrClaimLost)
	}

	record.job.Status = coachapi.JobStatusFailed
	record.job.Error = &errMsg
	finished := finishedAt
	record.job.FinishedAt = &finished
	return nil
}

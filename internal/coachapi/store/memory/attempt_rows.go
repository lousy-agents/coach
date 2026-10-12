package memory

import (
	"context"
	"fmt"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// InsertFindings implements WorkerJobStore.
func (m *Store) InsertFindings(ctx context.Context, jobID, workerID string, attempt int, findings []coachapi.JobFinding) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrJobNotFound)
	}
	if !fenceMatches(record.job, workerID, attempt) {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrClaimLost)
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
func (m *Store) InsertDiagnostics(ctx context.Context, jobID, workerID string, attempt int, diagnostics []coachapi.JobDiagnostic) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrJobNotFound)
	}
	if !fenceMatches(record.job, workerID, attempt) {
		return fmt.Errorf("coachapi: job %q: %w", jobID, coachapi.ErrClaimLost)
	}
	stamped := cloneJobDiagnostics(diagnostics)
	for i := range stamped {
		stamped[i].JobID = jobID
		stamped[i].Attempt = attempt
	}
	record.diagnostics = append(record.diagnostics, stamped...)
	return nil
}

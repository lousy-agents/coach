package coachapi

import (
	"context"

	"fmt"

	"time"
)

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

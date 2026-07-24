package worker

import (
	"context"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// JobWriter is the fenced persistence surface a JobHandler uses while holding
// a claim. Every method is conditional on the handler's ClaimLease.
type JobWriter interface {
	Lease() coachapi.ClaimLease
	InsertFindings(ctx context.Context, findings []coachapi.JobFinding) error
	InsertDiagnostics(ctx context.Context, diagnostics []coachapi.JobDiagnostic) error
}

// JobHandler runs one claimed job attempt. On success it returns a non-nil
// Completion (findings may already have been written via JobWriter). On
// failure it returns a non-nil error:
//   - Retryable(err) below MaxAttempts → ReleaseClaim + Nack(false)
//   - permanent error, or retryable at/above MaxAttempts → FailJob + Nack(true)
//   - errors.Is(err, coachapi.ErrClaimLost) → abandon without ack/nack
type JobHandler func(ctx context.Context, job coachapi.Job, w JobWriter) (*coachapi.Completion, error)

type leaseWriter struct {
	store coachapi.WorkerJobStore
	lease coachapi.ClaimLease
}

func (w *leaseWriter) Lease() coachapi.ClaimLease { return w.lease }

func (w *leaseWriter) InsertFindings(ctx context.Context, findings []coachapi.JobFinding) error {
	stamped := append([]coachapi.JobFinding(nil), findings...)
	for i := range stamped {
		stamped[i].JobID = w.lease.JobID
		stamped[i].Attempt = w.lease.Attempt
	}
	return w.store.InsertFindings(ctx, w.lease.JobID, w.lease.WorkerID, w.lease.Attempt, stamped)
}

func (w *leaseWriter) InsertDiagnostics(ctx context.Context, diagnostics []coachapi.JobDiagnostic) error {
	stamped := append([]coachapi.JobDiagnostic(nil), diagnostics...)
	for i := range stamped {
		stamped[i].JobID = w.lease.JobID
		stamped[i].Attempt = w.lease.Attempt
	}
	return w.store.InsertDiagnostics(ctx, w.lease.JobID, w.lease.WorkerID, w.lease.Attempt, stamped)
}

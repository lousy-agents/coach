package coachapi

import (
	"context"
	"errors"
	"time"
)

// ErrClaimLost is returned by fenced worker writes (Heartbeat, InsertFindings,
// InsertDiagnostics, CompleteJob, FailJob) when (claimed_by, attempt) no
// longer match the caller's lease. The worker must abandon the job without
// acking its queue message.
var ErrClaimLost = errors.New("coachapi: claim lost")

// ErrNotClaimable is returned by ClaimJob when the job exists but is not in a
// claimable state (not queued, and not a stale running job).
var ErrNotClaimable = errors.New("coachapi: job not claimable")

// ClaimLease is the worker's exclusive right to process one attempt of a job
// after a successful ClaimJob. Attempt is the jobs.attempt value assigned by
// that claim (1-based after the first successful claim).
type ClaimLease struct {
	JobID     string
	WorkerID  string
	Attempt   int
	StartedAt time.Time
}

// WorkerJobStore is the persistence seam coach-worker depends on: JobStore
// plus claim/heartbeat/fenced-write and reconciler listing. MemoryStore and
// PostgresStore implement it; HTTP handlers continue to use JobStore only.
type WorkerJobStore interface {
	JobStore

	// ClaimJob claims a queued job, or reclaims a running job whose
	// heartbeat is older than staleAfter (relative to now). On success it
	// increments attempt, deletes prior job_findings/job_diagnostics for the
	// job, sets status=running, claimed_by, heartbeat_at, and started_at, and
	// returns the new lease. Returns ErrJobNotFound or ErrNotClaimable.
	ClaimJob(ctx context.Context, jobID, workerID string, now time.Time, staleAfter time.Duration) (ClaimLease, error)

	// Heartbeat updates heartbeat_at when (claimed_by, attempt) match.
	// Returns ErrClaimLost on fence mismatch.
	Heartbeat(ctx context.Context, jobID, workerID string, attempt int, now time.Time) error

	// InsertFindings appends findings when (claimed_by, attempt) match.
	// Implementations stamp each finding's JobID and Attempt from the lease
	// args (caller-supplied values are ignored). Returns ErrClaimLost on
	// fence mismatch.
	InsertFindings(ctx context.Context, jobID, workerID string, attempt int, findings []JobFinding) error

	// InsertDiagnostics appends diagnostics when (claimed_by, attempt) match.
	// Implementations stamp each diagnostic's JobID and Attempt from the
	// lease args (caller-supplied values are ignored). Returns ErrClaimLost
	// on fence mismatch.
	InsertDiagnostics(ctx context.Context, jobID, workerID string, attempt int, diagnostics []JobDiagnostic) error

	// CompleteJob fenced-terminal-transitions the job to completed and
	// records report assembly fields. Findings already inserted for this
	// attempt (via InsertFindings) are kept; completion.Findings are also
	// inserted when non-empty. Returns ErrClaimLost on fence mismatch.
	CompleteJob(ctx context.Context, jobID, workerID string, attempt int, completion Completion) error

	// FailJob fenced-terminal-transitions the job to failed.
	// Returns ErrClaimLost on fence mismatch.
	FailJob(ctx context.Context, jobID, workerID string, attempt int, errMsg string, finishedAt time.Time) error

	// ListQueuedOlderThan returns queued jobs with created_at strictly before
	// olderThan, for the requeue reconciler.
	ListQueuedOlderThan(ctx context.Context, olderThan time.Time) ([]Job, error)

	// ReleaseStaleRunning sets running jobs whose heartbeat is older than
	// staleAfter (relative to now) back to queued and clears claim fields so
	// the requeue reconciler can re-enqueue them. It does not increment
	// attempt (ClaimJob does that on the next successful claim).
	ReleaseStaleRunning(ctx context.Context, now time.Time, staleAfter time.Duration) (released int, err error)
}

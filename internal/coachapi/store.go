package coachapi

import (
	"context"
	"errors"
	"time"
)

// ErrJobNotFound is returned (wrapped, so errors.Is works) by JobStore
// methods that look up a job or report by id when the id does not exist.
// Any other non-nil error from a JobStore method is a store failure and
// callers must fail closed (503), not treat it as "not found".
var ErrJobNotFound = errors.New("coachapi: job not found")

// Completion is the input to JobStore.RecordCompletion: everything needed to
// finalize one successful job attempt and later assemble its Report.
type Completion struct {
	Attempt     int
	CommitSHA   string
	Findings    []JobFinding
	Diagnostics []JobDiagnostic
	Versions    ReportVersions
	FinishedAt  time.Time
	GeneratedAt time.Time
}

// JobStore is the persistence seam POST/GET /v1/jobs handlers depend on. It
// has an in-memory implementation (store_memory.go) and, in a later task, a
// Postgres implementation (store_postgres.go). No method may leak an
// in-memory-only detail (e.g. no method returns a pointer into internal map
// storage) — implementations must return values a caller cannot use to
// mutate the store's internal state.
type JobStore interface {
	// CreateJob persists a new job row. Callers set ID, Kind, Params,
	// Status (must be JobStatusQueued), CreatedAt, Attempt (must be 0), and
	// CreatedByProvider/Subject/Login before calling; CreateJob does not
	// default or mutate them. Returns an error (never ErrJobNotFound) on
	// store failure, including a duplicate ID. Does not enqueue on any
	// TaskQueue — that is the HTTP handler's job, per the submit-durability
	// rule (persist, then enqueue, then 202).
	CreateJob(ctx context.Context, job Job) error

	// GetJob returns the current row for id, or an error wrapping
	// ErrJobNotFound.
	GetJob(ctx context.Context, id string) (Job, error)

	// GetReport assembles and returns the Report for a completed job.
	// Callers (the HTTP handler) are responsible for checking
	// Job.Status == JobStatusCompleted first (via GetJob) and returning 409
	// job_not_completed themselves. Calling GetReport for a job id that
	// does not exist, or that has never had a successful RecordCompletion,
	// both return an error wrapping ErrJobNotFound; normal callers won't
	// hit the latter case because they check Job.Status first.
	GetReport(ctx context.Context, id string) (Report, error)

	// RecordCompletion finalizes one attempt of a job as successful: sets
	// status=completed, attempt=completion.Attempt,
	// finished_at=completion.FinishedAt, and durably records
	// completion.Findings/completion.Diagnostics (each already stamped
	// with completion.Attempt by the caller) plus the report-level
	// CommitSHA/Versions/GeneratedAt needed to assemble the Report later.
	// Returns an error wrapping ErrJobNotFound if the job id does not
	// exist.
	RecordCompletion(ctx context.Context, jobID string, completion Completion) error

	// RecordFailure finalizes a job attempt as failed: sets status=failed,
	// finished_at=finishedAt, error=errMsg. Returns an error wrapping
	// ErrJobNotFound if the job id does not exist.
	RecordFailure(ctx context.Context, jobID string, errMsg string, finishedAt time.Time) error
}

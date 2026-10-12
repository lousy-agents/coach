// Package coachapi is the coach platform's job domain: job, finding, and
// report types; the stable API contract (error codes and envelope); the
// authenticated Principal; and the JobStore and WorkerJobStore persistence
// ports. Adapters live in subpackages that depend on this one: httpapi
// (the /v1/jobs HTTP surface), store/memory and store/postgres (the ports'
// implementations), queue (the TaskQueue port and its adapters), baseline
// (the repo_baseline_scan use case), and worker (claim/heartbeat/settle).
package coachapi

import (
	"encoding/json"
	"time"
)

// JobKind identifies a supported async job type.
type JobKind string

const (
	JobKindRepoBaselineScan JobKind = "repo_baseline_scan"
)

// JobStatus is the lifecycle state of a job row.
type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

// FindingSource distinguishes deterministic analysis from agent judgments.
type FindingSource string

const (
	FindingSourceDeterministic FindingSource = "deterministic"
	FindingSourceAgent         FindingSource = "agent"
)

// RepoBaselineScanParams is the submit-time params schema for repo_baseline_scan.
// There is no client-supplied clone URL field; git_url/clone_url must be rejected at the API boundary.
type RepoBaselineScanParams struct {
	RepoOwner string `json:"repo_owner"`
	RepoName  string `json:"repo_name"`
	Ref       string `json:"ref,omitempty"`
}

// Job is the persisted jobs row (domain model, not the HTTP status view).
type Job struct {
	ID                string          `json:"id"`
	Kind              JobKind         `json:"kind"`
	Params            json.RawMessage `json:"params"`
	Status            JobStatus       `json:"status"`
	Error             *string         `json:"error"`
	CreatedAt         time.Time       `json:"created_at"`
	StartedAt         *time.Time      `json:"started_at,omitempty"`
	FinishedAt        *time.Time      `json:"finished_at,omitempty"`
	ClaimedBy         *string         `json:"claimed_by,omitempty"`
	HeartbeatAt       *time.Time      `json:"heartbeat_at,omitempty"`
	Attempt           int             `json:"attempt"`
	CreatedByProvider string          `json:"created_by_provider"`
	CreatedBySubject  string          `json:"created_by_subject"`
	CreatedByLogin    string          `json:"created_by_login"`
}

// JobFinding is one attempt-scoped finding row.
// For Source=deterministic, RubricID, RubricVersion, and ModelIdentity are null.
type JobFinding struct {
	ID            string          `json:"id"`
	JobID         string          `json:"job_id"`
	Attempt       int             `json:"attempt"`
	Source        FindingSource   `json:"source"`
	RubricID      *string         `json:"rubric_id"`
	RubricVersion *string         `json:"rubric_version"`
	ModelIdentity *string         `json:"model_identity"`
	Payload       json.RawMessage `json:"payload"`
	PayloadHash   string          `json:"payload_hash"`
	CreatedAt     time.Time       `json:"created_at"`
}

// JobDiagnostic is one attempt-scoped diagnostic row.
type JobDiagnostic struct {
	ID        string    `json:"id"`
	JobID     string    `json:"job_id"`
	Attempt   int       `json:"attempt"`
	Scope     string    `json:"scope"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

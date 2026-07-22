package coachapi

import (
	"encoding/json"
	"time"
)

// ReportVersion1 is the frozen groundwork-era report_version value.
const ReportVersion1 = "1"

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

// Stable machine-readable API error codes (Story 1).
const (
	ErrorCodeUnauthenticated    = "unauthenticated"
	ErrorCodeUnauthorized       = "unauthorized"
	ErrorCodeInvalidRequest     = "invalid_request"
	ErrorCodeJobNotFound        = "job_not_found"
	ErrorCodeJobNotCompleted    = "job_not_completed"
	ErrorCodeUnsupportedJobKind = "unsupported_job_kind"
	ErrorCodeRepoNotAuthorized  = "repo_not_authorized"
	ErrorCodeNotFound           = "not_found"
	ErrorCodeInternalError      = "internal_error"
)

// Principal is the only identity type job handlers and authz checks consume.
// v1 GitHub maps Provider="github", Subject=<numeric user id string>, Login=<github login>.
type Principal struct {
	Provider string `json:"provider"`
	Subject  string `json:"subject"`
	Login    string `json:"login"`
}

// RepoBaselineScanParams is the submit-time params schema for repo_baseline_scan.
// There is no client-supplied clone URL field; git_url/clone_url must be rejected at the API boundary.
type RepoBaselineScanParams struct {
	RepoOwner string `json:"repo_owner"`
	RepoName  string `json:"repo_name"`
	Ref       string `json:"ref,omitempty"`
}

// CreateJobRequest is the body of POST /v1/jobs.
type CreateJobRequest struct {
	Kind   JobKind         `json:"kind"`
	Params json.RawMessage `json:"params"`
}

// CreateJobResponse is the 202 body after a durable submit.
type CreateJobResponse struct {
	ID string `json:"id"`
}

// JobStatusResponse is the body of GET /v1/jobs/{id}.
// Error is always present (JSON null when unset), matching Report.Error so
// clients use one nullability rule for job error fields.
type JobStatusResponse struct {
	ID        string    `json:"id"`
	Kind      JobKind   `json:"kind"`
	Status    JobStatus `json:"status"`
	Attempt   int       `json:"attempt"`
	Error     *string   `json:"error"`
	ReportURL string    `json:"report_url,omitempty"`
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

// Finding is one report finding with Story 5 provenance fields.
// Deterministic findings carry null rubric_id, rubric_version, and model_identity.
type Finding struct {
	Source        FindingSource   `json:"source"`
	RubricID      *string         `json:"rubric_id"`
	RubricVersion *string         `json:"rubric_version"`
	ModelIdentity *string         `json:"model_identity"`
	Payload       json.RawMessage `json:"payload"`
}

// Diagnostic is one report diagnostic entry.
type Diagnostic struct {
	Scope   string `json:"scope"`
	Message string `json:"message"`
}

// ReportSummary is a named-field summary (not a free-form map) so kind-specific
// counters cannot collide with rule/rubric ids in finding_counts.
type ReportSummary struct {
	FindingCounts map[string]map[string]int `json:"finding_counts"`
	PRCount       *int                      `json:"pr_count,omitempty"`
	PRFailedCount *int                      `json:"pr_failed_count,omitempty"`
}

// ReportVersions records analyzer and rubric id→version pairs used to build a report.
type ReportVersions struct {
	Analyzer string            `json:"analyzer"`
	Rubrics  map[string]string `json:"rubrics,omitempty"`
}

// Report is the frozen snake_case job report contract (report_version "1").
// Error is always present: JSON null when the job succeeded, a string when it failed.
// Findings and Diagnostics always serialize as JSON arrays (never null); a nil
// Summary.FindingCounts serializes as {}.
type Report struct {
	ReportVersion string          `json:"report_version"`
	JobID         string          `json:"job_id"`
	Kind          JobKind         `json:"kind"`
	Params        json.RawMessage `json:"params"`
	CommitSHA     string          `json:"commit_sha"`
	Summary       ReportSummary   `json:"summary"`
	Findings      []Finding       `json:"findings"`
	Diagnostics   []Diagnostic    `json:"diagnostics"`
	Error         *string         `json:"error"`
	Versions      ReportVersions  `json:"versions"`
	GeneratedAt   time.Time       `json:"generated_at"`
}

// MarshalJSON keeps the frozen wire shape: nil slices become [] and a nil
// finding_counts map becomes {}, so producers cannot accidentally emit null
// for fields the contract documents as arrays/objects.
func (r Report) MarshalJSON() ([]byte, error) {
	type reportJSON Report
	out := reportJSON(r)
	if out.Findings == nil {
		out.Findings = []Finding{}
	}
	if out.Diagnostics == nil {
		out.Diagnostics = []Diagnostic{}
	}
	if out.Summary.FindingCounts == nil {
		out.Summary.FindingCounts = map[string]map[string]int{}
	}
	return json.Marshal(out)
}

// APIError is the machine+human pair inside the stable error envelope.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorEnvelope is the stable JSON error shape for every API error response.
type ErrorEnvelope struct {
	Error APIError `json:"error"`
}

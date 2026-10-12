package coachapi

import (
	"encoding/json"
)

// Stable machine-readable API error codes.
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

// APIError is the machine+human pair inside the stable error envelope.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorEnvelope is the stable JSON error shape for every API error response.
type ErrorEnvelope struct {
	Error APIError `json:"error"`
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

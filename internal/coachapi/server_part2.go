package coachapi

import (
	"encoding/json"
	"errors"
	"fmt"

	"net/http"
)

func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, ErrorCodeUnauthenticated, "unauthenticated")
		return
	}

	id := r.PathValue("id")

	job, err := s.store.GetJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			writeAPIError(w, http.StatusNotFound, ErrorCodeJobNotFound, "job not found")
			return
		}
		writeAPIError(w, http.StatusServiceUnavailable, ErrorCodeInternalError, "failed to load job")
		return
	}

	if !ownsJob(principal, job) {
		writeAPIError(w, http.StatusForbidden, ErrorCodeUnauthorized, "you are not authorized to view this job")
		return
	}

	if job.Status != JobStatusCompleted {
		writeAPIError(w, http.StatusConflict, ErrorCodeJobNotCompleted,
			fmt.Sprintf("job is not yet completed (status: %s)", job.Status))
		return
	}

	report, err := s.store.GetReport(r.Context(), id)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, ErrorCodeInternalError, "failed to load report")
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// MarshalTaskPayload returns the ADR-006 versioned queue.Task.Payload body for
// jobID. POST /v1/jobs and the worker requeue reconciler must use this helper
// so submit and recovery publish the same wire shape.
func MarshalTaskPayload(jobID string) ([]byte, error) {
	return json.Marshal(taskPayload{
		SchemaVersion: TaskPayloadSchemaVersion1,
		JobID:         jobID,
	})
}

// Handler is the /v1/jobs HTTP surface. It expects a Principal via
// WithPrincipal. This package does not wrap itself, to avoid an import cycle
// with internal/authn. Missing Principal yields 401.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/jobs", s.handleCreateJob)
	mux.HandleFunc("GET /v1/jobs/{id}", s.handleGetJob)
	mux.HandleFunc("GET /v1/jobs/{id}/report", s.handleGetReport)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeAPIError(w, http.StatusNotFound, ErrorCodeNotFound, "not found")
	})
	return mux
}

// ownsJob reports whether principal is the creator of job. Login is
// deliberately excluded (a GitHub login can be renamed/reassigned); provider
// plus the stable subject id is the identity comparison.
func ownsJob(principal Principal, job Job) bool {
	return principal.Provider == job.CreatedByProvider && principal.Subject == job.CreatedBySubject
}
func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorEnvelope{Error: APIError{Code: code, Message: message}})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

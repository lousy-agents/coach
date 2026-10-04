package coachapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lousy-agents/coach/internal/authz"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

type ServerConfig struct {
	Store      JobStore
	Authorizer authz.RepoAuthorizer
	Queue      queue.TaskQueue
	Now        func() time.Time
	NewJobID   func() string
}

// Server is the /v1/jobs HTTP surface.
type Server struct {
	store      JobStore
	authorizer authz.RepoAuthorizer
	queue      queue.TaskQueue
	now        func() time.Time
	newJobID   func() string
}

// NewServer requires cfg.Store, cfg.Authorizer, and cfg.Queue.

// Handler is the /v1/jobs HTTP surface. It expects a Principal via
// WithPrincipal. This package does not wrap itself, to avoid an import cycle
// with internal/authn. Missing Principal yields 401.

// A "/" catch-all handles unmatched routes/methods with the stable 404
// envelope. Unlike looking up mux.Handler(r) and re-invoking it manually,
// letting mux.ServeHTTP dispatch directly is required for r.PathValue("id")
// to be populated on the {id} routes -- mux.Handler discards the match
// state ServeHTTP would otherwise attach to the request.

// TaskPayloadSchemaVersion1 is the supported queue.Task.Payload schema
// version for this package. ADR-006 requires versioned queue payloads; the
// worker decodes this wire shape independently of the job row.
const TaskPayloadSchemaVersion1 = 1

// taskPayload is the opaque queue.Task.Payload wire shape this package
// enqueues. The worker re-reads the job from the store, so only the
// schema version and job id travel through the queue. Task.ID remains the
// idempotency key; the queue adapter must not interpret these fields.
type taskPayload struct {
	SchemaVersion int    `json:"schema_version"`
	JobID         string `json:"job_id"`
}

// MarshalTaskPayload returns the ADR-006 versioned queue.Task.Payload body for
// jobID. POST /v1/jobs and the worker requeue reconciler must use this helper
// so submit and recovery publish the same wire shape.

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, ErrorCodeUnauthenticated, "unauthenticated")
		return
	}

	var req CreateJobRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, ErrorCodeInvalidRequest, "invalid request body")
		return
	}

	if req.Kind != JobKindRepoBaselineScan {
		writeAPIError(w, http.StatusBadRequest, ErrorCodeUnsupportedJobKind, fmt.Sprintf("unsupported job kind %q", req.Kind))
		return
	}

	var params RepoBaselineScanParams
	pdec := json.NewDecoder(bytes.NewReader(req.Params))
	pdec.DisallowUnknownFields()
	if err := pdec.Decode(&params); err != nil {
		writeAPIError(w, http.StatusBadRequest, ErrorCodeInvalidRequest, "invalid params for repo_baseline_scan")
		return
	}
	params.RepoOwner = strings.TrimSpace(params.RepoOwner)
	params.RepoName = strings.TrimSpace(params.RepoName)
	if params.RepoOwner == "" || params.RepoName == "" {
		writeAPIError(w, http.StatusBadRequest, ErrorCodeInvalidRequest, "repo_owner and repo_name are required")
		return
	}

	if err := s.authorizer.Authorize(r.Context(), principal.Login, params.RepoOwner, params.RepoName); err != nil {
		if errors.Is(err, authz.ErrNotAuthorized) {
			writeAPIError(w, http.StatusForbidden, ErrorCodeRepoNotAuthorized,
				"you have no role in this repository, or the Coach GitHub App is not installed on it; "+
					"public repositories with no assigned role are deliberately denied")
			return
		}
		writeAPIError(w, http.StatusServiceUnavailable, ErrorCodeInternalError, "authorization check temporarily unavailable")
		return
	}

	// Persist the validated/canonical params (trimmed owner/repo), not the
	// original raw JSON, so the worker fetches the same repository that was
	// authorized.
	canonicalParams, err := json.Marshal(params)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, ErrorCodeInternalError, "failed to encode job params")
		return
	}

	job := Job{
		ID:                s.newJobID(),
		Kind:              req.Kind,
		Params:            canonicalParams,
		Status:            JobStatusQueued,
		CreatedAt:         s.now(),
		Attempt:           0,
		CreatedByProvider: principal.Provider,
		CreatedBySubject:  principal.Subject,
		CreatedByLogin:    principal.Login,
	}

	if err := s.store.CreateJob(r.Context(), job); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, ErrorCodeInternalError, "failed to persist job")
		return
	}

	payload, err := MarshalTaskPayload(job.ID)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, ErrorCodeInternalError, "failed to build task payload")
		return
	}

	// Submit-durability rule: the job row is already persisted (queued) above.
	// If Enqueue fails, do not mark it failed and do not return bare success --
	// return a retriable 5xx and leave the row queued so a retry or an operator
	// requeue can still pick it up.
	if err := s.queue.Enqueue(r.Context(), queue.Task{ID: job.ID, Payload: payload}); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, ErrorCodeInternalError, "failed to enqueue job; it remains queued")
		return
	}

	writeJSON(w, http.StatusAccepted, CreateJobResponse{ID: job.ID})
}

// GetJob first (not GetReport) so ownership/precedence is enforced before
// any report data -- including an incomplete job's existence -- is
// touched. 401 -> 404 -> 403 -> 409 is the required precedence order.

// ownsJob reports whether principal is the creator of job. Login is
// deliberately excluded (a GitHub login can be renamed/reassigned); provider
// plus the stable subject id is the identity comparison.

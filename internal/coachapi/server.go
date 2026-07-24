package coachapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/authz"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// ServerConfig configures a Server.
type ServerConfig struct {
	Store      JobStore
	Authorizer authz.RepoAuthorizer
	Queue      queue.TaskQueue
	Now        func() time.Time
	NewJobID   func() string
}

// Server is the /v1/jobs... HTTP surface (Task 2 / GitHub issue #103).
type Server struct {
	store      JobStore
	authorizer authz.RepoAuthorizer
	queue      queue.TaskQueue
	now        func() time.Time
	newJobID   func() string
}

// NewServer builds a Server. cfg.Store, cfg.Authorizer, and cfg.Queue are
// required.
func NewServer(cfg ServerConfig) (*Server, error) {
	if cfg.Store == nil {
		return nil, errors.New("coachapi: ServerConfig.Store is required")
	}
	if cfg.Authorizer == nil {
		return nil, errors.New("coachapi: ServerConfig.Authorizer is required")
	}
	if cfg.Queue == nil {
		return nil, errors.New("coachapi: ServerConfig.Queue is required")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	newJobID := cfg.NewJobID
	if newJobID == nil {
		newJobID = watermill.NewUUID
	}
	return &Server{
		store:      cfg.Store,
		authorizer: cfg.Authorizer,
		queue:      cfg.Queue,
		now:        now,
		newJobID:   newJobID,
	}, nil
}

// Handler returns the /v1/jobs... HTTP surface. It expects to be wrapped by
// an auth middleware that attaches a Principal via WithPrincipal before a
// request reaches it (internal/authn.Service.Middleware, composed by
// cmd/coach-api's main.go -- not by this package, to avoid an import cycle
// between internal/coachapi and internal/authn). If no Principal is present,
// Handler responds 401 unauthenticated defensively, but this should not
// happen in a correctly composed deployment.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/jobs", s.handleCreateJob)
	mux.HandleFunc("GET /v1/jobs/{id}", s.handleGetJob)
	mux.HandleFunc("GET /v1/jobs/{id}/report", s.handleGetReport)
	// A "/" catch-all handles unmatched routes/methods with the stable 404
	// envelope. Unlike looking up mux.Handler(r) and re-invoking it manually,
	// letting mux.ServeHTTP dispatch directly is required for r.PathValue("id")
	// to be populated on the {id} routes -- mux.Handler discards the match
	// state ServeHTTP would otherwise attach to the request.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeAPIError(w, http.StatusNotFound, ErrorCodeNotFound, "not found")
	})
	return mux
}

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
func MarshalTaskPayload(jobID string) ([]byte, error) {
	return json.Marshal(taskPayload{
		SchemaVersion: TaskPayloadSchemaVersion1,
		JobID:         jobID,
	})
}

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

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
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

	resp := JobStatusResponse{
		ID:      job.ID,
		Kind:    job.Kind,
		Status:  job.Status,
		Attempt: job.Attempt,
		Error:   job.Error,
	}
	if job.Status == JobStatusCompleted {
		resp.ReportURL = "/v1/jobs/" + id + "/report"
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, ErrorCodeUnauthenticated, "unauthenticated")
		return
	}

	id := r.PathValue("id")
	// GetJob first (not GetReport) so ownership/precedence is enforced before
	// any report data -- including an incomplete job's existence -- is
	// touched. 401 -> 404 -> 403 -> 409 is the required precedence order.
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

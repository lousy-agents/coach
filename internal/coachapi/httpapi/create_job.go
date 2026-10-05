package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/lousy-agents/coach/internal/authz"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	principal, ok := coachapi.PrincipalFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, coachapi.ErrorCodeUnauthenticated, "unauthenticated")
		return
	}

	var req coachapi.CreateJobRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, coachapi.ErrorCodeInvalidRequest, "invalid request body")
		return
	}

	if req.Kind != coachapi.JobKindRepoBaselineScan {
		writeAPIError(w, http.StatusBadRequest, coachapi.ErrorCodeUnsupportedJobKind, fmt.Sprintf("unsupported job kind %q", req.Kind))
		return
	}

	var params coachapi.RepoBaselineScanParams
	pdec := json.NewDecoder(bytes.NewReader(req.Params))
	pdec.DisallowUnknownFields()
	if err := pdec.Decode(&params); err != nil {
		writeAPIError(w, http.StatusBadRequest, coachapi.ErrorCodeInvalidRequest, "invalid params for repo_baseline_scan")
		return
	}
	params.RepoOwner = strings.TrimSpace(params.RepoOwner)
	params.RepoName = strings.TrimSpace(params.RepoName)
	if params.RepoOwner == "" || params.RepoName == "" {
		writeAPIError(w, http.StatusBadRequest, coachapi.ErrorCodeInvalidRequest, "repo_owner and repo_name are required")
		return
	}

	if err := s.authorizer.Authorize(r.Context(), principal.Login, params.RepoOwner, params.RepoName); err != nil {
		if errors.Is(err, authz.ErrNotAuthorized) {
			writeAPIError(w, http.StatusForbidden, coachapi.ErrorCodeRepoNotAuthorized,
				"you have no role in this repository, or the Coach GitHub App is not installed on it; "+
					"public repositories with no assigned role are deliberately denied")
			return
		}
		writeAPIError(w, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError, "authorization check temporarily unavailable")
		return
	}

	// Persist the validated/canonical params (trimmed owner/repo), not the
	// original raw JSON, so the worker fetches the same repository that was
	// authorized.
	canonicalParams, err := json.Marshal(params)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError, "failed to encode job params")
		return
	}

	job := coachapi.Job{
		ID:                s.newJobID(),
		Kind:              req.Kind,
		Params:            canonicalParams,
		Status:            coachapi.JobStatusQueued,
		CreatedAt:         s.now(),
		Attempt:           0,
		CreatedByProvider: principal.Provider,
		CreatedBySubject:  principal.Subject,
		CreatedByLogin:    principal.Login,
	}

	if err := s.store.CreateJob(r.Context(), job); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError, "failed to persist job")
		return
	}

	payload, err := coachapi.MarshalTaskPayload(job.ID)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError, "failed to build task payload")
		return
	}

	// Submit-durability rule: the job row is already persisted (queued) above.
	// If Enqueue fails, do not mark it failed and do not return bare success --
	// return a retriable 5xx and leave the row queued so a retry or an operator
	// requeue can still pick it up.
	if err := s.queue.Enqueue(r.Context(), queue.Task{ID: job.ID, Payload: payload}); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError, "failed to enqueue job; it remains queued")
		return
	}

	writeJSON(w, http.StatusAccepted, coachapi.CreateJobResponse{ID: job.ID})
}

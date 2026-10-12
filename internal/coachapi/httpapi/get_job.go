package httpapi

import (
	"errors"
	"net/http"

	"github.com/lousy-agents/coach/internal/coachapi"
)

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	principal, ok := coachapi.PrincipalFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, coachapi.ErrorCodeUnauthenticated, "unauthenticated")
		return
	}

	id := r.PathValue("id")
	job, err := s.store.GetJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, coachapi.ErrJobNotFound) {
			writeAPIError(w, http.StatusNotFound, coachapi.ErrorCodeJobNotFound, "job not found")
			return
		}
		writeAPIError(w, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError, "failed to load job")
		return
	}

	if !ownsJob(principal, job) {
		writeAPIError(w, http.StatusForbidden, coachapi.ErrorCodeUnauthorized, "you are not authorized to view this job")
		return
	}

	resp := coachapi.JobStatusResponse{
		ID:      job.ID,
		Kind:    job.Kind,
		Status:  job.Status,
		Attempt: job.Attempt,
		Error:   job.Error,
	}
	if job.Status == coachapi.JobStatusCompleted {
		resp.ReportURL = "/v1/jobs/" + id + "/report"
	}
	writeJSON(w, http.StatusOK, resp)
}

package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/lousy-agents/coach/internal/coachapi"
)

func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	principal, ok := coachapi.PrincipalFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, coachapi.ErrorCodeUnauthenticated, "unauthenticated")
		return
	}

	id := r.PathValue("id")
	// GetJob first (not GetReport) so ownership/precedence is enforced before
	// any report data -- including an incomplete job's existence -- is
	// touched. 401 -> 404 -> 403 -> 409 is the required precedence order.
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

	if job.Status != coachapi.JobStatusCompleted {
		writeAPIError(w, http.StatusConflict, coachapi.ErrorCodeJobNotCompleted,
			fmt.Sprintf("job is not yet completed (status: %s)", job.Status))
		return
	}

	report, err := s.store.GetReport(r.Context(), id)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError, "failed to load report")
		return
	}
	writeJSON(w, http.StatusOK, report)
}

package coachapi

import (
	"errors"

	"github.com/ThreeDotsLabs/watermill"

	"net/http"

	"time"
)

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

// NewServer requires cfg.Store, cfg.Authorizer, and cfg.Queue.
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

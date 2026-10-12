// Package httpapi serves POST /v1/jobs, GET /v1/jobs/{id}, and
// GET /v1/jobs/{id}/report over a coachapi.JobStore, an
// authz.RepoAuthorizer, and a queue.TaskQueue. Every store or authorizer
// failure that is not a clean miss answers 503 with the stable error
// envelope.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/authz"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

type ServerConfig struct {
	Store      coachapi.JobStore
	Authorizer authz.RepoAuthorizer
	Queue      queue.TaskQueue
	Now        func() time.Time
	NewJobID   func() string
}

// Server is the /v1/jobs HTTP surface.
type Server struct {
	store      coachapi.JobStore
	authorizer authz.RepoAuthorizer
	queue      queue.TaskQueue
	now        func() time.Time
	newJobID   func() string
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

// Handler is the /v1/jobs HTTP surface. It expects a Principal via
// coachapi.WithPrincipal and does not authenticate requests itself: the
// composition root wraps it in internal/authn's middleware. Missing
// Principal yields 401.
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
		writeAPIError(w, http.StatusNotFound, coachapi.ErrorCodeNotFound, "not found")
	})
	return mux
}

// ownsJob reports whether principal is the creator of job. Login is
// deliberately excluded (a GitHub login can be renamed/reassigned); provider
// plus the stable subject id is the identity comparison.
func ownsJob(principal coachapi.Principal, job coachapi.Job) bool {
	return principal.Provider == job.CreatedByProvider && principal.Subject == job.CreatedBySubject
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, coachapi.ErrorEnvelope{Error: coachapi.APIError{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

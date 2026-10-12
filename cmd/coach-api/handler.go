package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/lousy-agents/coach/internal/authn"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/httpapi"
)

// buildHandler composes internal/authn and internal/coachapi into one HTTP
// surface: authnSvc.Handler() serves /oauth/..., /v1/me, and
// /v1/auth/test-mint; authnSvc.Middleware wraps httpapi.Server.Handler()
// for /v1/jobs and its subpaths, since httpapi.Server does not self-guard
// (see internal/coachapi/httpapi/server.go's Handler doc comment). A request whose
// path matches none of those registers on the "/" catch-all below, which
// returns the same stable not_found envelope every other unmatched route in
// this API returns.
func buildHandler(cfg Config, deps Dependencies) (http.Handler, error) {
	if deps.Store == nil {
		return nil, errors.New("coach-api: Dependencies.Store is required")
	}
	if deps.Authorizer == nil {
		return nil, errors.New("coach-api: Dependencies.Authorizer is required")
	}
	if deps.Queue == nil {
		return nil, errors.New("coach-api: Dependencies.Queue is required")
	}

	authnSvc, err := authn.New(authn.Options{
		SigningKey:      cfg.JWTSigningKey,
		Issuer:          cfg.JWTIssuer,
		TokenTTL:        cfg.JWTTokenTTL,
		TestMintEnabled: cfg.AuthTestMintEnabled,
		GitHubOAuth:     cfg.GitHubOAuth,
	})
	if err != nil {
		return nil, fmt.Errorf("coach-api: constructing authn service: %w", err)
	}

	coachSrv, err := httpapi.NewServer(httpapi.ServerConfig{
		Store:      deps.Store,
		Authorizer: deps.Authorizer,
		Queue:      deps.Queue,
	})
	if err != nil {
		return nil, fmt.Errorf("coach-api: constructing coachapi server: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/oauth/", authnSvc.Handler())
	mux.Handle("/v1/me", authnSvc.Handler())
	mux.Handle("/v1/auth/test-mint", authnSvc.Handler())

	jobsHandler := authnSvc.Middleware(coachSrv.Handler())
	mux.Handle("/v1/jobs", jobsHandler)
	mux.Handle("/v1/jobs/", jobsHandler)

	mux.HandleFunc("/", writeNotFoundEnvelope)

	return mux, nil
}

// writeNotFoundEnvelope answers any path not claimed by a more specific
// pattern on the composed mux with the same stable not_found envelope every
// other unmatched route in this API returns.
func writeNotFoundEnvelope(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(coachapi.ErrorEnvelope{
		Error: coachapi.APIError{Code: coachapi.ErrorCodeNotFound, Message: "not found"},
	})
}

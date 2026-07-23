package authn

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// Handler returns the auth HTTP surface: protected GET /v1/me; when
// TestMintEnabled, POST /v1/auth/test-mint; when GitHubOAuth is configured,
// unauthenticated GET /oauth/github/start and GET /oauth/github/callback.
// Unregistered paths return 404 with code not_found.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /v1/me", s.Middleware(http.HandlerFunc(s.handleMe)))
	if s.testMintEnabled {
		mux.Handle("POST /v1/auth/test-mint", http.HandlerFunc(s.handleTestMint))
	}
	if s.githubOAuth != nil {
		mux.Handle("GET /oauth/github/start", http.HandlerFunc(s.handleGitHubOAuthStart))
		mux.Handle("GET /oauth/github/callback", http.HandlerFunc(s.handleGitHubOAuthCallback))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, pattern := mux.Handler(r)
		if pattern == "" {
			writeAPIError(w, http.StatusNotFound, coachapi.ErrorCodeNotFound, "not found")
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (s *Service) handleMe(w http.ResponseWriter, r *http.Request) {
	p, ok := coachapi.PrincipalFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, coachapi.ErrorCodeUnauthenticated, "unauthenticated")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

type testMintRequest struct {
	Subject string `json:"subject"`
	Login   string `json:"login"`
}

type testMintResponse struct {
	Token string `json:"token"`
}

func (s *Service) handleTestMint(w http.ResponseWriter, r *http.Request) {
	var req testMintRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, coachapi.ErrorCodeInvalidRequest, "invalid request body")
		return
	}
	req.Subject = strings.TrimSpace(req.Subject)
	req.Login = strings.TrimSpace(req.Login)
	if req.Subject == "" || req.Login == "" {
		writeAPIError(w, http.StatusBadRequest, coachapi.ErrorCodeInvalidRequest, "subject and login are required")
		return
	}
	tok, err := s.Issue(r.Context(), coachapi.Principal{
		Provider: "github",
		Subject:  req.Subject,
		Login:    req.Login,
	})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, coachapi.ErrorCodeInternalError, "failed to mint token")
		return
	}
	writeJSON(w, http.StatusOK, testMintResponse{Token: tok})
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, coachapi.ErrorEnvelope{
		Error: coachapi.APIError{Code: code, Message: message},
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

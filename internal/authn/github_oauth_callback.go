package authn

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/internal/coachapi"
)

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

// handleGitHubOAuthCallback completes the OAuth flow: validates state, exchanges
// code, fetches GET /user, and returns a Coach-signed JWT bearer token.
func (s *Service) handleGitHubOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if s.githubOAuth == nil || s.oauthState == nil {
		writeAPIError(w, http.StatusNotFound, coachapi.ErrorCodeNotFound, "not found")
		return
	}
	q := r.URL.Query()
	if ghErr := q.Get("error"); ghErr != "" {
		writeAPIError(w, http.StatusBadRequest, coachapi.ErrorCodeInvalidRequest, "oauth denied: "+ghErr)
		return
	}
	state := strings.TrimSpace(q.Get("state"))
	if state == "" {
		writeAPIError(w, http.StatusBadRequest, coachapi.ErrorCodeInvalidRequest, "missing oauth state")
		return
	}
	ok, err := s.oauthState.Consume(r.Context(), state, s.now())
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError, "oauth state store unavailable")
		return
	}
	if !ok {
		writeAPIError(w, http.StatusBadRequest, coachapi.ErrorCodeInvalidRequest, "invalid or expired oauth state")
		return
	}
	code := strings.TrimSpace(q.Get("code"))
	if code == "" {
		writeAPIError(w, http.StatusBadRequest, coachapi.ErrorCodeInvalidRequest, "missing oauth code")
		return
	}

	accessToken, err := s.exchangeCode(r.Context(), code)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, coachapi.ErrorCodeInvalidRequest, "oauth code exchange failed")
		return
	}
	user, err := s.fetchGitHubUser(r.Context(), accessToken)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, coachapi.ErrorCodeInvalidRequest, "oauth user fetch failed")
		return
	}
	if user.ID == 0 || user.Login == "" {
		writeAPIError(w, http.StatusBadRequest, coachapi.ErrorCodeInvalidRequest, "oauth user incomplete")
		return
	}

	jwt, err := s.Issue(r.Context(), coachapi.Principal{
		Provider: "github",
		Subject:  strconv.FormatInt(user.ID, 10),
		Login:    user.Login,
	})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, coachapi.ErrorCodeInternalError, "failed to mint token")
		return
	}
	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken: jwt,
		TokenType:   "bearer",
	})
}

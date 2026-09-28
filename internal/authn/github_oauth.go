package authn

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// GitHubOAuthConfig is the GitHub OAuth App authorization-code flow.
// BaseURL is the OAuth origin for /login/oauth/authorize and
// /login/oauth/access_token (e.g. https://github.com or a fakegithub Server URL).
// APIBaseURL is the REST API origin for GET /user (e.g. https://api.github.com);
// when empty it defaults to BaseURL so single-host fakes keep working.
// GHE often needs APIBaseURL like https://ghe.example.com/api/v3. v1 requests no OAuth scope.
type GitHubOAuthConfig struct {
	ClientID     string
	ClientSecret string
	BaseURL      string // no trailing slash; OAuth authorize + token
	APIBaseURL   string // no trailing slash; GET /user (defaults to BaseURL)
	RedirectURI  string // absolute callback URL registered with the OAuth App
	HTTPClient   *http.Client
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type githubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

type githubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// handleGitHubOAuthStart begins the OAuth authorization-code flow: stores CSRF
// state and redirects to GitHub's authorize URL with no scope.

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

func (s *Service) authorizeURL(state string) string {
	// BaseURL is absolute (validated in New); build query via Values for escaping.
	// Intentionally no scope: public id/login only (ADR-001).
	base := strings.TrimRight(s.githubOAuth.BaseURL, "/")
	return base + "/login/oauth/authorize?" + url.Values{
		"client_id":    {s.githubOAuth.ClientID},
		"redirect_uri": {s.githubOAuth.RedirectURI},
		"state":        {state},
	}.Encode()
}

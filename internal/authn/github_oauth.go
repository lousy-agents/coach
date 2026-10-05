package authn

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
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

// handleGitHubOAuthStart begins the OAuth authorization-code flow: stores CSRF
// state and redirects to GitHub's authorize URL with no scope.
func (s *Service) handleGitHubOAuthStart(w http.ResponseWriter, r *http.Request) {
	if s.githubOAuth == nil || s.oauthState == nil {
		writeAPIError(w, http.StatusNotFound, coachapi.ErrorCodeNotFound, "not found")
		return
	}
	state, err := newOAuthState()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, coachapi.ErrorCodeInternalError, "failed to start oauth")
		return
	}
	now := s.now()
	if err := s.oauthState.Save(r.Context(), state, now.Add(s.oauthStateTTL)); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError, "oauth state store unavailable")
		return
	}
	authURL := s.authorizeURL(state)
	http.Redirect(w, r, authURL, http.StatusFound)
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

func newOAuthState() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("authn: generate oauth state: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

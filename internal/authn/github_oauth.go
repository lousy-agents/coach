package authn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// GitHubOAuthConfig configures the GitHub OAuth App authorization-code flow.
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
		writeAPIError(w, http.StatusInternalServerError, coachapi.ErrorCodeInternalError, "failed to start oauth")
		return
	}
	authURL := s.authorizeURL(state)
	http.Redirect(w, r, authURL, http.StatusFound)
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
		writeAPIError(w, http.StatusInternalServerError, coachapi.ErrorCodeInternalError, "oauth state store unavailable")
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
	base := strings.TrimRight(s.githubOAuth.BaseURL, "/")
	u, err := url.Parse(base + "/login/oauth/authorize")
	if err != nil {
		// BaseURL was validated at New; fall back to string join.
		return base + "/login/oauth/authorize?" + url.Values{
			"client_id":    {s.githubOAuth.ClientID},
			"redirect_uri": {s.githubOAuth.RedirectURI},
			"state":        {state},
		}.Encode()
	}
	q := u.Query()
	q.Set("client_id", s.githubOAuth.ClientID)
	q.Set("redirect_uri", s.githubOAuth.RedirectURI)
	q.Set("state", state)
	// Intentionally no scope: public id/login only (ADR-001).
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *Service) exchangeCode(ctx context.Context, code string) (string, error) {
	base := strings.TrimRight(s.githubOAuth.BaseURL, "/")
	form := url.Values{
		"client_id":     {s.githubOAuth.ClientID},
		"client_secret": {s.githubOAuth.ClientSecret},
		"code":          {code},
		"redirect_uri":  {s.githubOAuth.RedirectURI},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint status %d", resp.StatusCode)
	}
	var tr githubTokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", err
	}
	if tr.Error != "" {
		return "", fmt.Errorf("token error: %s", tr.Error)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("empty access_token")
	}
	return tr.AccessToken, nil
}

func (s *Service) fetchGitHubUser(ctx context.Context, accessToken string) (githubUser, error) {
	base := strings.TrimRight(s.githubOAuth.APIBaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/user", nil)
	if err != nil {
		return githubUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return githubUser{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return githubUser{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return githubUser{}, fmt.Errorf("user endpoint status %d", resp.StatusCode)
	}
	var u githubUser
	if err := json.Unmarshal(body, &u); err != nil {
		return githubUser{}, err
	}
	return u, nil
}

func newOAuthState() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("authn: generate oauth state: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

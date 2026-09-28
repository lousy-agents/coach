package authn

import (
	"context"

	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/lousy-agents/coach/internal/coachapi"

	"strings"
)

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

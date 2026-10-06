package authn

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// DefaultGitHubHTTPClientTimeout bounds outbound OAuth HTTP calls when
// GitHubOAuthConfig.HTTPClient is nil.
const DefaultGitHubHTTPClientTimeout = 10 * time.Second

func githubOAuthFromOptions(opts Options) (*GitHubOAuthConfig, OAuthStateStore, time.Duration, *http.Client, error) {
	if opts.GitHubOAuth == nil {
		return nil, nil, opts.OAuthStateTTL, nil, nil
	}
	if opts.GitHubOAuth.ClientID == "" || opts.GitHubOAuth.ClientSecret == "" {
		return nil, nil, 0, nil, errors.New("authn: GitHubOAuth ClientID and ClientSecret are required")
	}
	if opts.GitHubOAuth.BaseURL == "" {
		return nil, nil, 0, nil, errors.New("authn: GitHubOAuth BaseURL is required")
	}
	if err := requireAbsoluteURL("GitHubOAuth.BaseURL", opts.GitHubOAuth.BaseURL); err != nil {
		return nil, nil, 0, nil, err
	}
	if opts.GitHubOAuth.RedirectURI == "" {
		return nil, nil, 0, nil, errors.New("authn: GitHubOAuth RedirectURI is required")
	}
	cp := *opts.GitHubOAuth
	if cp.APIBaseURL == "" {
		cp.APIBaseURL = cp.BaseURL
	} else if err := requireAbsoluteURL("GitHubOAuth.APIBaseURL", cp.APIBaseURL); err != nil {
		return nil, nil, 0, nil, err
	}
	oauthState := opts.OAuthState
	if oauthState == nil {
		oauthState = NewMemoryOAuthState()
	}
	oauthTTL := opts.OAuthStateTTL
	if oauthTTL <= 0 {
		oauthTTL = 10 * time.Minute
	}
	httpClient := opts.GitHubOAuth.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultGitHubHTTPClientTimeout}
	}
	return &cp, oauthState, oauthTTL, httpClient, nil
}

func requireAbsoluteURL(field, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("authn: %s must be an absolute URL", field)
	}
	return nil
}

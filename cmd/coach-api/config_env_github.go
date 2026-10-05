package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/authn"
)

// loadGitHubOAuthConfigFromEnv returns nil (OAuth routes disabled) unless
// both COACH_GITHUB_OAUTH_CLIENT_ID and COACH_GITHUB_OAUTH_CLIENT_SECRET are
// set -- OAuth against real GitHub is optional for operators.
func loadGitHubOAuthConfigFromEnv() (*authn.GitHubOAuthConfig, error) {
	clientID := os.Getenv("COACH_GITHUB_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("COACH_GITHUB_OAUTH_CLIENT_SECRET")
	if clientID == "" && clientSecret == "" {
		return nil, nil
	}
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("coach-api: COACH_GITHUB_OAUTH_CLIENT_ID and COACH_GITHUB_OAUTH_CLIENT_SECRET must both be set or both unset")
	}

	redirectURI := os.Getenv("COACH_GITHUB_OAUTH_REDIRECT_URI")
	if redirectURI == "" {
		return nil, errors.New("coach-api: COACH_GITHUB_OAUTH_REDIRECT_URI is required when GitHub OAuth is configured")
	}

	baseURL := os.Getenv("COACH_GITHUB_OAUTH_BASE_URL")
	if baseURL == "" {
		baseURL = defaultOAuthBaseURL
	}
	apiBaseURL := os.Getenv("COACH_GITHUB_OAUTH_API_BASE_URL")
	if apiBaseURL == "" {
		apiBaseURL = defaultOAuthAPIBaseURL
	}

	return &authn.GitHubOAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		BaseURL:      baseURL,
		APIBaseURL:   apiBaseURL,
		RedirectURI:  redirectURI,
	}, nil
}

// loadGitHubAppPrivateKeyFromEnv supports either a raw PEM value in
// COACH_GITHUB_APP_PRIVATE_KEY, or a path to a PEM file in
// COACH_GITHUB_APP_PRIVATE_KEY_PATH -- a multi-line PEM crammed into one env
// var is awkward, so the file-path form is offered as well. Returns a nil
// slice (not an error) if neither is set, so the caller can report the
// missing-required-var case alongside its other missing vars.
func loadGitHubAppPrivateKeyFromEnv() ([]byte, error) {
	if raw := os.Getenv("COACH_GITHUB_APP_PRIVATE_KEY"); raw != "" {
		return []byte(raw), nil
	}
	path := os.Getenv("COACH_GITHUB_APP_PRIVATE_KEY_PATH")
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("coach-api: reading COACH_GITHUB_APP_PRIVATE_KEY_PATH %q: %w", path, err)
	}
	return data, nil
}

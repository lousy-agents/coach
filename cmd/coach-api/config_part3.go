package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/authn"

	"time"
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
func withOptionalRedisEnv(cfg InfraConfig) (InfraConfig, error) {
	if raw := os.Getenv("COACH_REDIS_DB"); raw != "" {
		var db int
		if _, err := fmt.Sscanf(raw, "%d", &db); err != nil {
			return InfraConfig{}, fmt.Errorf("coach-api: invalid COACH_REDIS_DB %q: %w", raw, err)
		}
		cfg.RedisDB = db
	}
	if raw := os.Getenv("COACH_REDIS_CLAIM_AFTER"); raw != "" {
		claimAfter, err := time.ParseDuration(raw)
		if err != nil {
			return InfraConfig{}, fmt.Errorf("coach-api: invalid COACH_REDIS_CLAIM_AFTER %q: %w", raw, err)
		}
		cfg.RedisClaimAfter = claimAfter
	}
	return cfg, nil
}

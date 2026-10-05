package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// loadConfigFromEnv reads Config from the process environment. It fails
// fast (a descriptive, non-nil error) if any required var is missing or
// malformed, rather than silently defaulting a signing key or issuer.
func loadConfigFromEnv() (Config, error) {
	var missing []string

	signingKey := os.Getenv("COACH_JWT_SIGNING_KEY")
	if signingKey == "" {
		missing = append(missing, "COACH_JWT_SIGNING_KEY")
	}
	issuer := os.Getenv("COACH_JWT_ISSUER")
	if issuer == "" {
		missing = append(missing, "COACH_JWT_ISSUER")
	}
	addr := os.Getenv("COACH_HTTP_ADDR")
	if addr == "" {
		missing = append(missing, "COACH_HTTP_ADDR")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("coach-api: missing required env var(s): %s", strings.Join(missing, ", "))
	}

	cfg := Config{
		HTTPAddr:            addr,
		JWTSigningKey:       []byte(signingKey),
		JWTIssuer:           issuer,
		AuthTestMintEnabled: os.Getenv("COACH_AUTH_TEST_MINT") == "1",
	}

	if raw := os.Getenv("COACH_JWT_TOKEN_TTL"); raw != "" {
		ttl, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("coach-api: invalid COACH_JWT_TOKEN_TTL %q: %w", raw, err)
		}
		cfg.JWTTokenTTL = ttl
	}

	oauthCfg, err := loadGitHubOAuthConfigFromEnv()
	if err != nil {
		return Config{}, err
	}
	cfg.GitHubOAuth = oauthCfg

	return cfg, nil
}

func valueOrDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// loadInfraConfigFromEnv reads InfraConfig from the process environment. It
// fails fast if any required var is missing or malformed, rather than
// silently defaulting GitHub App credentials. GitHub App ID/private key are
// optional only when both COACH_AUTHZ_BYPASS_OWNER and COACH_AUTHZ_BYPASS_REPO
// are set (credential-free compose smoke); otherwise App credentials remain
// required.
func loadInfraConfigFromEnv() (InfraConfig, error) {
	var missing []string

	redisAddr := os.Getenv("COACH_REDIS_ADDR")
	if redisAddr == "" {
		missing = append(missing, "COACH_REDIS_ADDR")
	}

	bypassOwner := os.Getenv("COACH_AUTHZ_BYPASS_OWNER")
	bypassRepo := os.Getenv("COACH_AUTHZ_BYPASS_REPO")
	hasBypass := bypassOwner != "" && bypassRepo != ""

	appIDRaw := os.Getenv("COACH_GITHUB_APP_ID")
	privateKey, err := loadGitHubAppPrivateKeyFromEnv()
	if err != nil {
		return InfraConfig{}, err
	}
	hasAppID := appIDRaw != ""
	hasAppKey := len(privateKey) > 0
	if hasAppID != hasAppKey {
		return InfraConfig{}, errors.New("coach-api: COACH_GITHUB_APP_ID and COACH_GITHUB_APP_PRIVATE_KEY (or _PATH) must both be set or both unset")
	}
	if !hasAppID && !hasBypass {
		missing = append(missing,
			"COACH_GITHUB_APP_ID",
			"COACH_GITHUB_APP_PRIVATE_KEY or COACH_GITHUB_APP_PRIVATE_KEY_PATH",
			"or both COACH_AUTHZ_BYPASS_OWNER and COACH_AUTHZ_BYPASS_REPO",
		)
	}

	if len(missing) > 0 {
		return InfraConfig{}, fmt.Errorf("coach-api: missing required env var(s): %s", strings.Join(missing, ", "))
	}

	var appID int64
	if hasAppID {
		if _, err := fmt.Sscanf(appIDRaw, "%d", &appID); err != nil || appID <= 0 {
			return InfraConfig{}, fmt.Errorf("coach-api: COACH_GITHUB_APP_ID must be a positive integer, got %q", appIDRaw)
		}
	}

	cfg := InfraConfig{
		GitHubAppID:         appID,
		GitHubAppPrivateKey: privateKey,
		RedisAddr:           redisAddr,
		RedisPassword:       os.Getenv("COACH_REDIS_PASSWORD"),
		RedisStream:         valueOrDefault(os.Getenv("COACH_REDIS_STREAM"), defaultRedisStream),
		RedisConsumerGroup:  valueOrDefault(os.Getenv("COACH_REDIS_CONSUMER_GROUP"), defaultRedisConsumerGroup),
		RedisConsumer:       os.Getenv("COACH_REDIS_CONSUMER"),
		RedisClaimAfter:     defaultRedisClaimAfter,
		PostgresDSN:         os.Getenv("COACH_PG_DSN"),
		AuthzBypassOwner:    bypassOwner,
		AuthzBypassRepo:     bypassRepo,
	}
	return withOptionalRedisEnv(cfg)
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

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lousy-agents/coach/internal/authn"
)

// defaultOAuthBaseURL and defaultOAuthAPIBaseURL are used when
// COACH_GITHUB_OAUTH_BASE_URL/COACH_GITHUB_OAUTH_API_BASE_URL are unset, so
// operators pointing at real GitHub do not need to configure them.
const (
	defaultOAuthBaseURL    = "https://github.com"
	defaultOAuthAPIBaseURL = "https://api.github.com"

	// defaultRedisStream and defaultRedisConsumerGroup match ADR-006's
	// example Redis Streams shape; coach-api only enqueues (it never calls
	// Claim), so ConsumerGroup matters far less here than for coach-worker,
	// but redisstream.Config.Validate still requires it to be set.
	defaultRedisStream        = "coach-jobs"
	defaultRedisConsumerGroup = "coach-api"

	// defaultRedisClaimAfter is required by redisstream.Config.Validate
	// even though coach-api never calls Claim.
	defaultRedisClaimAfter = 5 * time.Minute
)

// Config holds cmd/coach-api's environment-driven settings that do not
// require constructing a live dependency. Settings that do (JobStore,
// authz.RepoAuthorizer, queue.TaskQueue) live in InfraConfig/Dependencies
// instead, so buildHandler can be exercised against stubs (see
// main_acceptance_test.go) without a real Redis, Postgres, or GitHub App.
type Config struct {
	HTTPAddr string

	JWTSigningKey []byte
	JWTIssuer     string
	// JWTTokenTTL is 0 to use authn.Options' own default (1 hour).
	JWTTokenTTL time.Duration

	// AuthTestMintEnabled must default to false; only COACH_AUTH_TEST_MINT=1
	// turns it on.
	AuthTestMintEnabled bool

	// GitHubOAuth, when non-nil, registers the /oauth/github/* routes.
	GitHubOAuth *authn.GitHubOAuthConfig
}

// InfraConfig holds the environment-driven settings needed to construct the
// live Store/Authorizer/Queue Dependencies (buildDependencies). It is kept
// separate from Config so Config never implies a network dependency and can
// be constructed freely by tests.
type InfraConfig struct {
	GitHubAppID         int64
	GitHubAppPrivateKey []byte

	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	RedisStream        string
	RedisConsumerGroup string
	RedisConsumer      string
	RedisClaimAfter    time.Duration

	// PostgresDSN selects PostgresStore when set; MemoryStore when empty.
	PostgresDSN string

	// AuthzBypassOwner/AuthzBypassRepo, when both set, wrap the live
	// authz.RepoAuthorizer in authz.NewBypassAuthorizer for that exact pair
	// (Story 3's credential-free smoke exception). Must default to unset.
	AuthzBypassOwner string
	AuthzBypassRepo  string
}

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

// loadInfraConfigFromEnv reads InfraConfig from the process environment. It
// fails fast if any required var is missing or malformed, rather than
// silently defaulting GitHub App credentials.
func loadInfraConfigFromEnv() (InfraConfig, error) {
	var missing []string

	appIDRaw := os.Getenv("COACH_GITHUB_APP_ID")
	if appIDRaw == "" {
		missing = append(missing, "COACH_GITHUB_APP_ID")
	}
	redisAddr := os.Getenv("COACH_REDIS_ADDR")
	if redisAddr == "" {
		missing = append(missing, "COACH_REDIS_ADDR")
	}

	privateKey, err := loadGitHubAppPrivateKeyFromEnv()
	if err != nil {
		return InfraConfig{}, err
	}
	if len(privateKey) == 0 {
		missing = append(missing, "COACH_GITHUB_APP_PRIVATE_KEY or COACH_GITHUB_APP_PRIVATE_KEY_PATH")
	}

	if len(missing) > 0 {
		return InfraConfig{}, fmt.Errorf("coach-api: missing required env var(s): %s", strings.Join(missing, ", "))
	}

	var appID int64
	if _, err := fmt.Sscanf(appIDRaw, "%d", &appID); err != nil || appID <= 0 {
		return InfraConfig{}, fmt.Errorf("coach-api: COACH_GITHUB_APP_ID must be a positive integer, got %q", appIDRaw)
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
		AuthzBypassOwner:    os.Getenv("COACH_AUTHZ_BYPASS_OWNER"),
		AuthzBypassRepo:     os.Getenv("COACH_AUTHZ_BYPASS_REPO"),
	}

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

func valueOrDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

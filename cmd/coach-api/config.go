package main

import (
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

	// PostgresDSN selects postgres.Store when set; memory.Store when empty.
	PostgresDSN string

	// AuthzBypassOwner/AuthzBypassRepo, when both set, wrap the live
	// authz.RepoAuthorizer in authz.NewBypassAuthorizer for that exact pair
	// (credential-free smoke exception). Must default to unset.
	AuthzBypassOwner string
	AuthzBypassRepo  string
}

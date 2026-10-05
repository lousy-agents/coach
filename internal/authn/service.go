package authn

import (
	"errors"
	"net/http"
	"time"
)

// Options is Coach JWT auth, optional test-mint, and optional GitHub OAuth.
type Options struct {
	SigningKey      []byte
	Issuer          string
	TokenTTL        time.Duration
	Now             func() time.Time
	Denylist        Denylist
	TestMintEnabled bool
	// GitHubOAuth, when non-nil, registers unauthenticated OAuth start/callback routes.
	GitHubOAuth *GitHubOAuthConfig
	// OAuthState stores CSRF state for the OAuth flow; defaults to memory when GitHubOAuth is set.
	OAuthState OAuthStateStore
	// OAuthStateTTL is how long start-issued state remains valid; defaults to 10 minutes.
	OAuthStateTTL time.Duration
}

// Service issues and validates Coach JWTs and serves auth HTTP routes.
type Service struct {
	key             []byte
	issuer          string
	ttl             time.Duration
	now             func() time.Time
	denylist        Denylist
	testMintEnabled bool
	githubOAuth     *GitHubOAuthConfig
	oauthState      OAuthStateStore
	oauthStateTTL   time.Duration
	httpClient      *http.Client
}

// New requires SigningKey and Issuer. Denylist defaults to an in-memory
// store; Now defaults to time.Now; TokenTTL defaults to 1 hour.
func New(opts Options) (*Service, error) {
	if len(opts.SigningKey) == 0 {
		return nil, errors.New("authn: SigningKey is required")
	}
	if opts.Issuer == "" {
		return nil, errors.New("authn: Issuer is required")
	}
	ttl := opts.TokenTTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	dl := opts.Denylist
	if dl == nil {
		dl = NewMemoryDenylist()
	}
	gh, oauthState, oauthTTL, httpClient, err := githubOAuthFromOptions(opts)
	if err != nil {
		return nil, err
	}
	return &Service{
		key:             append([]byte(nil), opts.SigningKey...),
		issuer:          opts.Issuer,
		ttl:             ttl,
		now:             now,
		denylist:        dl,
		testMintEnabled: opts.TestMintEnabled,
		githubOAuth:     gh,
		oauthState:      oauthState,
		oauthStateTTL:   oauthTTL,
		httpClient:      httpClient,
	}, nil
}

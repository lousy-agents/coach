package authn

import (
	"errors"

	"net/http"

	"time"

	"github.com/golang-jwt/jwt/v5"
)

// DefaultGitHubHTTPClientTimeout bounds outbound OAuth HTTP calls when
// GitHubOAuthConfig.HTTPClient is nil.
const DefaultGitHubHTTPClientTimeout = 10 * time.Second

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

// Claims is the inspectable subset of Coach JWT claims (provider, sub, login, iss, exp, jti).
type Claims struct {
	Provider  string
	Subject   string
	Login     string
	Issuer    string
	ExpiresAt time.Time
	ID        string
}

type coachClaims struct {
	Provider string `json:"provider"`
	Login    string `json:"login"`
	jwt.RegisteredClaims
}

// Sentinel errors for Validate / middleware classification.
var (
	ErrUnauthenticated = errors.New("authn: unauthenticated")
	ErrDenylistStore   = errors.New("authn: denylist store error")
)

// New requires SigningKey and Issuer. Denylist defaults to an in-memory
// store; Now defaults to time.Now; TokenTTL defaults to 1 hour.

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

// Issue creates a signed Coach JWT for p (HS256) with a fresh jti.

// Validate checks signature, issuer, expiry, and jti denylist, then returns the Principal.
// Denylist store failures wrap ErrDenylistStore; credential failures wrap ErrUnauthenticated.

// Revoke denylists the token's jti until its exp so subsequent Validate calls fail.

// InspectClaims returns Coach JWT claims without denylist checks (signature and
// issuer still verified; expiry is not enforced so operators can inspect expired tokens).

// Skip library time validation so expiry uses the injected clock (tests and
// operators control Now); still restrict alg via WithValidMethods.

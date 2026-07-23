package authn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lousy-agents/coach/internal/coachapi"
)

// DefaultGitHubHTTPClientTimeout bounds outbound OAuth HTTP calls when
// GitHubOAuthConfig.HTTPClient is nil.
const DefaultGitHubHTTPClientTimeout = 10 * time.Second

// Options configures Coach JWT auth, optional test-mint, and optional GitHub OAuth.
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

// New constructs a Service. SigningKey and Issuer are required; Denylist
// defaults to an in-memory store; Now defaults to time.Now; TokenTTL defaults
// to 1 hour.
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
	var gh *GitHubOAuthConfig
	var oauthState OAuthStateStore
	oauthTTL := opts.OAuthStateTTL
	var httpClient *http.Client
	if opts.GitHubOAuth != nil {
		if opts.GitHubOAuth.ClientID == "" || opts.GitHubOAuth.ClientSecret == "" {
			return nil, errors.New("authn: GitHubOAuth ClientID and ClientSecret are required")
		}
		if opts.GitHubOAuth.BaseURL == "" {
			return nil, errors.New("authn: GitHubOAuth BaseURL is required")
		}
		if err := requireAbsoluteURL("GitHubOAuth.BaseURL", opts.GitHubOAuth.BaseURL); err != nil {
			return nil, err
		}
		if opts.GitHubOAuth.RedirectURI == "" {
			return nil, errors.New("authn: GitHubOAuth RedirectURI is required")
		}
		cp := *opts.GitHubOAuth
		if cp.APIBaseURL == "" {
			cp.APIBaseURL = cp.BaseURL
		} else if err := requireAbsoluteURL("GitHubOAuth.APIBaseURL", cp.APIBaseURL); err != nil {
			return nil, err
		}
		gh = &cp
		oauthState = opts.OAuthState
		if oauthState == nil {
			oauthState = NewMemoryOAuthState()
		}
		if oauthTTL <= 0 {
			oauthTTL = 10 * time.Minute
		}
		httpClient = opts.GitHubOAuth.HTTPClient
		if httpClient == nil {
			httpClient = &http.Client{Timeout: DefaultGitHubHTTPClientTimeout}
		}
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

// Issue creates a signed Coach JWT for p (HS256) with a fresh jti.
func (s *Service) Issue(_ context.Context, p coachapi.Principal) (string, error) {
	if p.Provider == "" || p.Subject == "" || p.Login == "" {
		return "", errors.New("authn: principal provider, subject, and login are required")
	}
	jti, err := newJTI()
	if err != nil {
		return "", err
	}
	now := s.now()
	claims := coachClaims{
		Provider: p.Provider,
		Login:    p.Login,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   p.Subject,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(s.key)
	if err != nil {
		return "", fmt.Errorf("authn: sign token: %w", err)
	}
	return signed, nil
}

// Validate checks signature, issuer, expiry, and jti denylist, then returns the Principal.
// Denylist store failures wrap ErrDenylistStore; credential failures wrap ErrUnauthenticated.
func (s *Service) Validate(ctx context.Context, token string) (coachapi.Principal, error) {
	claims, err := s.parseClaims(token)
	if err != nil {
		return coachapi.Principal{}, fmt.Errorf("%w: %v", ErrUnauthenticated, err)
	}
	if claims.ID == "" {
		return coachapi.Principal{}, fmt.Errorf("%w: missing jti", ErrUnauthenticated)
	}
	revoked, err := s.denylist.IsRevoked(ctx, claims.ID)
	if err != nil {
		return coachapi.Principal{}, fmt.Errorf("%w: %v", ErrDenylistStore, err)
	}
	if revoked {
		return coachapi.Principal{}, fmt.Errorf("%w: jti denylisted", ErrUnauthenticated)
	}
	if claims.Provider == "" || claims.Subject == "" || claims.Login == "" {
		return coachapi.Principal{}, fmt.Errorf("%w: incomplete principal claims", ErrUnauthenticated)
	}
	return coachapi.Principal{
		Provider: claims.Provider,
		Subject:  claims.Subject,
		Login:    claims.Login,
	}, nil
}

// Revoke denylists the token's jti until its exp so subsequent Validate calls fail.
func (s *Service) Revoke(ctx context.Context, token string) error {
	claims, err := s.parseClaims(token)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnauthenticated, err)
	}
	if claims.ID == "" {
		return fmt.Errorf("%w: missing jti", ErrUnauthenticated)
	}
	exp := time.Time{}
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	}
	if err := s.denylist.Revoke(ctx, claims.ID, exp); err != nil {
		return fmt.Errorf("%w: %v", ErrDenylistStore, err)
	}
	return nil
}

// InspectClaims returns Coach JWT claims without denylist checks (signature and
// issuer still verified; expiry is not enforced so operators can inspect expired tokens).
func (s *Service) InspectClaims(token string) (Claims, error) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	var cc coachClaims
	parsed, err := parser.ParseWithClaims(token, &cc, s.keyFunc)
	if err != nil {
		return Claims{}, err
	}
	if !parsed.Valid {
		return Claims{}, errors.New("authn: invalid token")
	}
	if cc.Issuer != s.issuer {
		return Claims{}, fmt.Errorf("authn: unexpected issuer %q", cc.Issuer)
	}
	out := Claims{
		Provider: cc.Provider,
		Subject:  cc.Subject,
		Login:    cc.Login,
		Issuer:   cc.Issuer,
		ID:       cc.ID,
	}
	if cc.ExpiresAt != nil {
		out.ExpiresAt = cc.ExpiresAt.Time
	}
	return out, nil
}

func (s *Service) parseClaims(token string) (*coachClaims, error) {
	// Skip library time validation so expiry uses the injected clock (tests and
	// operators control Now); still restrict alg via WithValidMethods.
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithoutClaimsValidation(),
	)
	var cc coachClaims
	parsed, err := parser.ParseWithClaims(token, &cc, s.keyFunc)
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	if cc.Issuer != s.issuer {
		return nil, fmt.Errorf("unexpected issuer %q", cc.Issuer)
	}
	if cc.ExpiresAt == nil {
		return nil, errors.New("missing exp")
	}
	now := s.now()
	if cc.ExpiresAt.Time.Before(now) || cc.ExpiresAt.Time.Equal(now) {
		return nil, errors.New("token is expired")
	}
	if cc.NotBefore != nil && cc.NotBefore.Time.After(now) {
		return nil, errors.New("token not yet valid")
	}
	return &cc, nil
}

func (s *Service) keyFunc(t *jwt.Token) (interface{}, error) {
	if t.Method != jwt.SigningMethodHS256 {
		return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
	}
	return s.key, nil
}

func newJTI() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("authn: generate jti: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

func requireAbsoluteURL(field, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("authn: %s must be an absolute URL", field)
	}
	return nil
}

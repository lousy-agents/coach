package authn

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"

	"time"
)

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

package authn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/lousy-agents/coach/internal/coachapi"
)

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

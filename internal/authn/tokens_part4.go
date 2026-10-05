package authn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"

	"github.com/lousy-agents/coach/internal/coachapi"
)

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

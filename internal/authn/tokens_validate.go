package authn

import (
	"context"
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

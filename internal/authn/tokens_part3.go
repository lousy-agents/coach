package authn

import (
	"context"

	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"

	"time"
)

func (s *Service) parseClaims(token string) (*coachClaims, error) {

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

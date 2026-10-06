package authn

import (
	"context"
	"fmt"
	"time"
)

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

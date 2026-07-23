package authn

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/lousy-agents/coach/internal/coachapi"
)

type ctxKey int

const principalKey ctxKey = 1

// PrincipalFromContext returns the Principal attached by Middleware, if any.
func PrincipalFromContext(ctx context.Context) (coachapi.Principal, bool) {
	p, ok := ctx.Value(principalKey).(coachapi.Principal)
	return p, ok
}

// Middleware validates the Authorization Bearer Coach JWT and attaches the
// Principal to the request context. Missing/invalid/expired/denylisted tokens
// yield 401 unauthenticated; denylist store errors yield 503 internal_error.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeAPIError(w, http.StatusUnauthorized, coachapi.ErrorCodeUnauthenticated, "missing or invalid authorization")
			return
		}
		p, err := s.Validate(r.Context(), raw)
		if err != nil {
			if errors.Is(err, ErrDenylistStore) {
				writeAPIError(w, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError, "authentication temporarily unavailable")
				return
			}
			writeAPIError(w, http.StatusUnauthorized, coachapi.ErrorCodeUnauthenticated, "unauthenticated")
			return
		}
		ctx := context.WithValue(r.Context(), principalKey, p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(h string) (string, bool) {
	const prefix = "Bearer "
	if h == "" || !strings.HasPrefix(h, prefix) {
		return "", false
	}
	tok := strings.TrimSpace(h[len(prefix):])
	if tok == "" {
		return "", false
	}
	return tok, true
}

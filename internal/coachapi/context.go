package coachapi

import "context"

type ctxKey int

const principalKey ctxKey = 1

// WithPrincipal returns a copy of ctx carrying p, retrievable via
// PrincipalFromContext.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// PrincipalFromContext returns the Principal attached by WithPrincipal, if
// any.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}

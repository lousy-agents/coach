package coachapi

import (
	"context"
)

// Principal is the only identity type job handlers and authz checks consume.
// v1 GitHub maps Provider="github", Subject=<numeric user id string>, Login=<github login>.
type Principal struct {
	Provider string `json:"provider"`
	Subject  string `json:"subject"`
	Login    string `json:"login"`
}

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

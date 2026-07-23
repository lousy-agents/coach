package authz

import (
	"context"
)

// BypassAuthorizer wraps inner and skips the live authorization check only
// for one exact, case-sensitive (owner, repo) pair -- Story 3's
// credential-free smoke / test-mint exception. It must be constructed
// explicitly by config-gated wiring (e.g. cmd/coach-api); this package never
// uses it as a default.
type BypassAuthorizer struct {
	inner RepoAuthorizer
	owner string
	repo  string
}

// NewBypassAuthorizer wraps inner, bypassing the live check only when the
// requested owner/repo case-sensitively equals the configured pair.
func NewBypassAuthorizer(inner RepoAuthorizer, owner, repo string) RepoAuthorizer {
	return &BypassAuthorizer{inner: inner, owner: owner, repo: repo}
}

// Authorize returns nil immediately for the configured bypass pair;
// otherwise it delegates to inner.
func (b *BypassAuthorizer) Authorize(ctx context.Context, login, owner, repo string) error {
	if owner == b.owner && repo == b.repo {
		return nil
	}
	return b.inner.Authorize(ctx, login, owner, repo)
}

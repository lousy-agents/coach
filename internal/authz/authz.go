// Package authz implements ADR-003's synchronous, submit-time repository
// authorization check: a principal may submit a scan of owner/repo only if
// the Coach GitHub App installation can read the repository and the
// principal has a GitHub-recognized role in it.
package authz

import (
	"context"
	"errors"
)

// ErrNotAuthorized indicates the principal has no legitimate relationship
// with the requested repository, per ADR-003: a nonexistent repo, a repo the
// Coach GitHub App installation cannot read, and a repo where the principal
// has no role all collapse to this single outcome.
var ErrNotAuthorized = errors.New("authz: principal not authorized for repository")

// RepoAuthorizer decides whether the principal identified by login may
// submit a scan of owner/repo. It takes a bare GitHub login rather than
// coachapi.Principal so this package has no dependency on internal/coachapi
// -- internal/coachapi/server.go depends on this package directly (to check
// errors.Is(err, ErrNotAuthorized)), and internal/coachapi is what issues
// Principal in the first place, so a Principal-typed parameter here would
// create an import cycle.
type RepoAuthorizer interface {
	// Authorize returns nil if allowed. Returns an error satisfying
	// errors.Is(err, ErrNotAuthorized) if the repo is nonexistent, the App
	// installation cannot read it, or the principal has no role in it (all
	// three collapse to repo_not_authorized per ADR-003). Any other non-nil
	// error is a transient/infrastructure failure (caller maps to 503) --
	// never persist a job in that case either.
	Authorize(ctx context.Context, login, owner, repo string) error
}

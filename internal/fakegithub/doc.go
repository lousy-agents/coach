// Package fakegithub is a Coach-owned, fixture-driven fake GitHub HTTP
// service for offline acceptance testing.
//
// It implements only the contracts Coach consumes: OAuth identity
// (authorize/token exchange, /user and /api/v3/user), GitHub App
// installation (token mint, repo→installation, effective permissions), and
// repository content reads (Contents API file fetch plus directory listing
// for pkg/githubingest symlink detection). Fixture.RejectedTokens is the
// seam for non-GitHub credentials (Coach JWT stand-ins) that must record as
// acceptanceharness.AuthModeRejected on every route.
//
// It is not a general-purpose GitHub emulator. Pull-request listing and
// changed-file reads are deferred to the PR History Scan epic.
//
// The package consumes internal/acceptanceharness's shared fixture and
// request-recording contract rather than inventing a competing one. See
// docs/architecture/acceptance-harness.md section 6.
package fakegithub

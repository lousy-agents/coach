// Package fakegithub is a Coach-owned, fixture-driven fake GitHub HTTP
// service for offline acceptance testing (GitHub issue #77, "Task 0.2:
// Implement the Coach-owned fake GitHub service", epic #73 "Feature Zero:
// Offline Acceptance Foundation").
//
// It implements only the GitHub contracts Coach actually consumes: OAuth
// identity (authorization-code/token exchange, "/user"), GitHub App
// installation/authorization (installation-token minting, repo-to-
// installation resolution, effective permissions), and repository content
// reads (the Contents API, both a single-file fetch and the directory
// listing pkg/githubingest also issues for symlink detection). It is
// explicitly not a general-purpose GitHub API emulator, and it explicitly
// does not yet cover pull-request listing or changed-file reads -- those
// are deferred to the PR History Scan epic per the Feature Zero epic doc.
//
// This package consumes internal/acceptanceharness's shared fixture and
// request-recording contract (FixtureHeader, AuthMode, RequestRecord,
// Recorder) rather than inventing a competing one -- see
// docs/architecture/acceptance-harness.md section 5.
package fakegithub

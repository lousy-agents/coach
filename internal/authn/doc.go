// Package authn implements Coach-signed JWT issue, validation, jti denylist
// revocation, HTTP auth middleware, optional GitHub OAuth App login (no scope),
// and a config-gated test-mint path for protected /v1 routes (ADR-001).
// It does not perform repository Contents reads (those stay in pkg/githubingest).
package authn

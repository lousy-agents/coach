# ADR-001: Coach API Authentication via GitHub OAuth App and Coach-JWT

| Field | Value |
| --- | --- |
| Status | Accepted |
| Date | 2026-07-19 |
| Deciders | Platform groundwork spec review |
| Supersedes | Static operator-provisioned bearer-token → GitHub-login binding tables as the primary auth path |

## Context

Coach's platform groundwork introduces an authenticated HTTP API (`/v1`) so that pilot engineers can submit async analysis jobs without installing local Go tooling. The prior spec draft relied on statically provisioned bearer-token tables bound to GitHub logins. That approach has no verified identity proof, is hard to rotate, and does not scale to additional identity providers or a self-serve web UI.

We need an authentication architecture that:

1. Proves the caller owns a real GitHub identity.
2. Issues a Coach-native credential that `/v1` handlers can validate locally.
3. Keeps the identity seam small enough that future OAuth2/OIDC providers (Google, Okta, etc.) plug in without rewriting job authorization.
4. Avoids over-scoping the user's OAuth grant.
5. Supports automated tests and local smoke without live GitHub round-trips.

## Decision

1. **GitHub OAuth App is the sole production identity provider in the groundwork phase.**
   - The browser completes GitHub's authorization-code flow against a Coach-configured OAuth App.
   - Coach exchanges the code, calls `GET /user`, and extracts the public `id` and `login`.
   - The OAuth App requests **no scope** in v1. Public `id` and `login` are available with the default public-read-only authorization; email and private profile fields are not used.

2. **Coach issues its own signed JWT bearer tokens.**
   - Claims carry a `Principal` (`provider=github`, `sub` = GitHub numeric user id, `login` = GitHub login), plus `iss`, `exp`, and a unique `jti`.
   - Protected `/v1` routes validate signature, issuer, expiry, and `jti` denylist.
   - The GitHub OAuth access token from the exchange is used only during login; it is **not** accepted as a `/v1` credential and is **not** forwarded to workers.

3. **Revocation is implemented via a JWT `jti` denylist.**
   - Logout/revoke inserts the `jti` into the denylist.
   - Protected routes reject a token whose `jti` is denylisted, even if the token is not yet expired.
   - Denylist availability is therefore required for revocation checks.

4. **A config-gated test-mint path is provided for non-production use only.**
   - When enabled by operator configuration (e.g., `COACH_AUTH_TEST_MINT=1` in the local compose profile), an endpoint can mint a Coach JWT for a supplied login/subject.
   - Disabled by default; when disabled the path is not registered and returns `404`.
   - This is the only supported static-token-like path, and it is explicitly not production.

## Consequences

- **Positive**: Verified identity, no long-lived static token tables, and a provider-shaped `Principal` seam that future identity providers can implement.
- **Positive**: Minimal OAuth scope reduces user consent friction and limits blast radius if an OAuth token leaks.
- **Positive**: Coach JWT validation is local and fast; handlers see a normalized `Principal`, not GitHub protocol details.
- **Negative**: Adds a JWT signing key and denylist store to operate and rotate.
- **Negative**: Revocation depends on the denylist store; a missing store cannot enforce revocation.
- **Tradeoff**: GitHub OAuth App cannot be used for repository reads; a separate GitHub App installation handles those (see ADR-002).

## Alternatives considered

| Alternative | Why rejected |
| --- | --- |
| Static bearer-token → login tables as primary auth | No verified identity, rotation burden, blocks future IdPs. |
| GitHub App user-to-server OAuth for identity | Over-complicated for pure identity; OAuth App is the standard user-login path. |
| Request `read:user` or `user:email` scope | Grants no v1-required capability; no-scope is sufficient and narrower. |
| Accept the GitHub OAuth access token as the `/v1` credential | Ties API credential lifetime to GitHub token lifetime and scope; prevents local validation and revocation. |
| Generic OIDC/SAML/passkeys day one | Out of scope for groundwork; `Principal` seam is designed to accommodate later. |

## Validation

- Acceptance tests prove the fake-GitHub OAuth round-trip produces a token that authorizes a protected route.
- Acceptance tests prove missing/invalid/expired/denylisted tokens return `401`.
- Acceptance tests prove the test-mint path returns `404` when disabled.

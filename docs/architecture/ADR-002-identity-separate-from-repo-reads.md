# ADR-002: Separate User Identity from Repository-Read Credentials

| Field | Value |
| --- | --- |
| Status | Accepted |
| Date | 2026-07-19 |
| Deciders | Platform groundwork spec review |

## Context

Coach needs two distinct capabilities from GitHub:

1. **Who is calling the API?** — verified end-user identity for authentication and self-serve authorization.
2. **What repository content can Coach read?** — server-side fetch of PR lists, file contents, and repository trees.

Conflating these two capabilities is tempting because both involve GitHub, but they have different trust boundaries, credential types, and privilege requirements. Using the user's OAuth token for repo reads would require broader scopes and would make Coach's data access depend on a user-granted token. Using a single GitHub App for both identity and repo read is awkward because GitHub Apps are designed for server-to-server repo access, not browser-based user login.

## Decision

Use **two separate GitHub integrations** and keep their credentials strictly apart:

| Integration | Role | Credential holder | Used by |
| --- | --- | --- | --- |
| **GitHub OAuth App** | End-user **identity** | Coach OAuth client id/secret; user grants access to their public GitHub identity | `cmd/coach-api`, `internal/authn` |
| **GitHub App installation** | Server-side **repository read** | App id + installation id + private key; held by Coach infrastructure | `pkg/githubingest`, worker, `internal/authz` |

Rules:

1. The user's GitHub OAuth access token is used only to verify identity during login. It is **never** used to read repository content, list PRs, or act as the user against the GitHub API.
2. The worker reads repositories exclusively via `pkg/githubingest`'s GitHub App installation auth, deriving short-lived installation tokens from the App private key.
3. `internal/authz.RepoAuthorizer` checks whether a `Principal` has a role in a repository using the GitHub App installation token, not the user's OAuth token.
4. A future credential mode for low-friction local setups (e.g., user OAuth token or plain PAT) may be added behind a `Credential`/`CredentialResolver` seam without re-architecting the queue or API.
5. Installation tokens are minted by exactly **one** credential seam (the `CredentialResolver` of rule 4, colocated with `pkg/githubingest`), consumed by both repository ingestion and `internal/authz`'s role checks. No other package loads the App private key — `cmd/coach-api` and `cmd/coach-worker` both reach installation tokens through this one seam, which is what the production credential broker (system-overview §5/§8) later replaces without a rewrite.

## Consequences

- **Positive**: Least privilege. The user's OAuth grant needs no repository scopes; the worker's installation token is scoped to repos the App installation can access.
- **Positive**: Clear security boundary. Identity code lives in `internal/authn`; repo-read code lives in `pkg/githubingest` and `internal/authz`.
- **Positive**: Server-side repo access does not depend on a user keeping a browser session alive or retaining an OAuth grant.
- **Positive**: Future local setups can opt into a different repo credential mode without changing the identity/authz model.
- **Negative**: Operators must configure and rotate two GitHub integrations (OAuth App + GitHub App).
- **Negative**: Repository authorization checks require the GitHub App installation to be able to read the repository, even for public repos.

## Alternatives considered

| Alternative | Why rejected |
| --- | --- |
| Use the user's OAuth token with `repo`/`public_repo` scope for repo reads | Requires over-broad user scopes; breaks the self-serve trust model; user can revoke access at any time. |
| Use a single GitHub App for both identity and repo read | GitHub Apps are not designed for browser-based user login; OAuth App is the correct tool for identity. |
| Plain PAT as primary repo-read credential | Fine as a future local fallback, but not acceptable as the primary production path due to key management and audit limitations. |

## Validation

- Package boundaries: `internal/authn` does not perform repository Contents reads; `pkg/githubingest` does not implement user OAuth.
- Acceptance tests prove worker PR/file fetches use the installation-token path, not the user's OAuth token.
- Acceptance tests prove `/v1` rejects the GitHub OAuth access token as an API credential.

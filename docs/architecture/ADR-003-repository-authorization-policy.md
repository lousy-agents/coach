# ADR-003: Repository Authorization Policy for Self-Serve Scans

| Field | Value |
| --- | --- |
| Status | Accepted |
| Date | 2026-07-19 |
| Deciders | Platform groundwork spec review |

## Context

The platform groundwork era is self-serve: a pilot engineer scans their own recent PRs or a repository they name. To prevent surveillance and unauthorized data access, the API must enforce that a principal can only request scans of repositories they legitimately have a role in.

The spec considered several options:

- **Option A**: No repository authorization beyond a valid token.
- **Option B**: The Coach GitHub App installation must be able to read the repository.
- **Option C**: The App installation must be able to read the repository **and** the authenticated principal must have a role in that repository according to GitHub.

The policy must also respect the "no developer scoring" and "no surveillance" principles in the PRD and architecture doc.

## Decision

Adopt **Option C full**: a `pr_history_scan` or `repo_baseline_scan` may only be submitted when **both** of the following are true:

1. The Coach GitHub App installation can read the requested repository.
2. The authenticated principal has a role in that repository according to GitHub.

"Has a role" includes:

- Direct collaborator access.
- Organization-derived access via team membership or base permissions.
- User-owned public/private repos where the principal is the owner.

Implementation rules:

- The check is performed **synchronously at submit time** by `internal/authz.RepoAuthorizer`, before the job is persisted.
- The check uses the GitHub App installation token.
- If the principal lacks access, or the App installation cannot read the repo, the API responds `403` with code `repo_not_authorized` and persists nothing.
- If GitHub returns `404` for the repo, the API shall map it to `403` with code `repo_not_authorized` — a nonexistent repository is deliberately indistinguishable from an unauthorized one, and a deterministic contract keeps the acceptance tests stable.
- The role check uses `GET /repos/{owner}/{repo}/collaborators/{username}/permission`, which returns the principal's *effective* permission including team- and org-derived access. The GitHub App manifest permission this endpoint requires must be verified against a real App installation during implementation and recorded here once confirmed; if it demands more than the intended minimal manifest, the fallback (org membership + team enumeration) needs org `Members` permission — a materially different manifest that would need a fresh decision.
- Owner/repo → installation resolution uses `GET /repos/{owner}/{repo}/installation` with the App JWT. A pilot spanning more than one org or user account therefore requires per-repo installation resolution, not a single statically configured installation id.
- Transient GitHub API failures during the check map to `503` or `internal_error`; the job is not persisted.
- No caching in v1; repeated GitHub API calls on submit are acceptable for pilot volume.

## Consequences

- **Positive**: Prevents scanning repositories the principal has no legitimate relationship with.
- **Deliberate**: public repositories where the principal has **no role** are also denied (`403` `repo_not_authorized`) — scan authorization is gated on relationship, not readability. The error message must state this actionably, or pilots will report "can't scan this OSS repo I read every day" as a bug. A rate-limited public-repo carve-out ("Option C-minus") exists in the baseline spec's Future Considerations, to be proposed only if pilot friction materializes.
- **Positive**: Enforces the no-surveillance principle in the API shape itself.
- **Positive**: Keeps the worker simple — it never starts a job the principal was not authorized to request.
- **Negative**: Submit latency depends on a synchronous GitHub API round-trip.
- **Negative**: Requires the GitHub App installation to be authorized in the target org/user account.
- **Tradeoff**: Per-principal repository allowlists are deferred; this policy relies entirely on GitHub's role resolution.

## Alternatives considered

| Alternative | Why rejected |
| --- | --- |
| Option A: no repo authz | Allows surveillance of arbitrary repos; violates trust principles. |
| Option B: App visibility only | A repo may be App-visible to Coach without the requesting user having any role in it; still enables surveillance. |
| Static `allowed_repos` allowlist per principal | Higher operator burden and does not scale; deferred to a later story. |
| Check authorization only at report read time | Would allow unauthorized jobs to run and consume resources; rejected. |

## Validation

- Acceptance tests cover happy paths: user-owned repo, org repo with direct collaborator access, org repo with team/base permissions.
- Acceptance tests cover unhappy paths: repo the principal has no role in, repo outside App installation, non-existent repo, transient GitHub failure.
- `internal/authz.RepoAuthorizer` has tests against a fake GitHub installation API.

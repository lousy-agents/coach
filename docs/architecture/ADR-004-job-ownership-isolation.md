# ADR-004: Job Ownership and Cross-Principal Read Isolation

| Field | Value |
| --- | --- |
| Status | Accepted |
| Date | 2026-07-19 |
| Deciders | Platform groundwork spec review |

## Context

Groundwork-era reports are private to the requester. There is no team dashboard, no manager view, and no anonymous audience. The API must guarantee that one authenticated principal cannot view another principal's job status or report.

Previously the spec deferred this question and allowed any provisioned token to `GET` any job id. Resolving it is a load-bearing privacy decision.

## Decision

Every job is owned by exactly one authenticated principal, persisted at submit time, and cross-principal reads are rejected.

Rules:

1. The `jobs` table stores a denormalized principal snapshot at submit time:
   - `created_by_provider`
   - `created_by_subject`
   - `created_by_login`
2. `GET /v1/jobs/{id}` returns the job status and metadata only if the authenticated principal matches `created_by_provider` and `created_by_subject` — the stable identifiers. `created_by_login` is denormalized for audit/display only and is **not** part of the match, so a GitHub login rename cannot lock an owner out of their prior jobs.
3. `GET /v1/jobs/{id}/report` returns the report under the same `provider` + `subject` match.
4. On mismatch, the API responds `403` (the requester is authenticated but not permitted).
5. There is no operator-facing multi-tenant view in the groundwork phase. A future `tenant_id` column or admin role can be added without redesigning the ownership check.

## Consequences

- **Positive**: Privacy by construction — reports are only visible to the requester.
- **Positive**: Simple authorization model: exact principal match on the stable `provider` + `subject` identifiers.
- **Positive**: Aligns with the PRD's self-serve constraint and no-scoring principle.
- **Negative**: No admin or support view of jobs in the groundwork phase.
- **Negative**: Principal identity must be stable across token reissues (provider + subject; login may drift).
- **Accepted**: because ownership mismatch returns `403` while a nonexistent id returns `404`, an authenticated principal can probe whether a job id exists. With UUIDv4 job ids, enumeration is impractical; this existence leak is an accepted consequence of the `403` decision. Status precedence is `401` → `404` → `403` → `409`, so job *status* never leaks to non-owners.
- **Tradeoff**: Changing a GitHub login mid-flight could orphan jobs; `subject` (numeric user id) is the stable identifier and `login` is stored for audit/display.

## Alternatives considered

| Alternative | Why rejected |
| --- | --- |
| Any authenticated principal can read any job | Violates privacy and self-serve trust model. |
| Role-based access with admin/operator reads | Adds complexity before multi-tenant needs are validated; deferred. |
| Share jobs by explicit invitation | Out of scope for groundwork; can be layered on later. |

## Validation

- Acceptance tests prove cross-principal `GET /v1/jobs/{id}` returns `403`.
- Acceptance tests prove cross-principal `GET /v1/jobs/{id}/report` returns `403`.
- Unit tests prove the store rejects reads by mismatched `created_by_*`.

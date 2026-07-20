# Feature: Coach API Platform — PR History Scan

## Problem Statement

The [Coach API Platform — Baseline Scan spec](coach-api-platform-baseline.spec.md) delivers the shared platform (GitHub OAuth identity, async job API, worker lifecycle, model gateway, agent tool loop, and seed rubrics) and the `repo_baseline_scan` capability. The sibling [Feature Zero: Offline Acceptance Foundation](coach-api-platform-acceptance-foundation.spec.md) supplies the shared fake GitHub, offline Compose acceptance, deterministic controls, and task conventions. This spec adds the second vertical slice: a self-serve scan of the authenticated pilot engineer's recent pull requests in a repository they have access to.

The PR-history scan is the richer customer-facing feature, but it adds the largest new GitHub-ingestion surface (PR listing by author, changed-file retrieval) and the strictest self-serve enforcement. It is intentionally split out so the baseline platform can be validated end-to-end first.

## Personas

| Persona | Impact | Notes |
| ------- | ------ | ----- |
| Pilot Engineer | Positive | Signs in with GitHub, submits a scan of their own last 10 PRs in a repo, and receives an async code-quality report without installing Go tooling |
| Platform Operator | Positive (secondary) | Reuses the same Docker Compose stack from the baseline spec; no new infrastructure |
| Human Reviewer | Positive (secondary) | Benefits indirectly when pilot engineers act on findings before requesting review |

## Value Assessment

- **Primary value**: Customer — surfaces recurring structural quality patterns across a pilot engineer's recent work, generating the product feedback needed to prioritize further investment.
- **Secondary value**: Future — proves the platform can support a second job kind with distinct ingestion and analysis paths without redesigning the shared seams (auth, job lifecycle, agent loop, gateway, rubrics).

## User Stories

### Story 2: Scan my last 10 pull requests

As a **Pilot Engineer**,
I want **a scan of the last 10 merged/open PRs I authored in a given repository**,
so that I can **see recurring code-quality signals across my recent work**.

#### Acceptance Criteria

- When a `pr_history_scan` job runs, the system shall list at most the 10 most recent pull requests that (a) were authored by the requested GitHub login in the requested repository and (b) are either **open** or **merged** (closed-unmerged PRs are excluded), ordered by most recently updated (`updated_at` descending), and analyze each PR's changed files (base and head sides) through the `pkg/semantics` → `pkg/codesignal` pipeline, with tool invocations going through `internal/agentloop` (see Design).
- When per-PR deterministic analysis completes, the system shall evaluate the results against the configured LLM-as-judge rubrics (via the model gateway / agent loop) and record the judgments alongside — never in place of — the deterministic findings.
- The coach-api shall accept a `pr_history_scan` only when the effective `author_login` equals the authenticated principal's verified GitHub `login` (from GitHub OAuth, [Baseline Scan spec Story 1](coach-api-platform-baseline.spec.md#story-1-authenticate-with-github-and-use-the-async-analysis-api)); if they differ, then the coach-api shall respond `403` (self-serve scans). If `author_login` is omitted from params, the coach-api shall default it to the principal's GitHub login. If the authenticated principal's provider is not `github`, then `pr_history_scan` shall respond `403`.
- The coach-api shall accept a `pr_history_scan` only when the principal has a role in the requested repository according to GitHub. The authorization check shall consider both direct collaborator access and organization-derived access (e.g., org membership with team or base permissions). If the principal lacks access, or if the Coach GitHub App installation cannot read the repository, the coach-api shall respond `403` with code `repo_not_authorized` and persist nothing.
- The API shall provide no endpoint to enumerate or scan arbitrary third-party authors or repositories.
- If GitHub returns fewer than 10 matching pull requests, then the system shall analyze the available set and record the actual count in `summary.pr_count` in the report.
- The PR listing shall page through at most a configurable lookback budget of recently-updated pull requests (default 200) while filtering by author — GitHub's list API cannot filter by author server-side — and if the budget is exhausted before `pr_limit` matches are found, the system shall record a diagnostic noting the truncated search and proceed with the matches found.
- If an individual PR's analysis fails (fetch error, unsupported language, oversized file), then the system shall record a per-PR diagnostic and continue with the remaining PRs rather than failing the whole job.

#### Notes

PR diff analysis does not require a local git checkout. The worker fetches base and head contents for each changed file via `pkg/githubingest` and invokes the `codesignal_report` tool with that content; the tool produces PR-level deterministic findings through the existing `pkg/semantics` → `pkg/codesignal` pipeline.

The self-serve constraint encodes the no-surveillance principle (PRD §11, architecture doc §11 "no developer scoring") into the API shape itself. Enforcement rests on **verified GitHub identity from the OAuth App login** ([Baseline Scan spec Story 1](coach-api-platform-baseline.spec.md#story-1-authenticate-with-github-and-use-the-async-analysis-api)), not on operator-provisioned static token bindings. The user's OAuth grant authenticates them *to Coach*; it is not used as the worker's credential to read repositories—that remains the GitHub App installation path in `pkg/githubingest`.

### Story 5: Trustworthy provenance (applied)

As a **Platform Operator**,
I want **deterministic findings and agent judgments kept structurally distinct**,
so that I can **always tell reproducible evidence apart from model opinion**.

For `pr_history_scan`, this means:
- Every per-PR deterministic finding is stored with `source=deterministic`.
- Every rubric judgment is stored with `source=agent`, plus `rubric_id`, `rubric_version`, and `model_identity`.
- A malformed judgment is recorded as a per-PR diagnostic and the deterministic report is still delivered for that PR and the job as a whole.

The full provenance rules are defined in [Baseline Scan spec Story 5](coach-api-platform-baseline.spec.md#story-5-trustworthy-provenance).

---

## Design

> Engineering standards: `AGENTS.md` (inlined into `CLAUDE.md`). Binding constraints respected here: `pkg/semantics` never imports `pkg/githubingest` (or any GitHub client) and vice versa; acceptance-test-first policy applies to every task; all commands are `mise` tasks.
>
> Shared platform design (auth, data model, agent loop, gateway, rubrics, queue, tenant scoping, and diagrams) lives in the [Baseline Scan spec](coach-api-platform-baseline.spec.md#design). This section covers only the PR-history-specific additions and variations.
>
> **Feature Zero dependency (binding):** PR History extends Feature Zero's fixture-driven fake GitHub service with PR lists and changed files, and uses its credential recorder, controlled clocks/durations, golden report conventions, and acceptance task categories. It shall not create a second fake GitHub implementation, host-only Compose `httptest` dependency, or alternate queue/agent-loop acceptance approach.

### Components Affected

- `pkg/githubingest/` — **extended**: PR listing by author and PR changed-file content retrieval (base and head), same GitHub App **installation** auth and sentinel-error conventions as `ReadFile` (unchanged by user OAuth).
- `internal/coachapi/handler_pr_history.go` — **new**: `pr_history_scan` job handler.
- `internal/agentloop/` — **extended**: add `github_list_prs` and `github_pr_files` model-selected tools to the registry.
- `internal/codesignalcli/` — **reused**: diff analysis path invoked by the worker (import is allowed; both live in this module).

### Dependencies

All infrastructure dependencies (Postgres, Redis, GitHub OAuth App, GitHub App installation, model gateway stub/llama.cpp) are inherited from the [Baseline Scan spec](coach-api-platform-baseline.spec.md#dependencies). [Feature Zero: Offline Acceptance Foundation](coach-api-platform-acceptance-foundation.spec.md) is also a direct prerequisite: its fake-GitHub fixture schema is extended for PR lists/files, while its offline/no-real-credentials contract and Compose/queue/agent-loop harness remain shared. No separate test infrastructure is introduced.

### Data Model Changes

The `pr_history_scan` job kind extends the shared `jobs`, `job_findings`, and `job_diagnostics` schema from the [Baseline Scan spec](coach-api-platform-baseline.spec.md#data-model-changes) without changing its shape. The only additions are:

- `kind = pr_history_scan` in the `jobs` table (API + worker must accept this kind; baseline Task 1 may only define `repo_baseline_scan` until this slice lands).
- `params` schema for `pr_history_scan` (see below).
- `scope` values in `job_diagnostics` may include `pr:123` to identify the PR a diagnostic belongs to.
- Report `summary.pr_count` (int) — the number of PRs **selected for analysis** after the open/merged + author filter; PRs whose analysis subsequently fails remain counted here and carry per-PR diagnostics. `summary.pr_failed_count` (int, omitted when zero) — the number of selected PRs whose analysis failed. (Both are optional named fields of the baseline `summary` struct per its freeze policy.)
- Each analyzed PR's resolved **base and head commit SHAs** are recorded (in the finding payloads or the `pr:<n>` diagnostic/summary entries) so per-PR findings are reproducible after branches move.

Per-kind `params` schemas (validated at submit; violations → `400`, nothing persisted):

- `pr_history_scan`: `repo_owner` (string, required), `repo_name` (string, required), `author_login` (string, optional; default = authenticated principal's GitHub `login`; if present must equal that login per Story 2 or → `403`), `pr_limit` (int, optional, default 10, max 10 in v1).

**Submit-time checks (API layer, before persist)** — same durability/enqueue rules as baseline Design:

1. Principal `provider` must be `github` else `403`.
2. Effective `author_login` must equal `principal.login` else `403`.
3. `RepoAuthorizer` (ADR-003) must allow the repo else `403` `repo_not_authorized`.
4. Params schema validation else `400`.

The top-level report shape is identical to the baseline spec, with the addition of `summary.pr_count` for the actual number of PRs analyzed.

**Orchestration split (agent loop vs fixed handler code):**

- **Handler-driven via `internal/agentloop`** (guaranteed coverage): the per-PR deterministic pass — `github_list_prs`, `github_pr_files`, `semantics_analyze`, `codesignal_report` over each selected PR — and the rubric-judgment tools (`hidden_mutation_contextualization`, `change_cohesion`). The handler drives these through the registry/loop before and independent of any model-selected activity, so the deterministic report survives total gateway failure (Baseline Story 5).
- **Model-selected via `internal/agentloop`** (supplemental evidence): during rubric judgment the model may re-invoke `github_pr_files`, `semantics_analyze`, `codesignal_report` to gather additional evidence. Unknown tools and over-budget loops are typed errors.
- **Deterministically owned by the job handler / API layer** (not model-selected): open/merged PR filter policy, self-serve author check at submit (`principal.login`), per-PR diagnostic-and-continue behavior, and terminal status transitions.

### Decisions

This spec inherits all decisions from the [Baseline Scan spec](coach-api-platform-baseline.spec.md#decisions-see-adrs-for-rationale-and-alternatives) and adds none.

### Open Questions

- [ ] **Per-principal repo allowlists**: Deferred from the baseline spec; remains deferred. Repository role checks via GitHub App installation are the authorization boundary for `pr_history_scan`.

---

## Tasks

> Each task must start with a failing acceptance test (repo policy — see `AGENTS.md` "Acceptance-test-first"). Verification commands are the repo's `mise` tasks.

### Task 6: PR listing and PR file retrieval in `pkg/githubingest`

**Objective**: Add `ListRecentPullRequestsByAuthor(owner, repo, login, limit)` and per-PR changed-file content retrieval (base and head), following the package's existing auth and sentinel-error conventions.

**Context**: The largest missing ingestion capability; unblocks Story 2. Independent of the shared platform tasks, but depends on Feature Zero's fake-GitHub fixture/recorder and the baseline platform to exercise end-to-end.

**Depends on**: Feature Zero Tasks 0.1–0.2; Baseline Scan acceptance conventions

**Affected files**:

- `pkg/githubingest/pulls.go`, `pulls_test.go`, `acceptance_test.go` (extended)

**Requirements**:

- Story 2: at most `limit` most recent **open or merged** PRs by the given author, ordered by `updated_at` descending; closed-unmerged PRs must not appear; fewer eligible → return what exists.
- Configurable pagination lookback budget (default 200 recently-updated PRs scanned) with a typed truncation result the handler records as a diagnostic when the budget is reached before `limit` matches.
- Acceptance tests extend Feature Zero's shared fake-GitHub fixture with a mixed-state fixture (open, merged, closed-unmerged) proving closed-unmerged exclusion and ordering; they do not create a second GitHub test approach.
- Errors map to the package's existing sentinels (`ErrNotFound`, `ErrAuth`, `ErrTooLarge`, …); no import of `pkg/semantics` (dependency rule).

**Verification**:

- [ ] `mise run test` and `mise run test-examples` pass; red first against Feature Zero's shared fake-GitHub fixture (in-process for package tests)
- [ ] Mixed-state fixture asserts closed-unmerged exclusion
- [ ] `mise run tidy-check` clean

**Done when**:

- [ ] All verification steps pass
- [ ] Doc comment updates reflect the widened package scope

---

### Task 6a: Register `pr_history_scan` on the shared API contract

**Depends on**: Feature Zero Tasks 0.1–0.2; Baseline Tasks 1–2 (job domain + HTTP service) and Baseline Task 3a (enqueue path).

**Objective**: Extend the frozen job/API contract so `POST /v1/jobs` accepts `kind=pr_history_scan` with its params schema, submit-time self-serve + repo authz checks, and report `summary.pr_count` — without implementing the worker handler yet.

**Affected files**:

- `internal/coachapi/types.go`, golden fixtures, server/handler tests
- migrations only if kind constraints are DB-enforced

**Requirements**:

- Story 2 submit-time criteria: default `author_login`, reject mismatched author (`403`), reject non-`github` provider (`403`), `RepoAuthorizer` gate (`403` `repo_not_authorized`), params validation (`400`), successful submit → `202` + enqueue.
- Report JSON contract gains optional/kind-specific `summary.pr_count` without breaking baseline golden fixtures (`report_version` stays `"1"`; unknown fields remain forward-compatible per existing freeze rules).

**Verification**:

- [ ] `mise run test` red first on new kind/authz cases, then green
- [ ] Baseline `repo_baseline_scan` contract tests still pass

**Done when**:

- [ ] API accepts and authorizes `pr_history_scan` submits; worker may still no-op or fail unknown handler until Task 7

---

### Task 7: `pr_history_scan` job handler

**Depends on**: Feature Zero Tasks 0.1–0.4; Baseline Tasks 1, 2a, 2, 3a, 3, 4, 5, and 9 (enumerated — letter-suffixed tasks included), plus Task 6 and Task 6a of this spec.

**Objective**: Run list open-or-merged PRs → fetch changed files → per-PR semantics/codesignal analysis → rubric judgment → attempt-scoped provenance-tagged report **through `internal/agentloop`** (registered tools + stub gateway sequences).

**Context**: The second end-user capability; proves the platform can support a distinct job kind without redesigning shared seams.

**Affected files**:

- `internal/coachapi/handler_pr_history.go`, plus tests
- `internal/agentloop/` tool registration for `github_list_prs`, `github_pr_files`

**Requirements**:

- Story 2 acceptance criteria end-to-end with authenticated principal + Feature Zero fake GitHub (installation API) + scripted stub gateway/recording tool registry driving the agent loop, including per-PR diagnostic-and-continue, open/merged filter, and self-serve author constraint at submit (`principal.login`).
- Acceptance test asserts the analysis path executes via `internal/agentloop` (tool registry), not by the handler calling `pkg/semantics` / `pkg/githubingest` / rubrics directly for that path.
- Worker path uses GitHub App installation credentials only — not the user's OAuth token.
- Open/merged filter policy remains handler-owned (not model-selected), matching ADR-005.

**Verification**:

- [ ] `mise run test` passes; the full-flow acceptance test was red first
- [ ] Report golden fixture shows deterministic and agent findings side by side
- [ ] Mixed-state PR set excludes closed-unmerged
- [ ] Agent-loop path asserted (no analysis bypass)

**Done when**:

- [ ] All verification steps pass
- [ ] Job completes with partial results when one PR fails

---

## Out of Scope

- Everything already covered by the [Baseline Scan spec](coach-api-platform-baseline.spec.md#out-of-scope).
- New `pkg/semantics` languages or new deterministic rules (tracked separately).
- General-purpose "scan any author's PRs" endpoints; self-serve enforcement is binding.

## Future Considerations

- Increase `pr_limit` above 10 once usage validates cost and latency.
- Add PR-level filtering (e.g., date range, merged-only) once pilots request it.
- Surface the currently-unemitted `pkg/semantics` findings as additional `pkg/codesignal` rules for richer per-PR rubric evidence.

# Coach PRD — Platform Groundwork Era (v2)

> Supersedes the v1 "Private Review-Readiness Coach" draft PRD. This revision reflects the product direction set in July 2026: decouple end-user consumption from the feedback platform, validate the full agentic flow locally before cloud investment, and park review-readiness verdicts in favor of async code-quality analysis. Implementation status claims follow the evidence hierarchy: only behavior locked by passing acceptance tests counts as implemented.

> **Shipped-behavior evidence:** [`evaluations/codesignal-pilot-readiness.html`](evaluations/codesignal-pilot-readiness.html) is the living leave-pilot evaluation of the shipped `coach codesignal` CLI — last reviewed 2026-08-31 at `main` = `e5bfa4a`. Gaps are tracked in epic [#282](https://github.com/lousy-agents/coach/issues/282); the standing decision is that all of them close before CodeSignal leaves pilot.

## 1. Product Purpose

AI coding assistants make it easy to produce large code changes quickly, but they do not reliably produce changes that are easy for humans to review, trust, or maintain. Coach helps engineers understand and improve the quality of AI-assisted code.

Coach is built as two separable things:

1. **A feedback platform** — deterministic structural analysis (`pkg/semantics` → `pkg/codesignal`) combined with LLM-as-judge rubric evaluation, exposed through a versioned Coach API that runs analysis jobs asynchronously.
2. **Consumption surfaces** — how people receive that feedback. Today: a local CLI (`coach codesignal`). Next: the Coach API consumed directly. Later: hooks into agent harnesses and a web UI for viewing feedback.

This separation is deliberate: the platform earns trust through the quality of its analysis, while consumption surfaces can multiply without changing it.

For the current release posture, the local `coach codesignal` CLI is the verified
consumer for deterministic signals. The API and worker surfaces are specified
future consumers; implementing a rule in the shared analyzer/report packages is
not, by itself, evidence that a hosted or API job path delivers that rule.

## 2. One-Sentence Positioning

An async code-quality coach that combines deterministic structural analysis with LLM-as-judge rubric evaluation over your recent pull requests and repositories — private to you, and honest about what is evidence versus opinion.

For deterministic analysis, “evidence” means reproducible output for pinned
source, analyzer, and rule versions. It does not mean semantic completeness,
runtime proof, or the absence of false positives and false negatives.

## 3. Target User (this era)

The primary customer right now is the project owner and a small pool of like-minded engineers who want to experiment and give feedback. Everything is self-serve: an engineer scans **their own** recent PRs or a repository they have a role in per GitHub (the Coach GitHub App must also be installed for that repository — part of pilot onboarding). There is no team rollout, no manager view, and no anonymous audience to design for yet.

This constraint is a feature: it keeps the trust posture simple (you see your own results), keeps feedback loops short, and defers every multi-tenant question until the analysis itself proves valuable.

## 4. Core Problem

AI-generated changes often carry structural quality problems that are invisible until review: hidden input mutation, tangled coupling, unclear change scope, tests that mirror implementation instead of behavior. Engineers lack a fast, private way to get evidence-grounded quality feedback across their recent work — not one PR at a time in public review comments, but asynchronously, across the last N changes, with a clear line between reproducible findings and model judgment.

Filesystem check-then-act races are one concrete security and correctness pattern
in this problem space: a path is checked and then used later under the assumption
that the checked state still holds.

## 5. Product Hypothesis

If an engineer can ask a private platform "analyze my last 10 PRs" or "baseline this repo" and get back a report that combines deterministic signals with well-reasoned rubric judgments, they will act on it and come back voluntarily. Voluntary repeat use by the pilot pool is the proof point — not coverage, not verdicts.

## 6. Differentiated Wedge

Most AI review tools comment publicly on open PRs. Linters enforce rules. CI gates pass/fail. Coach's wedge in this era:

1. **Async and retrospective** — analysis over a person's recent history and repo baselines, not just the PR currently under review.
2. **Provenance-separated** — every finding is tagged `deterministic` (reproducible, rule-versioned) or `agent` (rubric id + version + model identity); agent output can never overwrite or suppress a deterministic finding.
3. **Self-serve and private** — results go only to the authenticated requester. The platform performs no GitHub writes: no comments, no checks, nothing a teammate can see.
4. **Locally verifiable** — the entire stack (API, worker, Postgres, Redis queue, and a deterministic model stub — optionally native llama.cpp for real judgments) runs on a laptop via Docker Compose before any cloud deployment exists.

## 7. Product Surface (this era)

The intended Coach API (`/v1`), fronting an async job platform, is the next
consumption surface after the local CLI preview:

- `repo_baseline_scan` — analyze a whole repository at a ref. **Roadmap order: first API slice**: it validates every load-bearing platform seam (auth, queue, worker, agent loop, gateway, rubrics, compose smoke) against the smallest GitHub-ingestion surface.
- `pr_history_scan` — analyze the last 10 open-or-merged PRs authored by the requester in a repository they have a role in. **Roadmap order: second API slice**, on the validated platform.

Reports combine deterministic codesignal findings with LLM-as-judge rubric judgments. Full contracts: `.github/specs/coach-api-platform-baseline.spec.md` and `.github/specs/coach-api-platform-pr-history.spec.md` (index: `.github/specs/coach-api-platform-groundwork.spec.md`).

The local CLI is the current verified deterministic consumption surface. The
specified API/worker report path is intended to reuse the same signal contract,
but TOCTOU-specific API/worker delivery is not independently verified yet. The
TOCTOU rule adds no endpoint, job kind, queue message, model-gateway call,
schema migration, or GitHub write.

Across consumption surfaces, an empty deterministic signal set means that no
matched signal was produced for the inputs that were analyzed. Unsupported,
skipped, or unanalyzable inputs remain distinct diagnostics; an empty set is not
a safety verdict.

Consumption is pull-only (submit, poll, fetch report). Harness hooks and a web UI are future consumers of this same API, not part of this era.

## 8. Core Capabilities

Status in this table describes evidence at each layer: **Implemented** means
package or CLI behavior is locked by passing acceptance tests; **pilot/preview**
qualifies the current release posture; **Specified** means intended platform
behavior without feature-specific acceptance evidence.

| Capability | Status (evidence standard: passing acceptance tests) |
| --- | --- |
| Deterministic structural analysis, Go/TS/TSX (`pkg/semantics`) | **Implemented** — metrics, imports, findings; frozen JSON contract |
| Diff-aware deterministic signal reports with lifecycle (`pkg/codesignal` + `coach codesignal` CLI) | **Implemented — pilot/preview** — merge-base diffing, scope filtering, baseline mode, and versioned signals across state, coupling, structure, complexity, and security categories |
| TOCTOU check-then-act security signal (Go/TS/TSX) | **Implemented — pilot/preview advisory** — `security.toctou_check_then_act` emits a medium-severity, provenance-tagged signal with evidence, remediation guidance, and existing diff/baseline lifecycle handling for selected syntactic filesystem patterns |
| Single-file GitHub App ingestion (`pkg/githubingest`) | **Implemented** |
| Coach API, worker, job model | **Specified** — see baseline spec |
| GitHub OAuth identity → Coach-signed JWT (`Principal`, `jti` revocation, job ownership) | **Specified** — ADR-001/002/004 |
| `TaskQueue` port over Watermill (Redis Streams + SQS adapters) | **Specified** — ADR-006 |
| PR listing / PR file retrieval | **Specified** — see PR History spec |
| Minimal agent tool loop + model gateway (stub, llama.cpp) | **Specified** |
| LLM-as-judge rubrics (versioned, schema-validated) | **Specified** — two seed rubrics |
| Docker Compose stack + E2E smoke | **Specified** |
| Deterministic rule coverage and precision follow-up | **Planned/ongoing** — prioritize additional supported patterns and false-positive/false-negative work from pilot-corpus evidence; TOCTOU coverage and TypeScript binding precision remain follow-up work in [#205](https://github.com/lousy-agents/coach/issues/205) |
| SGLang/Qwen serving, AWS deployment | **Planned** — gated on compose-stack validation |
| Harness hooks, web UI | **Planned** — future API consumers |

The current deterministic CodeSignal report path can emit these versioned rule
IDs: `state.hidden_input_mutation`, `coupling.tight_constructor_init`,
`structure.constructor_density`, `structure.pointer_return_density`,
`security.toctou_check_then_act`, `complexity.max_nesting_depth`,
`complexity.branch_density`, `coupling.deep_relative_import`,
`complexity.cognitive_complexity`, and
`structure.react_component_orchestration_density`. This inventory is not a
claim that every rule matches every file; individual rules remain language-,
threshold-, and pattern-specific.

## 9. Explicitly Parked: Review-Readiness Digest

The v1 PRD's centerpiece — a five-section review-readiness digest with a readiness verdict — is parked, not abandoned. Reasons, from the grounded analysis:

- The readiness verdict had no defensible decision rule; a wrong verdict burns trust faster than no verdict.
- Behavioral test-gap detection (the declared primary capability) is a judgment task with no v1-honest deterministic proxy yet; rubric infrastructure built in this era is the prerequisite for attempting it credibly.
- The "private digest on a draft PR" delivery story conflicted with GitHub's visibility model (checks/comments are repo-visible). The pull-only API resolves privacy by construction; the digest can return when a delivery channel that honors it exists.

Behavioral evidence remains the long-term differentiator. This era builds the platform it will run on.

## 10. Non-Goals (unchanged in spirit from v1)

- No management dashboard; no developer scoring; no per-person productivity metrics — ever. The API shape enforces self-serve scans (OAuth-verified author identity: scans are bound to the GitHub login Coach verified at sign-in, plus a GitHub-role check on the target repository) precisely so the platform cannot quietly become surveillance.
- No auto-approval, no merge blocking, no CI replacement.
- No comprehensive security scanner or TOCTOU completeness guarantee; absence of a deterministic signal is not a safety verdict.
- No GitHub writes of any kind in this era.
- No style policing; no universal architecture enforcement.
- No new analysis languages this era (Go/TS/TSX only, per the `pkg/semantics` registry).

## 11. Trust Principles

- **Self-serve by construction** — you scan yourself or a repo you have a role in (per GitHub); the API refuses cross-author scans and unauthorized repositories.
- **Provenance over polish** — deterministic output is reproducible and rule-versioned; it is not semantic proof or a completeness claim, and model opinion is never blended into it.
- **Behavior over style** — rubrics judge structural and behavioral quality, not formatting.
- **Coverage honesty** — an absent signal may mean an unsupported pattern, skipped or unanalyzable input, or no match; it is never a safety verdict.
- **Fewer, better findings** — a short, actionable report is a pilot hypothesis to measure, not proof that the analysis is better.
- **Advisory security signals** — security-category rules explain narrow, reproducible patterns; they do not provide runtime proof, block CI or merges, or trigger GitHub writes.
- **Degrade honestly** — if the model fails rubric schema validation, the deterministic report still ships, with the failure recorded as a diagnostic.

## 12. Success Signals (this era)

- The compose stack's E2E smoke passes in CI against the core (stub) profile **and** on the operator's machine against the `llm` (llama.cpp) profile with at least one schema-valid agent judgment — together, the gate for SGLang/AWS investment. A stub-only smoke proves plumbing, not real-model rubric behavior.
- Pilot engineers run scans voluntarily more than once, and at least one rubric judgment per scan is rated useful by its requester (collected out-of-band during pilot check-ins; no in-product feedback mechanism this era).
- Pilot-corpus review records confirmed true positives, false positives, known misses, and user action for new deterministic rules—especially security-category rules—before broader coverage or security claims are made.
- Zero findings presented as deterministic that are not reproducible from the recorded analyzer/rule versions and the report's pinned commit SHA (per-PR scans pin base/head SHAs).
- The operator can add a new rubric or tool to the platform without touching the API contract or worker lifecycle (the groundwork seams hold).

## 13. Roadmap

1. **Now — platform groundwork**: the Baseline Scan slice first (shared platform + `repo_baseline_scan` + compose smoke), then the PR History slice (`pr_history_scan`) — see the spec index at `.github/specs/coach-api-platform-groundwork.spec.md`.
2. **Next**: deterministic-rule coverage and precision follow-up based on pilot evidence (including [TOCTOU follow-up #205](https://github.com/lousy-agents/coach/issues/205)); more rubrics; SGLang/Qwen behind the same gateway; AWS deployment per the architecture doc; additional platform tools/skills.
3. **Later**: harness hooks (e.g., an MCP surface over the API), web UI for viewing feedback, revisiting the review-readiness digest — including behavioral test-gap detection — on top of proven rubric infrastructure, and the GitHub-event-driven ingestion plane.

## 14. Relationship to the System Design

`docs/architecture/system-overview.md` describes the full GitHub-native, webhook-driven platform (ingestion, orchestration, SGLang serving, AWS deployment). This PRD's era implements a deliberately trimmed slice of it — same principles (deterministic-before-inference, provenance separation, gateway-contract model access, no scoring), smaller machinery (API trigger instead of webhooks, an application-owned `TaskQueue` port over Watermill with Redis Streams and SQS adapters instead of raw SQS, llama.cpp instead of SGLang). Job state, findings, diagnostics, and the JWT `jti` denylist remain in Postgres. The design doc's §14 phasing records where the groundwork phase sits relative to the rest.

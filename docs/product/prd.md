# Coach PRD — Platform Groundwork Era (v2)

> Supersedes the v1 "Private Review-Readiness Coach" draft PRD. This revision reflects the product direction set in July 2026: decouple end-user consumption from the feedback platform, validate the full agentic flow locally before cloud investment, and park review-readiness verdicts in favor of async code-quality analysis. Implementation status claims follow the evidence hierarchy: only behavior locked by passing acceptance tests counts as implemented.

## 1. Product Purpose

AI coding assistants make it easy to produce large code changes quickly, but they do not reliably produce changes that are easy for humans to review, trust, or maintain. Coach helps engineers understand and improve the quality of AI-assisted code.

Coach is built as two separable things:

1. **A feedback platform** — deterministic structural analysis (`pkg/semantics` → `pkg/codesignal`) combined with LLM-as-judge rubric evaluation, exposed through a versioned Coach API that runs analysis jobs asynchronously.
2. **Consumption surfaces** — how people receive that feedback. Today: a local CLI (`coach codesignal`). Next: the Coach API consumed directly. Later: hooks into agent harnesses and a web UI for viewing feedback.

This separation is deliberate: the platform earns trust through the quality of its analysis, while consumption surfaces can multiply without changing it.

## 2. One-Sentence Positioning

An async code-quality coach that combines deterministic structural analysis with LLM-as-judge rubric evaluation over your recent pull requests and repositories — private to you, and honest about what is evidence versus opinion.

## 3. Target User (this era)

The primary customer right now is the project owner and a small pool of like-minded engineers who want to experiment and give feedback. Everything is self-serve: an engineer scans **their own** recent PRs or a repository they choose. There is no team rollout, no manager view, and no anonymous audience to design for yet.

This constraint is a feature: it keeps the trust posture simple (you see your own results), keeps feedback loops short, and defers every multi-tenant question until the analysis itself proves valuable.

## 4. Core Problem

AI-generated changes often carry structural quality problems that are invisible until review: hidden input mutation, tangled coupling, unclear change scope, tests that mirror implementation instead of behavior. Engineers lack a fast, private way to get evidence-grounded quality feedback across their recent work — not one PR at a time in public review comments, but asynchronously, across the last N changes, with a clear line between reproducible findings and model judgment.

## 5. Product Hypothesis

If an engineer can ask a private platform "analyze my last 10 PRs" or "baseline this repo" and get back a report that combines deterministic signals with well-reasoned rubric judgments, they will act on it and come back voluntarily. Voluntary repeat use by the pilot pool is the proof point — not coverage, not verdicts.

## 6. Differentiated Wedge

Most AI review tools comment publicly on open PRs. Linters enforce rules. CI gates pass/fail. Coach's wedge in this era:

1. **Async and retrospective** — analysis over a person's recent history and repo baselines, not just the PR currently under review.
2. **Provenance-separated** — every finding is tagged `deterministic` (reproducible, rule-versioned) or `agent` (rubric id + version + model identity); agent output can never overwrite or suppress a deterministic finding.
3. **Self-serve and private** — results go only to the authenticated requester. The platform performs no GitHub writes: no comments, no checks, nothing a teammate can see.
4. **Locally verifiable** — the entire stack (API, worker, Postgres, llama.cpp) runs on a laptop via Docker Compose before any cloud deployment exists.

## 7. Product Surface (this era)

The Coach API (`/v1`), fronting an async job platform:

- `pr_history_scan` — analyze the last 10 PRs authored by the requester in a chosen repository.
- `repo_baseline_scan` — analyze a whole repository at a ref.

Reports combine deterministic codesignal findings with LLM-as-judge rubric judgments. Full contract: `.github/specs/coach-api-platform-groundwork.spec.md`.

Consumption is pull-only (submit, poll, fetch report). Harness hooks and a web UI are future consumers of this same API, not part of this era.

## 8. Core Capabilities

| Capability | Status (evidence standard: passing acceptance tests) |
| --- | --- |
| Deterministic structural analysis, Go/TS/TSX (`pkg/semantics`) | **Implemented** — metrics, imports, findings; frozen JSON contract |
| Diff-aware signal reports with lifecycle (`pkg/codesignal` + `coach codesignal` CLI) | **Implemented** — one rule surfaced (`hidden_input_mutation`); merge-base diffing, scope filtering, baseline mode |
| Single-file GitHub App ingestion (`pkg/githubingest`) | **Implemented** |
| Coach API, worker, job model | **Specified** — see groundwork spec |
| PR listing / PR file retrieval | **Specified** |
| Minimal agent tool loop + model gateway (stub, llama.cpp) | **Specified** |
| LLM-as-judge rubrics (versioned, schema-validated) | **Specified** — two seed rubrics |
| Docker Compose stack + E2E smoke | **Specified** |
| Surfacing remaining deterministic findings (`tight_coupling`, complexity/nesting, constructor patterns) as rules | **Planned** — cheap follow-on; rubric evidence |
| SGLang/Qwen serving, AWS deployment | **Planned** — gated on compose-stack validation |
| Harness hooks, web UI | **Planned** — future API consumers |

## 9. Explicitly Parked: Review-Readiness Digest

The v1 PRD's centerpiece — a five-section review-readiness digest with a readiness verdict — is parked, not abandoned. Reasons, from the grounded analysis:

- The readiness verdict had no defensible decision rule; a wrong verdict burns trust faster than no verdict.
- Behavioral test-gap detection (the declared primary capability) is a judgment task with no v1-honest deterministic proxy yet; rubric infrastructure built in this era is the prerequisite for attempting it credibly.
- The "private digest on a draft PR" delivery story conflicted with GitHub's visibility model (checks/comments are repo-visible). The pull-only API resolves privacy by construction; the digest can return when a delivery channel that honors it exists.

Behavioral evidence remains the long-term differentiator. This era builds the platform it will run on.

## 10. Non-Goals (unchanged in spirit from v1)

- No management dashboard; no developer scoring; no per-person productivity metrics — ever. The API shape enforces self-serve scans (token-bound author identity) precisely so the platform cannot quietly become surveillance.
- No auto-approval, no merge blocking, no CI replacement.
- No GitHub writes of any kind in this era.
- No style policing; no universal architecture enforcement.
- No new analysis languages this era (Go/TS/TSX only, per the `pkg/semantics` registry).

## 11. Trust Principles

- **Self-serve by construction** — you scan yourself or a repo you name; the API refuses cross-author scans.
- **Provenance over polish** — deterministic evidence and model opinion are never blended.
- **Behavior over style** — rubrics judge structural and behavioral quality, not formatting.
- **Fewer, better findings** — a short, high-confidence report beats an exhaustive one.
- **Degrade honestly** — if the model fails rubric schema validation, the deterministic report still ships, with the failure recorded as a diagnostic.

## 12. Success Signals (this era)

- The compose stack's E2E smoke passes in CI and on the operator's machine — the gate for SGLang/AWS investment.
- Pilot engineers run scans voluntarily more than once, and at least one rubric judgment per scan is rated useful by its requester.
- Zero findings presented as deterministic that are not reproducible from the recorded analyzer/rule versions.
- The operator can add a new rubric or tool to the platform without touching the API contract or worker lifecycle (the groundwork seams hold).

## 13. Roadmap

1. **Now — platform groundwork**: everything in `.github/specs/coach-api-platform-groundwork.spec.md` (API, worker, gateway, agent loop, two seed rubrics, compose stack, smoke).
2. **Next**: more deterministic rules from existing semantics findings; more rubrics; SGLang/Qwen behind the same gateway; AWS deployment per the architecture doc; additional platform tools/skills.
3. **Later**: harness hooks (e.g., an MCP surface over the API), web UI for viewing feedback, revisiting the review-readiness digest — including behavioral test-gap detection — on top of proven rubric infrastructure, and the GitHub-event-driven ingestion plane.

## 14. Relationship to the System Design

`docs/architecture/system-overview.md` describes the full GitHub-native, webhook-driven platform (ingestion, orchestration, SGLang serving, AWS deployment). This PRD's era implements a deliberately trimmed slice of it — same principles (deterministic-before-inference, provenance separation, gateway-contract model access, no scoring), smaller machinery (API trigger instead of webhooks, Postgres-as-queue instead of SQS, llama.cpp instead of SGLang). The design doc's §14 phasing records where the groundwork phase sits relative to the rest.

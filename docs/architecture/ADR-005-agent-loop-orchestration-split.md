# ADR-005: Agent Loop Orchestration Split

| Field | Value |
| --- | --- |
| Status | Accepted |
| Date | 2026-07-19 |
| Deciders | Platform groundwork spec review |

## Context

The platform runs deterministic structural analysis first, then LLM-as-judge rubric evaluation. The rubric step is agentic: a model issues tool calls to fetch evidence and render judgments. We need a boundary that gives the model enough freedom to gather evidence while keeping product policy, authz, and lifecycle decisions in deterministic Go code.

A fully model-driven orchestrator would risk the model bypassing authz, choosing wrong rubrics, or running indefinitely. A fully fixed handler would lose the flexibility that makes rubric judgment valuable.

## Decision

Split control into three clearly separated layers inside `internal/agentloop`:

### 1. Handler-driven tools (guaranteed coverage)

The job handler registers and **drives** tools through the registry/loop, before and independent of any model-selected activity:

- The **full deterministic analysis pass**: `semantics_analyze`/`codesignal_report` over every in-scope file (for PR history, also `github_list_prs`/`github_pr_files` over each selected PR). Driving this from the handler — not the model — is what guarantees the deterministic report exists even if the model never issues a tool call or the gateway is down entirely ("determinism before inference"; the degrade-honestly guarantee in baseline Story 5 is otherwise unimplementable).
- The **rubric-judgment tools**: `hidden_mutation_contextualization`, `change_cohesion`.

The handler decides which of these run and when; the loop executes them, but the model does not choose whether they are invoked and cannot bypass them.

### 2. Model-selected tools (supplemental evidence)

During rubric judgment the model may choose to call any registered tool from a fixed allowlist to gather additional evidence:

- **Baseline Scan (`repo_baseline_scan`)**: `semantics_analyze`, `codesignal_report` (re-invocation on specific files)
- **PR History Scan (`pr_history_scan`)**: the baseline tools plus `github_pr_files`

Unknown tools and over-budget loops are typed errors. Model text never becomes an arbitrary action. Model cooperation is never load-bearing for deterministic report content — only for the quality of agent judgments.

### 3. Deterministically owned by the handler / API layer

The following are never delegated to the model:

- Authentication and `Principal` resolution.
- Job claim, heartbeat, lifecycle, and terminal status transitions.
- Attempt-scoped persistence and idempotency.
- Which rubrics are registered for a job.
- Open/merged PR filter policy.
- Self-serve author check at submit.
- Smoke fixture path resolution.
- Size budgets and max-iteration budgets.
- **Judgment packing and prioritization** (how many rubric model calls, which findings, pack boundaries)—never model-selected. See amendment below and [local-LLM judgment spec](../../.github/specs/coach-api-platform-local-llm-judgment.spec.md).

Budgets for v1 (loop defaults; judgment phase may apply a separate wall—see amendment):

- `max_tool_calls`: 50
- `max_model_calls`: 20
- `max_wall_time`: 5 minutes (loop default; **insufficient as a single shared wall** for analyze+uncapped local-LLM judgment—amendment)

## Consequences

- **Positive**: The model cannot mutate policy, bypass authz, or escape the bounded loop.
- **Positive**: Tool calls are typed, schema-validated, and auditable.
- **Positive**: New rubrics and tools can be added without changing the API contract or worker lifecycle.
- **Negative**: The loop must be generic enough to support both model-selected and handler-driven tool registration patterns.
- **Negative**: Debugging requires reading both the tool registry and the handler's registration logic.

## Alternatives considered

| Alternative | Why rejected |
| --- | --- |
| Fully model-driven orchestration | Unsafe: model could choose rubrics, bypass authz, or loop forever. |
| Fully fixed handler code with no model tool choice | Removes the evidence-gathering flexibility that justifies the agent loop. |
| Model chooses which rubrics to run | Would let the model skip required judgments; rejected. |
| Direct package calls from handler instead of registry | Violates the architecture doc's tool-broker boundary and makes budget enforcement impossible. |

## Validation

- Acceptance tests drive the loop with a scripted stub gateway and assert tool-call sequences.
- Acceptance tests prove unknown tools and over-budget loops end with typed errors.
- Task 7 and Task 8 acceptance tests assert the analysis path executes via `internal/agentloop`, not via direct package calls.
- Acceptance tests prove a job completes with a deterministic-only report when the gateway is unavailable for the entire judgment phase.

## Amendment: Local-LLM judgment packing and budgets (2026-07-25)

| Field | Value |
| --- | --- |
| Status | Accepted (amends this ADR; does not replace the three-layer split) |
| Date | 2026-07-25 |
| Spec | [coach-api-platform-local-llm-judgment.spec.md](../../.github/specs/coach-api-platform-local-llm-judgment.spec.md) |

### Context

Pilot measurement on a small TypeScript repo (~42 `hidden_input_mutation` signals) with local `qwen3.5:4b-class` OpenAI-compatible inference showed that **handler-driven one-gateway-call-per-finding** exhausts a 5-minute shared loop wall before producing `source=agent` rows. The failure mode is call amplification and generation latency on small models—not monorepo size. Uncapped packing (max 4 or max 2 findings per pack) reduces call count and can improve JSON validity but still exceeds five minutes when judging all signals; **priority caps** are required for laptop viability.

### Decision (additive)

1. **Handler-owned packing**: `hidden_mutation_contextualization` may be invoked on **packs** of findings (token-aware packing with file-affinity preference). Pack formation, caps, and ordering are deterministic handler policy—not model-selected tools.
2. **Span-local evidence** by default (line windows), not full-file bodies on every call.
3. **Judgment-phase wall budget** is separated from deterministic analyze time (reset or distinct budget). Operator-configurable; local profile default **≥10 minutes** for the judgment phase unless a lower explicit config is set.
4. **Partial agent persistence**: if judgment budget/caps exhaust mid-phase, **keep** already-produced `source=agent` findings and record diagnostics; do not discard them on `ErrBudgetExceeded`.
5. **Quality cap**: `max_hidden_mutation_judgments` (local default **16**) with cross-file round-robin prioritization so small models spend budget on spread insight, not only the hottest file.
6. **Pack size default 4** (not 2): empirical full-set runs were slower at max 2 due to round-trips; operators may lower pack size if batch validity suffers.
7. Loop `max_wall_time` 5 minutes remains a safety default for the broker but must not be the sole control that zeros agent output after a successful deterministic pass.

### Consequences (additive)

- **Positive**: Local Qwen/Gemma-class Path B can complete with meaningful agent rows under explicit caps.
- **Positive**: Cloud/SGLang can raise caps without changing the orchestration split.
- **Negative**: Reports may include a judged subset of deterministic hidden-mutation signals; diagnostics must state selected vs omitted counts so pilots do not assume exhaustive agent coverage.
- **Negative**: Batch output schema and pack planner add surface area beyond singular rubric JSON.

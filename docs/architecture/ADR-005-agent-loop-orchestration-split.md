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

### 1. Model-selected tools

The model may choose to call any registered tool from a fixed allowlist. Tools are registered per job kind:

- **Baseline Scan (`repo_baseline_scan`)**: `semantics_analyze`, `codesignal_report`
- **PR History Scan (`pr_history_scan`)**: the baseline tools plus `github_list_prs`, `github_pr_files`

Unknown tools and over-budget loops are typed errors. Model text never becomes an arbitrary action.

### 2. Handler-driven tools

The job handler registers job-specific tools at loop start, primarily the rubric-judgment tools:

- `hidden_mutation_contextualization`
- `change_cohesion`

The handler decides which rubrics run and when; the loop executes them, but the model does not choose whether they are invoked or bypass them.

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

Budgets for v1:

- `max_tool_calls`: 50
- `max_model_calls`: 20
- `max_wall_time`: 5 minutes

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

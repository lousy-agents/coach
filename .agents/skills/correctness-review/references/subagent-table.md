# Subagent Selection for Correctness Review

Use the repository's custom subagents only when they add an independent review
surface. In this repo they are defined per harness under `.claude/agents/*.md`
and `.codex/agents/*.toml`.

| Agent | Use it when | Ask it to decide |
| --- | --- | --- |
| `spec-review-agent` | The PR implements a linked issue or canonical spec. | Whether the PR completes the specified task, preserves testable acceptance criteria, and leaves no required contract undefined for a dependent issue. |
| `system-design-expert` | The PR changes a reusable boundary, security model, infrastructure, async behavior, or platform foundation. | Whether the diff is consistent with the system overview, relevant ADRs, and future consumers; distinguish defects from valid deferrals. |
| `product-sme` | The PR changes product scope, claims customer value, establishes a foundation, or experts disagree about what belongs now. | Whether scope preserves intended customer value without scaffolding theater or premature platform work. |

Do not use `epic-reviewer` for an implementation PR: it is a spec-editing
convergence workflow. Reconcile every subagent concern with the current diff,
tests, and canonical documents before reporting it as a finding.

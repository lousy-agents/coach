---
name: correctness-review
description: Perform an evidence-backed GitHub pull-request correctness review against its linked issue's acceptance criteria, repository architecture, and downstream specs. Use when asked to deeply review a PR for completeness, explore its parent/sibling/child issue scope, validate a change against the larger initiative, or post coding-agent-ready inline review feedback. Do not use to implement fixes or to review a standalone specification before coding.
argument-hint: "PR number or URL to review (e.g., #123); optionally note review mode (explore/plan first, or full review) and whether to post to GitHub"
allowed-tools: Read, Grep, Glob, Bash, Agent
---

# Correctness Review

## When to Use

Use this skill for a PR that must be evaluated as a prerequisite in a larger
initiative, especially when the user asks whether it is complete, meets a
linked issue's acceptance criteria, or preserves architecture direction.

Do not use it for ordinary style-only review, implementing fixes, resolving
existing review comments, or spec-only review with no implementation diff.

## Procedure

### 1. Establish the review contract

1. Read `AGENTS.md` and inspect the current worktree without altering the
   user's existing changes.
2. Resolve the PR, its current head SHA, base SHA, and linked issue using the
   configured GitHub integration or `gh` when available. If no linked issue
   exists, use the PR description as a provisional contract and report the
   missing delivery contract as review context. If the repository, PR, or
   intended review depth is ambiguous, ask one question.
3. Create an isolated worktree for the PR head unless the user says not to.
4. Determine the review mode:
   - If the user asks to explore or plan before review, map the delivery
     surface, report the plan, and stop for approval before making findings.
   - Otherwise continue through the full review.

### 2. Map the entire delivery surface

1. Inspect the PR description, commits, changed files, patch, checks, review
   submissions, and unresolved or outdated threads.
2. Fetch the linked issue, its parent epic, sibling issues, child issues, and
   referenced PRs. Follow links that define requirements, dependencies, or
   later consumers.
3. Read the canonical local spec for the issue or epic, then only the
   downstream specs that consume the changed seam.
4. Read `docs/architecture/system-overview.md`, applicable ADRs, and the PRD
   when the PR establishes a reusable platform boundary.
5. Report a concise relationship map, changed-surface summary, worktree path,
   and review plan before making findings when the user requested the staged
   mode.

### 3. Trace acceptance criteria to evidence

For every criterion in the linked issue, identify one of:

- implementation plus a meaningful public-boundary or acceptance test;
- an explicit verification result;
- deferred work with a named later owner; or
- a gap.

Do not accept a PR description's claim as evidence. Verify the current PR
head, not an earlier reviewed commit. Treat an item as correctly deferred only
when the canonical issue/spec assigns it to a later task and this PR preserves
the required seam.

### 4. Consult custom reviewers selectively

Use the repository's custom subagents only when they add an independent review
surface. See [./references/subagent-table.md](./references/subagent-table.md) for
selection guidance; in this repo they are defined per harness under
`.claude/agents/*.md` and `.codex/agents/*.toml`.

Before delegating, capture the PR head SHA and put it in every reviewer prompt.
Require each reviewer to verify that SHA before inspecting code. Re-fetch the PR
after their reports return; if the head changed, discard or refresh every stale
report before reconciling findings.

Reconcile every subagent concern with the current diff, tests, and canonical
documents before reporting it as a finding.

### 5. Review and verify

1. Review behavior, failure paths, security boundaries, concurrency, task
   wiring, test isolation, and documentation claims in proportion to the
   changed surface.
2. Reproduce suspected defects with the narrowest safe check. Run focused
   checks before broader validation. Follow project instructions after each
   failure; do not claim an unrun check passed.
3. Re-check the PR head and review-thread state after any long-running check
   or before posting feedback.
4. Keep the isolated worktree clean. Do not implement fixes unless the user
   explicitly asks.

### 6. Report or post findings

Report only actionable findings that either fail the linked issue's contract,
violate a binding architecture/product decision, or incorrectly defer work
required now. For each finding include:

- severity and concise title;
- exact evidence;
- the violated acceptance criterion or architectural contract;
- downstream impact; and
- a bounded, implementation-ready direction plus acceptance evidence.

Clearly separate required fixes, valid later-task deferrals, and optional
hardening. State commands run and their outcomes.

Do not post to GitHub unless the user explicitly asks. When authorized:

1. Submit one review anchored to the current head SHA.
2. Prefer inline comments on changed lines when they are meaningful anchors.
3. Write for the receiving coding agent first: required behavior, failure
   mode, and the test or observable evidence that proves the fix.
4. If GitHub prevents requesting changes because the PR author is the current
   user, submit the same inline feedback as a comment review.

## Completion Checklist

- The issue acceptance criteria have an evidence-backed completion assessment.
- Relevant issue relationships, specs, architecture, and ADRs were explored.
- Each consulted subagent added a distinct perspective and its output was
  reconciled with repository evidence.
- Findings distinguish present defects from explicit later-task work.
- Validation results and GitHub posting status are reported accurately.

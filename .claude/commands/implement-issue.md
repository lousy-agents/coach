---
description: Implement a GitHub issue end to end — plan, delegate to implementer/reviewer subagents with the review gate blocking the critical path, and open a PR.
argument-hint: [issue-number]
model: inherit
---

Implement GitHub issue #$1. You are the orchestrator: you plan and delegate, but
you do not write feature code yourself — implementation and review are delegated
to subagents.

1. Read the issue with `gh issue view $1` (and any linked spec). Extract the
   explicit acceptance criteria. If any are ambiguous, state your interpretation
   and continue — do not stall.

2. Explore the affected code enough to build a task list. For each task record:
   the files it touches, its acceptance criteria, and its dependencies. Tasks that
   share no files and have no ordering dependency are parallelizable; everything
   else is sequential. Derive this from what you find, not from assumption. Track
   the task list with TodoWrite.

3. Do ONE self-check of the task list for three failure modes:
   (a) false parallelism — "parallel" tasks that actually touch the same file or
       consume each other's output;
   (b) an acceptance criterion no task covers;
   (c) an implementation task with no paired review gate.
   Fix what you find, then stop reviewing the plan and start work.

4. Execute. A task is COMPLETE only when its reviewer returns PASS; a task may not
   START until every task it depends on is COMPLETE. Independent branches run their
   full implement→review cycles concurrently — only the critical path is
   serialized.

   For each task:
   - Delegate implementation to the `task-implementer` subagent, scoped to that one
     task. It shares no context with you, so put everything in its prompt: the
     task's acceptance criteria, the in-scope file paths, and the conventions +
     lint/test commands from CLAUDE.md and package.json. If a `task-implementer-quick`
     variant exists, prefer it for mechanical, single-file edits.

     **Do not weaken AGENTS.md in implementer prompts.** Pass conventions from
     AGENTS.md as written. Never offer "stdlib table tests are fine" (or similar)
     as an alternative to Ginkgo acceptance tests for features/bug fixes. Do not
     invent weaker acceptance-test, HTTP-timeout, fail-closed, or comment rules
     than AGENTS.md states.
   - When it returns, delegate the task's diff to the `task-reviewer` subagent. Pass
     it the same acceptance criteria, scope, and conventions, plus any recurring bug
     patterns to watch for, plus the implementer's `## Implementer Report` block
     forwarded verbatim — the reviewer's own instructions require it to check the
     report's red-then-green test evidence, so never summarize or drop it. Do NOT
     start any dependent task until the reviewer returns PASS. On FINDINGS, hand the
     reviewer's `## Reviewer Findings` block to a fresh `task-implementer` verbatim
     — that implementer shares no history with the one that ran before it, so
     anything you paraphrase or drop while relaying is gone for good — and
     re-review. Repeat until PASS.

5. When every task's reviewer has returned PASS and the full lint/test suite is
   green, open the PR with `gh pr create`. In the description, map each acceptance
   criterion to where it is satisfied, and note which tasks ran in parallel.

If issue #$1 is trivial enough that decomposition adds no value, say so and
implement it directly through a single implementer→reviewer cycle rather than
forcing a task graph.

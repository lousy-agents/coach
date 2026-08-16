---
description: Implement a GitHub issue end to end — plan via the implement-issue-plan workflow, execute the implement/review loop from this session, then review the integrated diff and open a PR.
argument-hint: [issue-number]
model: inherit
---

Implement GitHub issue #$1. You are the orchestrator: you delegate planning,
implementation, and review, and you do not write feature code yourself.

**Why the split matters.** The review and PR gates
(`.claude/hooks/verify-review-verdict.sh`, `.claude/hooks/gate-pr-creation.sh`)
fire on agents spawned by *this* session and on *your* tool calls. Agents spawned
inside a workflow do not reach them. So planning runs as a workflow, but every
implement/review cycle and the PR creation itself must be your own calls — a run
that delegates those into a workflow looks identical and enforces nothing.

1. **Plan.** Call the `implement-issue-plan` workflow with `args: {issue: "$1"}`.
   It returns `{issue, plan, defects}`, where `plan` holds the acceptance
   criteria (stable IDs), the task DAG, and a verbatim `conventions` string.

   Read the returned `defects` before proceeding. They are the self-check's
   findings on its own plan — already repaired, but they tell you where the
   decomposition was fragile, which is where to be skeptical of a PASS.

   If the plan has one task and the issue is genuinely trivial, say so and run a
   single implement→review cycle rather than performing a task graph.

2. **Execute the DAG.** Track it with TodoWrite. A task is COMPLETE only when its
   reviewer returns PASS; a task may not START until every task in its `dependsOn`
   is COMPLETE. Independent tasks run their full implement→review cycles
   concurrently — only the critical path is serialized.

   For each task, use the **Agent tool** from this session (never a workflow):

   - Delegate to the `task-implementer` subagent, scoped to that one task. It
     shares no context with you, so its prompt must carry everything: the task's
     acceptance criteria, its `files` scope, its `acceptanceTest`, and the
     `conventions` string **verbatim**.

     **Do not weaken AGENTS.md in implementer prompts.** Never offer "stdlib
     table tests are fine" or any similar alternative to Ginkgo acceptance tests
     for features and bug fixes. Do not invent weaker acceptance-test,
     HTTP-timeout, fail-closed, or comment rules than AGENTS.md states. Pass the
     `conventions` string through as given — softening it is the failure mode
     these gates exist to catch.

   - When it returns, delegate the task's diff to the `task-reviewer` subagent
     with the same criteria, scope, and conventions, plus the implementer's
     `## Implementer Report` block **forwarded verbatim** — the reviewer is
     required to check its red-then-green evidence, so never summarize or drop
     it.

   - On FINDINGS, hand the reviewer's `## Reviewer Findings` block to a *fresh*
     `task-implementer` verbatim. That implementer shares no history with the one
     before it, so anything you paraphrase while relaying is gone for good. Then
     re-review. Repeat until PASS.

   Do not start a dependent task until its dependencies' reviewers return PASS.

3. **Review the integrated diff.** Once every task has PASSed, delegate to the
   `workflow-integration-reviewer` subagent — again via the Agent tool. Per-task
   review is scoped to one task's diff and cannot see what only appears in
   aggregate: two tasks that each met their criteria but together do not,
   integration glue no task owned, or a later task that invalidated an earlier
   one's evidence.

   Give it the full acceptance criteria with IDs, the per-task scopes, and each
   task's verdict. On FINDINGS, route each one back through a
   `task-implementer` → `task-reviewer` cycle, then re-run the integration
   review. Repeat until PASS.

4. **Validate.** Run `mise run ci-all` yourself and confirm it passes. Do not
   leave this to the gate: the gate's job is to refuse a red PR, not to be how
   you find out the suite is red. Discovering it there means the whole run has
   already been spent. It also warms the gate's own run — ~40s against ~390s
   cold, well inside its 900s timeout either way.

5. **Open the PR yourself**, with your own tool call, from this session. Commit
   and push first: the gate denies a dirty working tree, because the suite
   validates the working tree while a pull request publishes committed history.

   Fill every section of `.github/PULL_REQUEST_TEMPLATE.md` — no placeholders.
   Map each acceptance criterion to where it is satisfied, paste the
   red-then-green acceptance proof and the validation commands you actually ran,
   and record implementer/reviewer cycle counts and which tasks ran in parallel.

   Use a commit type that describes who the change is for. `feat`/`fix` are for
   behavior a `coach` user can invoke; changes to agent tooling, hooks, CI, or
   subagent definitions are `chore`/`ci`/`build` and stay out of the generated
   release notes.

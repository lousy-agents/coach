---
description: Implement a GitHub issue end to end — plan via the implement-issue-plan workflow, execute the implement/review loop from this session, then review the integrated diff and open a PR.
argument-hint: [issue-number]
model: inherit
---

Implement GitHub issue #$1. You are the orchestrator: you delegate planning,
implementation, and review, and you do not write feature code yourself.

The point of this command is **continuous review**: every change is written by
one agent and adversarially reviewed by another before it counts, and the
integrated result is reviewed again before it ships. Merge safety is not your
job — branch protection and the required `status` check on the base branch are
what make an unattended run safe. Your job is the quality of the loop.

**Why the split matters.** Planning runs as a workflow because it is read-only,
parallelizable research. The implement/review loop stays in this session
because the two review-fidelity hooks (`verify-review-verdict.sh`,
`verify-context-relay.sh`) fire on agents spawned by *this* session and on
*your* tool calls — agents spawned inside a workflow never reach them.

1. **Plan.** Planning produces one artifact: the issue's acceptance criteria with
   stable IDs (AC-1, AC-2, …), a `conventions` string quoted verbatim from
   AGENTS.md, and a task DAG in which each task records

   - `files` — every file the task may touch
   - `criteriaIds` — the acceptance criteria it satisfies, by ID
   - `dependsOn` — the task IDs that must be COMPLETE before it may start
   - `acceptanceTest` — the observable behavior whose absence the implementer
     must demonstrate as a failing test first

   In Claude Code, delegate this: call the `implement-issue-plan` workflow with
   `args: {issue: "$1"}`. It fans the research out in parallel, self-checks the
   result, and returns `{issue, plan, defects, repairApplied}`.

   **Without a Workflow tool** — OpenCode, or any other harness this file is
   mirrored into — do the same work inline instead: read the issue with
   `gh issue view $1` and any spec it links, explore the affected code, quote the
   conventions from AGENTS.md, and build the DAG above yourself. Nothing below
   depends on *how* the plan was produced, only on its shape.

   **With a Workflow tool that does not know this workflow**, pass
   `scriptPath: '.claude/workflows/implement-issue-plan.js'` instead of `name`,
   which runs the committed script directly.

   Self-check the plan once, however it was produced, for three failure modes:
   (a) false parallelism — tasks marked independent that share a file or consume
   each other's output; (b) an acceptance criterion no task covers; (c) a
   dependency cycle, or a `dependsOn` naming a task that does not exist — both
   deadlock step 2 rather than failing it. Fix what you find, then start.

   The workflow reports its own findings as `defects` and repairs them; read
   them before proceeding. They mark where the decomposition was fragile, which
   is where to be skeptical of a PASS later.

   If the issue is genuinely trivial, say so and run a single implement→review
   cycle rather than performing a task graph.

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
     the review loop exists to catch.

     The implementer does not commit, push, or open PRs — you own git. That is
     a rule its prompt states, not a mechanism; repeat it in your delegation.

   - When it returns, delegate the task's diff to the `task-reviewer` subagent
     with the same criteria, scope, and conventions, plus the implementer's
     `## Implementer Report` block **forwarded verbatim** — the reviewer is
     required to check its red-then-green evidence, so never summarize or drop
     it.

   - On FINDINGS, hand the reviewer's `## Reviewer Findings` block to a *fresh*
     `task-implementer` verbatim. That implementer shares no history with the one
     before it, so anything you paraphrase while relaying is gone for good.
     (`verify-context-relay.sh` denies a rework delegation that lacks the
     literal `## Reviewer Findings` heading.) Then re-review.

   Do not start a dependent task until its dependencies' reviewers return PASS.

   **The loop is bounded.** Run at most **3** implement→review cycles per task.
   Stop earlier when a cycle made no progress: the reviewer returned
   substantially the *same findings* as the previous cycle **and** the task's
   diff is unchanged since then. Both conditions matter — repeated findings
   against a moved diff means the implementer is chipping away at a real
   problem, and an unchanged diff with new findings means the reviewer is still
   discovering things. Only together do they mean the cycle is spinning.

   When you stop, name the reason from this set and **stop the run — do not
   open a PR**:

   - `repeated-finding` — the bound or the no-progress rule fired
   - `agent-failure` — an implementer or reviewer returned nothing usable twice
   - `ambiguous-product-decision` — the task needs a call the issue does not
     make and you are not authorized to invent
   - `scope-change` — the work required is materially outside the plan
   - `environment-failure` — the toolchain or a required command is unusable
   - `merge-conflict` — the branch cannot be reconciled with its base

   Report the reason, the task it stopped on, and what was completed. A stop is
   a legitimate outcome; a silent loop is not.

3. **Review the integrated diff.** Once every task has PASSed, delegate to the
   `workflow-integration-reviewer` subagent — again via the Agent tool. Per-task
   review is scoped to one task's diff and cannot see what only appears in
   aggregate: two tasks that each met their criteria but together do not,
   integration glue no task owned, or a later task that invalidated an earlier
   one's evidence.

   Give it the full acceptance criteria with IDs, the per-task scopes, and each
   task's verdict. On FINDINGS, hand its **entire `## Reviewer Findings` block
   verbatim** to a fresh `task-implementer`, exactly as in step 2 — do not split
   it into one delegation per finding. Then re-review, and re-run the
   integration review.

   **This loop is bounded exactly as step 2 is:** at most **3** integration
   rounds, the same no-progress rule, and the same named stop reasons. It needs
   its own bound because step 4 routes a red suite back through here — an
   unbounded loop at this point would swallow every repair attempt without the
   per-task cap ever applying.

4. **Validate — cheaply here, exhaustively in CI.** Run `mise run ci-fast`,
   the same command the per-task cycles use. Do **not** run the full suite
   yourself. GitHub Actions runs those checks as required parallel jobs — a
   strict superset, including `platform-smoke`, which no local task covers — in
   less wall clock than a serial local run and on compute that is not this
   session's. Re-running it here would spend ~910s of the scarcer budget to
   prove less than CI proves minutes later.

   **A red required check is repairable, not terminal.** After the PR is open,
   watch its checks. Route a failure back through the step-2 loop rather than
   abandoning it: paste the failing **job output** under a literal
   `## Reviewer Findings` heading, hand that to a fresh `task-implementer`
   scoped to the offending files, re-review, then push and let the checks
   re-run. Treat it as a step-3 finding for invalidation purposes — the
   integration review must run again afterwards.

   This path exists because CI is the *first* place `wasm-build`, the
   sidecar-built `pkg/projectmodel` suite, cross-file `gofmt`/`tidy-check`, and
   `platform-smoke` ever meet the integrated tree. No per-task cycle exercises
   them, so this is where a break is most likely — and it arrives *after* the
   PR exists rather than before. That is the trade: a red PR is a normal
   working state you drive to green, not a failed run.

   **Attribute the failure before delegating.** Map the failing job's paths
   to the tasks' declared `files` scopes. If it lands in exactly one task's
   scope, that task owns it. If it lands in several or none — a `wasm-build`
   break from two tasks interacting, a cross-file `gofmt`, an untidy `go.mod` —
   it belongs to no single implementer: open an **integration-repair task**
   scoped to the offending files, with the integration reviewer as its gate.
   Guessing an owner sends a fresh implementer to work outside the scope it was
   given.

   **`platform-smoke` is always unattributable.** It runs Docker and live
   services against the whole stack, so its failures are essentially never
   traceable to one task's declared files. Route it straight to an
   integration-repair task without attempting attribution.

   **Repair pushes are checked by CI, not locally.** The required checks re-run
   on every push, so a bad repair shows up there rather than being refused up
   front. Commit everything before pushing: the pull request body's evidence
   describes the tree you pushed, and a partial commit quietly makes that
   description false.

   **If the session dies mid-repair, that is `environment-failure`** — a typed
   stop, not a completed run. CCR containers are reclaimed on inactivity and
   repair is the longest-lived phase, so this will happen. An interrupted repair
   must never be reported as a finished one.

   Repair carries **its own cap of at most 3 attempts**, separate from the
   per-task counter. Sharing that counter would make a task already at its cap
   unrepairable, so a late validation failure would be terminal after all —
   which is the outcome this path exists to remove. Exhausting the repair cap
   stops the run with `repeated-finding`.

5. **Open the PR yourself**, with your own tool call, from this session. Commit
   and push first, and open the PR from a clean working tree: the PR body's
   evidence describes the tree you pushed, and uncommitted changes make that
   description false.

   Opening the PR is not the end of the run — step 4's repair loop continues
   against its required checks until they are green.

   **If you stop after the PR is open, leave it open and red.** Post the typed
   stop reason as a comment on the PR itself, where the next reader will look,
   and say what was completed and what was not. Never close it to tidy up, and
   **never merge it** to make the run look finished — branch protection should
   refuse that anyway, and a run that needs the protection to save it has
   already gone wrong.

   Fill every section of `.github/PULL_REQUEST_TEMPLATE.md` — no placeholders.
   Map each acceptance criterion to where it is satisfied, paste the
   red-then-green acceptance proof and the validation commands you actually ran,
   and record implementer/reviewer cycle counts and which tasks ran in parallel.

   Use a commit type that describes who the change is for. `feat`/`fix` are for
   behavior a `coach` user can invoke; changes to agent tooling, hooks, CI, or
   subagent definitions are `chore`/`ci`/`build` and stay out of the generated
   release notes.

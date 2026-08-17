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

0. **Prove the gates are live before you rely on them.** Everything below is
   written as though the hooks will catch you. A session where they never
   registered looks identical from the inside — the agents answer, the reviews
   return verdicts, the PR opens — so confirm registration first, by provoking a
   denial. Inspecting the files proves nothing: they are on disk either way.

   Call the Agent tool with `subagent_type: task-implementer` and a prompt that
   mentions reviewer findings but omits the literal `## Reviewer Findings`
   heading — `Address the reviewer findings below and fix them.` is enough.
   `verify-context-relay.sh` denies exactly that shape, before any agent spawns,
   so the call costs nothing when it works. The probe is not Claude-specific:
   `.opencode/plugin/implement-issue-gates.ts` denies the same shape with the
   same reason string, so a harness this file is mirrored into either denies or
   has no gate to prove.

   Being denied is the pass. Two other outcomes both mean stop with
   `environment-failure`, and neither is a warning to note and move past:

   - The call is **not** denied and an implementer actually runs — the hooks are
     not registered for this session.
   - The call errors because `task-implementer` is an unknown agent type —
     agents register from `.claude/` exactly as hooks do, so this is the same
     failure one layer earlier, and it is the likelier symptom of the two.

   In both cases stop the run here: do not proceed to step 1, and do not
   substitute a generic agent to get moving. An ungated run is worth less than
   no run — it produces the same artifacts with none of the guarantees, and
   nothing downstream can tell the difference.

   The usual cause is not in this repository. Claude Code binds `.claude/` —
   hooks, agents, and workflows alike — to the session's project directory at
   session start. A repository cloned into the session afterwards is never
   registered, and attaching it mid-session reloads CLAUDE.md and skills but not
   hooks, agents, or workflows. The fix is to make the repository the session's
   project directory when the session is created, then start again.

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

   **With a Workflow tool that does not know this workflow.** Registration and
   the name are separate failures: a session can hold the tool and still report
   `implement-issue-plan` unknown, because workflows register from `.claude/` at
   session start exactly as the hooks do. Pass
   `scriptPath: '.claude/workflows/implement-issue-plan.js'` instead of `name`,
   which runs the committed script directly. Note it — if the workflow was not
   registered, step 0 should already have stopped the run, and reaching here
   without that stop means the probe was skipped.

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
     these gates exist to catch.

   - When it returns, delegate the task's diff to the `task-reviewer` subagent
     with the same criteria, scope, and conventions, plus the implementer's
     `## Implementer Report` block **forwarded verbatim** — the reviewer is
     required to check its red-then-green evidence, so never summarize or drop
     it.

   - On FINDINGS, hand the reviewer's `## Reviewer Findings` block to a *fresh*
     `task-implementer` verbatim. That implementer shares no history with the one
     before it, so anything you paraphrase while relaying is gone for good. Then
     re-review.

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
   it into one delegation per finding. `verify-context-relay.sh` denies any
   rework delegation that lacks that literal heading, so a per-finding split is
   refused at the Agent tool. Then re-review, and re-run the integration review.

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

   The PR-creation gate runs only `mise run ci-gate` (about a second) plus a
   clean-worktree check, which is the one thing CI structurally cannot do: CI
   validates the *pushed commit* and cannot see that your working tree differs
   from it.

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
   them, so this is where a break is most likely — and it now arrives *after*
   the PR exists rather than before. That is the trade: a red PR is a normal
   working state you drive to green, not a failed run.

   **Attribute the failure before delegating.** Map the failing job's paths
   to the tasks' declared `files` scopes. If it lands in exactly one task's
   scope, that task owns it. If it lands in several or none — a `wasm-build`
   break from two tasks interacting, a cross-file `gofmt`, an untidy `go.mod` —
   it belongs to no single implementer: open an **integration-repair task**
   scoped to the offending files, with the integration reviewer as its gate.
   Guessing an owner sends a fresh implementer to work outside the scope it was
   given.

   Repair carries **its own cap of at most 3 attempts**, separate from the
   per-task counter. Sharing that counter would make a task already at its cap
   unrepairable, so a late validation failure would be terminal after all —
   which is the outcome this path exists to remove. Exhausting the repair cap
   stops the run with `repeated-finding`.

5. **Open the PR yourself**, with your own tool call, from this session. Commit
   and push first: the gate denies a dirty working tree, because it validates
   the working tree while a pull request publishes committed history.

   Opening the PR is not the end of the run — step 4's repair loop continues
   against its required checks until they are green.

   Fill every section of `.github/PULL_REQUEST_TEMPLATE.md` — no placeholders.
   Map each acceptance criterion to where it is satisfied, paste the
   red-then-green acceptance proof and the validation commands you actually ran,
   and record implementer/reviewer cycle counts and which tasks ran in parallel.

   Use a commit type that describes who the change is for. `feat`/`fix` are for
   behavior a `coach` user can invoke; changes to agent tooling, hooks, CI, or
   subagent definitions are `chore`/`ci`/`build` and stay out of the generated
   release notes.

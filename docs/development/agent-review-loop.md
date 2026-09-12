# The `/implement-issue` review loop: what it guarantees

Reference detail behind AGENTS.md's pointer to this page. AGENTS.md keeps the
operational facts an agent needs mid-run; this page carries the full account of
which guarantees are held by code and which are held by prose, so a reviewer can
judge how much a PR from this flow actually asserts.

The command's job is continuous review: every change is written by one agent and adversarially reviewed by another before it counts, and the integrated diff is reviewed again before a PR opens. It deliberately does not try to box agents in with control mechanisms — an earlier revision carried a git-write jail, a cycle-ceiling counter, a PR-creation gate, a hook trace, and a five-probe liveness ritual to prove them all live, and that apparatus spent more effort watching itself than reviewing code. It was removed. Merge safety belongs to branch protection plus the required `status` check, which run where no agent can reach them.

Deterministic, held by code:

- **Planning.** `.claude/workflows/implement-issue-plan.js` produces the task DAG. Everything decidable from the plan alone is JS rather than an auditor prompt — arg validation, null and empty-result guards on every agent, cycle and dangling-reference detection, duplicate task/criterion IDs, unscoped tasks, and criterion coverage — covered by `mise run workflow-test`, and mutation-tested. The two semantic lenses (false parallelism, coverage) run as agents, and run again against the repaired DAG so a repair cannot return clean unverified; whatever it failed to fix comes back as `residualDefects`.
- **Review-loop fidelity.** Two small hooks, both scoped to the loop's conversation rather than to what agents may do: `verify-review-verdict.sh` (a reviewer's reply shall begin `PASS` or `FINDINGS`, so the orchestrator always receives a parseable verdict) and `verify-context-relay.sh` (a rework delegation shall carry the literal `## Reviewer Findings` block, so findings cannot be paraphrased away). They fire on agents this session spawns — agents inside a workflow never reach them, which is why the implement/review loop stays in the main session.
- **Merge safety.** Branch protection and the required `status` aggregator on the base branch. This is the only gate that matters for an unattended run, and it runs on GitHub's side.

Prose, held by the orchestrator following instructions:

- The per-task cycle cap (3), the integration and repair caps, and the no-progress rule
- Dependency ordering — that a task waits for its `dependsOn`
- Which task receives a validation failure, and the invalidation rule after rework
- That the `conventions` string reaches implementers unweakened
- That implementers do not commit, push, or open PRs — the orchestrator owns git

Those are instructions in `.claude/commands/implement-issue.md`. The specs in `internal/agentworkflows/` assert that the instruction is present and says the right thing — they cannot assert that a run obeyed it. A run that ignores them produces a worse PR, not an unsafe merge: the required checks still gate the merge.

A PR opened by this flow asserts that every task reached reviewer `PASS`, that the integration reviewer passed the whole diff, and that the body records the per-task `ci-fast` output as its test evidence. It does not assert that the exhaustive suite passed locally — that no longer runs locally — nor `platform-smoke` or `test-acceptance-fast` results, nor that any prose-held rule above was obeyed. Read such a PR as a well-evidenced proposal, not a verified one. Branch protection is what makes it safe to open one unattended.

One environment caveat: Claude Code binds `.claude/` — hooks, agents, and workflows alike — to the session's project directory at session start. A repository cloned into a session whose project directory is elsewhere never registers any of them, and attaching it mid-session reloads CLAUDE.md and skills but not hooks, agents, or workflows. In that state the two review-fidelity hooks are silently absent and the run degrades to prose-only review discipline — still merge-safe, because branch protection does not care, but weaker. Create sessions for this repo with the repo as the project directory.

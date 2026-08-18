<!--
AGENT INSTRUCTIONS (read fully before opening the PR)

This template is the PR contract. Coding agents (opencode, Claude Code, Codex,
Copilot, etc.) fill it when running `gh pr create` / create_pull_request.

Rules:
1. Fill every section with concrete facts from THIS change. No placeholders.
2. One concern per PR. Unrelated changes → split or stop.
3. Link the delivery issue (`Closes #N` / `Fixes #N`). If none exists, say why
   and treat the Acceptance criteria table as the provisional contract.
4. Evidence > claims. Paste commands + outcomes. Do not assert tests passed
   without naming what you ran.
5. Follow AGENTS.md: acceptance-test-first, mise validation, package boundaries.
6. Present the full diff to a human and get explicit approval before commit/PR
   when a human partner is available. Check the Human review box only if true.

Close-without-review triggers:
- blank/placeholder sections
- multiple unrelated concerns
- no linked issue AND no provisional AC table
- "tests pass" with no command evidence
- feature/bugfix with no red-then-green acceptance evidence
- pkg/semantics ↔ pkg/githubingest import-boundary violation
-->

Closes <!-- #N -->

## Problem
<!-- What is broken or missing TODAY? Not "improve X". Cite issue/spec path. -->

## Change
<!-- 1-3 sentences: what the diff does. What, not why. -->

## Scope
- [ ] Single concern (no bundled drive-bys)
- Related PRs (open + closed search): <!-- #N, #N, or "none found" -->
- Out of scope / deferred: <!-- what this PR deliberately does not do -->

## Architecture invariants
<!-- Check only what applies. Uncheck + explain if an invariant is touched. -->
- [ ] `pkg/semantics` does not import `pkg/githubingest` / `go-github` / `ghinstallation`
- [ ] `pkg/githubingest` does not import `pkg/semantics`
- [ ] Public JSON / error sentinels unchanged, or change is intentional and tested
- [ ] Production HTTP clients keep a finite `Timeout` (no bare `http.DefaultClient`)
- [ ] Store/dependency errors on protected paths fail closed (503 + stable envelope)
- [ ] No new Go comments that restate code; comments only for non-local contracts

## Acceptance criteria → evidence
<!-- One row per criterion from the linked issue (or provisional contract).
     "Where satisfied" must point at code path and/or test name, not vibes. -->

| Criterion | Where satisfied |
| --- | --- |
| | |

## Acceptance-test-first
<!-- Required for features and bug fixes. Pure refactors/docs: mark N/A + why. -->
- Policy: failing acceptance test landed BEFORE production implementation
- Test file(s): <!-- e.g. pkg/foo/bar_acceptance_test.go -->
- Form: Ginkgo v2 `Describe`/`When`/`It` in `*_acceptance_test.go` (or N/A reason)
- Red command + outcome:
```
<!-- paste: command, FAIL output summary -->
```
- Green command + outcome (same test):
```
<!-- paste: command, PASS -->
```
- [ ] False-green check: test exercises the intended branch/failure mode (not a
      shared clock/fake making a different path look correct)

## Validation run
<!-- Paste real output summaries. Agents: run these before gh pr create.
     The exhaustive gate is CI: the required status check must pass to merge. -->

| Check | Command | Result |
| --- | --- | --- |
| Go CI slice | `mise run ci` | <!-- pass / fail + note --> |
| Acceptance fast | `mise run test-acceptance-fast` | <!-- pass / N/A --> |
| JS (if touched) | `mise run js-ci` | <!-- pass / N/A --> |
| WASM (if tags/engine touched) | `mise run wasm-build` | <!-- pass / N/A --> |
| Focused package | `go test -race ./path/... -run Name` | <!-- pass / N/A --> |

Extra verification (behavior, not just green suite):
<!-- e.g. golden Result before/after, parity.test.ts, coach codesignal --baseline,
     thin-proof, manual API smoke. "mise run ci passed" alone is insufficient
     for behavior claims. -->

## Alternatives considered
<!-- What else did you try or reject? "None" is allowed but is a review flag. -->

## Risk / rollback
<!-- Blast radius, feature flags, data/API compat, how to revert. -->

## Review process
<!-- How this was built/reviewed. implement-issue: note implementer/reviewer cycles. -->
- Implementer/reviewer cycles: <!-- N × implement→review until PASS, or "direct" -->
- Residual FINDINGS deferred: <!-- none | list with owner -->

## Human review
- [ ] A human reviewed the complete diff and approved opening this PR

<!--
STOP if Human review is unchecked and a human partner exists.
Agents opening PRs in unattended pipelines: leave unchecked and state
"unattended" in Review process — do not fake the checkbox.
-->

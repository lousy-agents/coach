# ADR-009: The Project Policy Is Read from Disk in Every Scan

| Field | Value |
| --- | --- |
| Status | Accepted |
| Date | 2026-11-22 |
| Deciders | Repository owner and maintainers ([#168](https://github.com/lousy-agents/coach/issues/168)) |
| Supersedes | In part, [ADR-008](ADR-008-explicit-policy-disk-and-revision.md): its decision 2 ("What does the policy say? The analyzed revision answers"), and its sentence "A policy found by discovery keeps its source: `.lousy-agents/coach.json` and the root `project.json` are judged at the revision, and a discovered policy deleted on disk is scanned from its committed copy". ADR-008's decisions 1 and 3, and its case and kind refusals for discovery, stand. Also the sentence "The selected file is read at the selected revision" in [`docs/conventions/lousy-agents-config.md`](../conventions/lousy-agents-config.md) |

## Context

ADR-008 split an explicit `--project-config` between two sources: the working tree says whether a usable file is at the path, and the analyzed revision supplies its content. It deferred reading the content from disk to a later slice of [#168](https://github.com/lousy-agents/coach/issues/168).

That split leaves the case #168 exists for unserved. Agents edit files, and a hook runs Coach on that work before anyone commits. A policy edit in that work is ignored, and a policy that is on disk and not committed is refused or reported as `project_config_uncommitted`. ESLint and Biome read their configuration from disk, and the owner chose that behavior for Coach: "I want to keep the same behavior as eslint or biome for the policy configuration."

The PRD already states the rule (section 7): "The project policy is read from disk, as ESLint and Biome read their configuration. The comparison with `HEAD` is Coach's own addition, not parity with those tools." Section 8.2 states that "the base side and the head side of a comparison use the same project policy". This ADR records the decision behind those lines.

## Decision

1. **Content comes from disk, in every scan.** The project policy's content is read from the working tree in a committed scan and in a `--worktree` scan, for `--baseline` and for `--base <ref>`, whatever revision the source is read at.
2. **One policy per scan.** Both sides of a comparison use the one policy read from disk. The base side of `--base <ref>` does not read the policy committed at the merge base.
3. **Discovery reads the disk too.** Without `--project-config`, the lookup order is unchanged and each step asks the working tree:
   - A schema-valid `.lousy-agents/coach.json` on disk is used.
   - `.lousy-agents/coach.json` is committed at `HEAD` and deleted on disk, and a schema-valid root `project.json` is on disk: `project.json` is used, with a WARNING that names the fallback.
   - `.lousy-agents/coach.json` is committed at `HEAD` and deleted on disk, and no root `project.json` is on disk: the scan is file-local, with a WARNING that names the missing file.
   - An invalid `.lousy-agents/coach.json` on disk: the scan exits `2` with `project_config_invalid`, and `project.json` is not read.
4. **Disclosure replaces reproducibility from the commit.** JSON `scope.policy` is present in every scan. It gives the path and digest of the policy used, and `matches_head`. `matches_head` is `true` when the policy selected from disk has the same path and digest as the policy selected at `HEAD`. The text output names the policy when one is selected. When the policy on disk differs from `HEAD`, the text says so as a WARNING, and the WARNING qualifies any clean result. Exit codes are unchanged.

ADR-008's decision 1 stands: an explicit path must be a usable regular file on disk, or the scan exits `2`. Its decision 3 stands: the analyzed revision decides which spelling a path names, and a policy committed in another case is refused as `name_case_differs`. Decision 3 assumed a committed policy. When the analyzed revision commits nothing at the path or at a case variant of it, the path as written is read from disk, and its spelling is checked against the directory listing on disk, as the worktree overlay already checks source paths; a mismatch is refused as `name_case_differs`. ADR-008's refusals of a discovered canonical path that the revision commits in another case, or as a directory or another entry a policy cannot be, also stand.

## Options considered

| Option | Effect | Verdict |
| --- | --- | --- |
| Content at the revision (ADR-008's decision 2) | Reproducible from the commit. A policy edit is invisible until committed, so the pre-commit hook judges agents' work against a policy that is not the one on disk | Rejected: it is the gap #168 closes |
| Disk for the head side, the revision for the base side | Each side keeps its own policy. A policy-only change shows up as new or resolved findings | Rejected: a finding would change only because the rule changed, not the code |
| `--policy-source committed`, an opt-in flag to read the policy from `HEAD` | Keeps the old behavior on request | Rejected by the owner, for ESLint and Biome parity |
| `--fail-on-policy-differs`, an opt-in flag to exit `2` when disk differs from `HEAD` | Turns the disclosure into a gate | Rejected by the owner, for ESLint and Biome parity |
| Disk for content, in every scan, with disclosure (chosen) | Matches ESLint and Biome. Reproducibility moves from the commit to the disclosure | Chosen |

ADR-008 rejected a "disk only" option because it "breaks `--base` and reproducibility". This ADR accepts that cost for the policy's content only: source is still read at the revision, and the disclosure says when the policy is not the one at `HEAD`.

## Consequences

- `project_config_uncommitted` is removed. A policy that is on disk and not committed is read, and the disclosure names it. An explicit `--project-config` that names such a file is read too, and no longer exits `2`.
- A finding that changes only because the policy changed is classified `existing`, because both sides use the same policy.
- A scan's result is reproducible from the commit only when `scope.policy.matches_head` is `true`. Otherwise the policy's path and digest identify what was used.
- A local edit to the policy changes a local run's result. CI reads its own checkout, so it enforces the policy committed in the change under test. A change that edits the policy is reviewed like an ESLint configuration change.
- A scan of an old revision with `--base` uses today's policy on disk, not the one committed at that revision.
- The project fingerprints of an unchanged policy edit are restored when the edit is committed, because the identity of a finding does not depend on the mode.
- Open implementation details, left to the code slices of #168: how `--check-project` maps its `policy_missing` cause `not_committed` (ADR-008) once an uncommitted policy is read, and whether the WARNING for the root `project.json` fallback is its own diagnostic or part of `project_config_deprecated`.
- Until the code that implements this lands with its acceptance tests, the shipped CLI reads the content at the revision, as ADR-008 describes, and the PRD's status table marks policy-from-disk as planned.

# ADR-009: The Project Policy Is Read from Disk in Every Scan

| Field | Value |
| --- | --- |
| Status | Accepted |
| Date | 2026-11-22 |
| Deciders | Repository owner and maintainers ([#168](https://github.com/lousy-agents/coach/issues/168)) |
| Supersedes | In part, [ADR-008](ADR-008-explicit-policy-disk-and-revision.md): its decision 2 ("What does the policy say? The analyzed revision answers"); its decision 3 ("Which spelling does the path name? The analyzed revision answers, and only it"), its refusals of a discovered policy committed in another case or as another kind of entry; and its sentence "A policy found by discovery keeps its source: `.lousy-agents/coach.json` and the root `project.json` are judged at the revision, and a discovered policy deleted on disk is scanned from its committed copy"; its consequence "A scan of a non-HEAD revision (`--base`) still reads the policy at that revision"; and its statement that the scan "still reads nothing from disk" and says `project_config_uncommitted`. ADR-008's decision 1 stands. Also the sentence "The selected file is read at the selected revision" in [`docs/conventions/lousy-agents-config.md`](../conventions/lousy-agents-config.md) |

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
   - An unusable entry at `.lousy-agents/coach.json` on disk (a directory, a symbolic link, a file that cannot be read): the scan exits `2` with `project_config_invalid`, naming the path and the reason. It does not fall back to `project.json` or to file-local analysis.
   - The kind of entry is judged on disk. A usable regular file on disk is read whatever `HEAD` commits at the path, such as a symbolic link or a directory, with `matches_head: false` and a WARNING.
4. **Disclosure replaces reproducibility from the commit.** JSON `scope.policy` is present in every scan. It gives the path and digest of the policy used, and `matches_head`. `matches_head` is `true` when the policy selected from disk has the same path and digest as the policy selected at `HEAD`. `scope.policy` also gives a `cause`, always present when `policy` is an object, one of `same_as_head`, `differs_from_head`, `not_committed`, `missing_on_disk` and `unusable_on_disk`. `matches_head` is `true` exactly when the cause is `same_as_head`. `missing_on_disk` and `unusable_on_disk` name the interim cases where the scan falls back to the committed copy, until the code slices that refuse those states replace the fallback. The text output names the policy when one is selected. When the policy on disk differs from `HEAD`, the text says so as a WARNING, and the WARNING qualifies any clean result. Exit codes are unchanged.

5. **The disk decides the spelling.** Because the content comes from disk, the disk decides which spelling the policy path names, in every mode, including `--base`. The spelling is compared with the directory listing on disk case-folded, on every file system, as the worktree overlay checks source paths, so a case-only difference is `name_case_differs` on macOS and Linux alike, and never `project_config_missing`. An exact-case entry wins over its case variants; with several variants and no exact one, the refusal lists them, up to 16, as in commit mode. Where the name passed with `--project-config`, or the canonical `.lousy-agents/coach.json`, differs only in case from the spelling on disk and no entry on disk is spelled exactly as the name, the scan exits `2` with `project_config_invalid` and `name_case_differs`, and the message says to rename the file on disk or to pass the spelling on disk, never to commit. A spelling that differs only from the one committed at `HEAD` is disclosed (`matches_head: false` and a WARNING), not refused, so an uncommitted `git mv` that fixes the case works before the commit. This supersedes ADR-008's decision 3.
6. **The committed copy is read only for disclosure.** When it cannot be read at `HEAD` within git's time or stderr bound, the scan uses the policy on disk, reports `matches_head: false`, and prints a WARNING that says the committed copy could not be read, not that the content differs.
7. **No policy on either side.** When neither the disk nor `HEAD` selects a policy, `scope.policy` is `null`. When only one side selects one, `matches_head` is `false` with a WARNING. The exact field values of the one-sided case are left to the code slice.

ADR-008's decision 1 stands: an explicit path must be a usable regular file on disk, or the scan exits `2`.

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
- The disk read now reads bytes that ADR-008 never read in a committed scan. It keeps the size budget, the non-blocking regular-file open and the nesting limits. The B8 hostile-input specs (rows F09 to F13 of `docs/development/evil-input-checklist-595.md`, and `cmd/coach/project_config_hostile_input_acceptance_test.go`), which assert that on-disk bytes are never read, are rewritten with the code slice that reads the policy from disk.
- Open implementation details, left to the code slices of #168: how `--check-project` maps its `policy_missing` cause `not_committed` (ADR-008) once an uncommitted policy is read, and whether the WARNING for the root `project.json` fallback is its own diagnostic or part of `project_config_deprecated`.
- Until the code that implements this lands with its acceptance tests, the shipped CLI reads the content at the revision, as ADR-008 describes, and the PRD's status table marks policy-from-disk as planned.

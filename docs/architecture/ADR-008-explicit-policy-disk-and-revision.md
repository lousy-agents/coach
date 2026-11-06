# ADR-008: An Explicit Policy Path Is Judged on Disk, and Its Content Is Read at the Revision

| Field | Value |
| --- | --- |
| Status | Accepted |
| Date | 2026-11-04 |
| Deciders | Repository maintainers |
| Supersedes | The sentence "The selected file is read at the selected revision" in [`docs/conventions/lousy-agents-config.md`](../conventions/lousy-agents-config.md), for an explicit flag only |

## Context

Every lousy-agents tool selects its configuration by one lookup: an explicit flag, then `.lousy-agents/<tool>.json`, then a deprecated root file. The convention read the selected file at the analyzed revision, so a scan was reproducible from the commit alone.

For `--project-config` that left two gaps. A path deleted from the working tree but committed at the analyzed revision was read from the commit and scanned as if nothing were wrong, so a user who had removed or replaced the file saw a report about a policy that was no longer on disk. And a path that was a symbolic link, a directory, a FIFO, or reached through a link could be committed as a different kind of entry than the one on disk, so the entry a reviewer saw and the entry the scan read were not the same thing.

## Decision

For an **explicit** `--project-config` only, two questions are answered from two sources:

1. *Is there a usable entry at the path?* The working tree answers. The path must be a regular file, reached without a symbolic link, that the process can open. Otherwise the scan exits `2`: `project_config_missing` when nothing is there, `project_config_invalid` when something unusable is.
2. *What does the policy say?* The analyzed revision answers, as before. The file is not read from disk in this slice. Reading the bytes from disk is the later slice of [#168](https://github.com/lousy-agents/coach/issues/168).

A policy found by discovery is unchanged. `.lousy-agents/coach.json` and the root `project.json` are judged at the revision, and a discovered policy deleted on disk is scanned from its committed copy.

When the working tree decides, `--check-project` marks the policy check as decided on disk and leaves that path out of `dirty_worktree`, because the report did read the working tree for it. The exception is a staged edit to a committed policy, or a staged entry for a policy that was never committed, that the working tree could not supply, whether the file is gone, a file stands in a parent's place, or an unusable entry took its place: nothing read that edit, so the path stays in `dirty_worktree` and no restore is offered over it. An unmerged entry for a committed policy is the same case. Only the policy's own path leaves the list: every other uncommitted path under the policy's roots still counts, and the roots of a policy the working tree refused are those of its committed copy at the analyzed revision. With no valid policy committed there are no roots, and only package and compiler metadata counts.

## Options considered

| Option | Effect | Verdict |
| --- | --- | --- |
| Revision only (the previous convention) | Reproducible from the commit. A file deleted or swapped on disk is invisible to the scan | Rejected: the user is told about a policy that is not the one in front of them |
| Disk only | A scan of a non-HEAD revision judges the working tree, and CI on a sparse checkout cannot run | Rejected: breaks `--base` and reproducibility |
| Refuse on any dirty policy | Strictest, but turns an uncommitted edit into a hard failure, which `--check-project` and the dirty-worktree disclosure already report without one | Rejected: the disclosure exists for this |
| Disk for existence and kind, revision for content (chosen) | Catches the file that is gone or unusable, keeps the content reproducible, changes nothing for discovery | Chosen |

## Consequences

- A scan of a non-HEAD revision (`--base`) still reads the policy at that revision, but fails if the path is not a usable file on disk now. A reproducible scan of an old revision needs a checkout where the explicit path exists.
- `--check-project` carries a `restore_policy` next action beside `author_policy`, so a policy committed and then deleted is restored rather than rewritten. Coach prints the restore command; it never runs it.
- Other lousy-agents tools adopt the same split for their own explicit flag, or state that they do not. Until one does, the convention document describes the split and the tools keep their own behavior.
- The probe lives in `projectconfig`, which ADR-007 places in the model layer. Moving it to a driven adapter is tracked in [coach#613](https://github.com/lousy-agents/coach/issues/613).
- `configExistsInWorktree`, used for the discovered path, still follows symbolic links without root containment. Aligning it is tracked in [coach#615](https://github.com/lousy-agents/coach/issues/615).

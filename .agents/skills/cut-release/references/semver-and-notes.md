# SemVer and release notes

## Bump table

| Signal in the Phase 2 research | Bump |
| --- | --- |
| New user-visible command, flag, signal ID, or platform path | MINOR |
| Bug, reliability, or security fix with no new capability | PATCH |
| Behavior break on 0.x | MINOR, and the notes say what broke and how to adapt |
| Only excluded types | See Phase 2 step 5 |
| Only `fix(deps)` bumps | PATCH, and only if the user wants a cut for them |

## Reader and posture

The reader already runs `coach codesignal`, or is about to try it. Write for that person, who cannot see git history.

State these facts in the prose of the opening frame and limits section:

- Coach is experimental and stays on 0.x.
- What ships is the local CLI — not a 1.0 product, a hosted API, or a merge gate.

## Required sections, in order

1. **Opening frame** — two to four sentences. First: `Feature release` or `Patch release` on the v0.x local-pilot line. Second: `Headline:` plus one customer-visible change. Then what did not change (for example, default behavior when the new flag is omitted).
2. **`## What’s new`** — one `###` per story, headline first; group related commits so there are usually one to three. Say what the user can do now, how to invoke it, and what the output means. Name the flags, JSON keys, and signal IDs they will see, and state each feature's limits beside it.
3. **`## Try it`** — copy-paste commands that pin `NEXT_TAG` and exercise the headline (use the new flag itself, not only a generic `--baseline`). For a dependency-only patch, run `coach --version` and the standard `--baseline` scan.
4. **`## Limits and trust notes`** — what the analysis does not prove, what a clean report does not mean, what stays out of scope.
5. **`## Binaries`** — the GoReleaser archives and the signed `checksums.txt`.
6. **`## Related`** — a table of issues, epics, and the previous release.

## Voice references

- Good: [v0.7.0](https://github.com/lousy-agents/coach/releases/tag/v0.7.0) is the primary voice reference. [v0.4.0](https://github.com/lousy-agents/coach/releases/tag/v0.4.0) teaches `--project-config` as a capability — what it reads, which signal it emits, what happens without it — names honest limits beside the feature, gives commands using the new flag, and links issues. [v0.3.0](https://github.com/lousy-agents/coach/releases/tag/v0.3.0) and [v0.6.0](https://github.com/lousy-agents/coach/releases/tag/v0.6.0) follow the same pattern.
- Bad: [v0.5.0](https://github.com/lousy-agents/coach/releases/tag/v0.5.0) and [v0.8.0](https://github.com/lousy-agents/coach/releases/tag/v0.8.0) as published are GoReleaser's commit log — a `## Changelog` heading and merge SHAs, from which a customer cannot tell the headline story. This is the state Phase 6 exists to replace.

## Quality gate

The draft passes when all of these hold; rewrite until they do:

- The first line starts with `Feature release` or `Patch release` and a `Headline:` sentence follows.
- Every heading is one of the six required sections or a `###` story under `## What’s new`.
- No line contains a commit SHA, `Merge pull request`, or a conventional-commit subject such as `feat(codesignal): …`.
- The headline names a user-visible change; for a dependency-only patch, it names the user-visible effect (for example, a parser fix) rather than the package bump.
- `## Try it` pins `NEXT_TAG` and runs the headline feature, or `coach --version` plus a `--baseline` scan for a dependency-only patch.
- `## Related` links issues or releases with no SHA column.

## Example shape

```markdown
Feature release on the v0.x local-pilot line. Headline: <one user-facing change>.
Default behavior is unchanged if you omit the new flag. Coach remains experimental
and stays on the 0.x line. This is a local CLI preview, not a 1.0 product, hosted
API, or merge gate.

## What’s new

### <user-facing name, usually a flag or signal ID>

<what the user can do now, how to invoke it, what the report means>
<what the feature does not do>

## Try it

    mise use -g github:lousy-agents/coach@NEXT_TAG
    coach codesignal --baseline <headline flags>

## Limits and trust notes

Advisory static analysis only. A clean report is not proof of absence.
<feature-specific limits>

As in PREV_TAG: local CLI preview, no hosted deployment, no web UI.

## Binaries

GoReleaser publishes archives for macOS (darwin_arm64, darwin_x86_64), Linux
(linux_x86_64), and Windows (windows_x86_64), plus checksums.txt and its cosign
signature bundle (checksums.txt.bundle).

## Related

| Area | Issue / release |
| --- | --- |
| <headline story> | #<issue> |
| Previous release | PREV_TAG |
```

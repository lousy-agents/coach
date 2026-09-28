---
name: cut-release
description: "Cut a lousy-agents/coach GitHub Release: read the last tag, propose the next 0.x SemVer, draft customer-facing release notes, push an annotated tag so the release workflow's GoReleaser job publishes the archives and a cosign-signed checksums.txt, then replace GoReleaser's commit changelog with the confirmed notes. Use when asked to 'cut a release', 'cut a coach release', 'ship coach', 'create a GitHub Release', 'write coach release notes', 'bump the coach version', 'tag v0.x', or 'publish the CLI'. Not for rewriting PR commits before merge (use curate-release), snapshot builds, or uploading binaries by hand."
argument-hint: "optional next version such as v0.9.0; omit to have one proposed"
effort: high
allowed-tools: Read, Grep, Glob, Bash(git remote get-url*), Bash(git status*), Bash(git fetch*), Bash(git tag --list*), Bash(git rev-parse*), Bash(git merge-base*), Bash(git log*), Bash(git diff*), Bash(git show*), Bash(git ls-remote*), Bash(gh auth status*), Bash(gh release view*), Bash(gh run list*), Bash(gh run view*), Bash(gh run watch*), Bash(gh pr view*)
---

# Cut Release

Cut one `lousy-agents/coach` release. Pushing an annotated `vX.Y.Z` tag starts the `release` job in `.github/workflows/release.yml`; GoReleaser creates the GitHub Release, attaches the archives, and fills the body with a commit changelog. This skill replaces that body with customer notes the user confirmed.

The run is human-in-the-loop at two points: the user confirms the version, notes, and any README change (Phase 3), and the user approves the tag push (Phase 5). Everything before Phase 4 is read-only apart from writing the notes file.

`PREV_TAG`, `NEXT_TAG`, `TARGET_SHA`, and `NOTES_FILE` are placeholders: write their values into every command, because shell variables do not survive between commands or turns.

## When to Use

- The user asks to cut, ship, tag, or publish a coach version.
- The user asks for a new GitHub Release of the `coach` CLI, or for release notes for one.
- The user asks to bump the version after user-facing work landed on `main`.

Use another path when:

- The user wants a PR's commits rewritten before merge — that is `curate-release`.
- The user wants a snapshot build — the `snapshot` job in `release.yml` already runs on pull requests.
- The origin is not `lousy-agents/coach` and the user did not name it as the target.

## Hard stops

The CI `release` job is the only publisher, because it signs `checksums.txt` with cosign and attests provenance and a laptop publish does neither. Stop and report instead of running any of these:

- `gh release create`, `gh release delete`, `gh release upload`
- `goreleaser release` in any form, or uploading `dist/` from the laptop
- `git tag` without `-a`, or moving, deleting, or force-pushing an existing tag
- Proposing `v1.0.0` without the user asking for it
- Rewriting commit history (that belongs to `curate-release`)

The only release-mutating command this skill runs is `gh release edit NEXT_TAG --title … --notes-file NOTES_FILE`.

## Procedure

### Phase 0 — Preflight

1. Confirm `git remote get-url origin` is `lousy-agents/coach` (or the fork the user named) and `gh auth status` succeeds.
2. Read `.github/workflows/release.yml` and `.goreleaser.yaml`. Confirm the workflow triggers on `v*.*.*` tags and runs `goreleaser release --clean`, and note the `changelog.filters.exclude` patterns for Phase 2.

If a check fails, stop and report it using [./references/preflight.md](./references/preflight.md).

### Phase 1 — Pin the range

1. `git fetch origin --tags --prune`
2. `git tag --list 'v*' --sort=-version:refname` — `PREV_TAG` is the first `vMAJOR.MINOR.PATCH` tag with no suffix (use a suffixed line such as `-rc.1` only if the user asked to cut from it).
3. `TARGET_SHA` is `git rev-parse origin/main`. `git merge-base --is-ancestor PREV_TAG TARGET_SHA` must succeed; if it does not, or no tag exists, stop and ask the user which tag to cut from.
4. `gh run list --workflow=ci.yml --commit TARGET_SHA --json databaseId,status,conclusion` must show `completed`/`success`. If it is in progress, run `gh run watch <id> --exit-status` in the background or with a 600000 ms timeout; if it failed or was cancelled, stop and report it using [./references/preflight.md](./references/preflight.md).

### Phase 2 — Research and classify

1. `git log PREV_TAG..TARGET_SHA --no-merges --format='%h %s'`, and `git log PREV_TAG..TARGET_SHA --merges --format='%s'` for PR numbers. `gh pr view <N> --json title,closingIssuesReferences` gives the issues each PR closed.
2. Set aside subjects matching the exclude patterns from Phase 0. The rest are candidate stories.
3. Read `git diff PREV_TAG...TARGET_SHA --stat` and the diff under `cmd/`, `internal/`, `pkg/`, `js/`, and `README.md`.
4. Propose `NEXT_TAG` as `v0.Y.Z` from the bump table in [./references/semver-and-notes.md](./references/semver-and-notes.md#bump-table).
5. If no `feat`/`fix` commits remain, list any excluded-type commits whose diff touches `cmd/`, `internal/`, `pkg/`, or `js/` and ask the user whether they justify a cut; if there are none, stop and report that there is nothing to cut.

### Phase 3 — Draft notes and confirm

The notes are the product surface of this skill: a news update for people who download the binary, not a commit inventory.

1. Read [./references/semver-and-notes.md](./references/semver-and-notes.md) for the voice, required sections, and quality gate.
2. Read `gh release view PREV_TAG --json body --jq .body`. Match its voice if it follows the required sections; if it starts with `## Changelog`, use `v0.7.0` as the voice reference instead.
3. Set `NOTES_FILE` to the absolute path printed by `printf '%s\n' "${TMPDIR:-/tmp}/coach-release-notes-NEXT_TAG.md"`, write the notes there, and apply the quality gate until it passes.
4. Check `README.md` for the line `latest published tag is` and the `mise use -g github:lousy-agents/coach@vX.Y.Z` example. If either names a tag other than `NEXT_TAG`, propose updating both — `README.md` ships inside the archives, so the change lands before the tag.
5. Show `PREV_TAG`, `NEXT_TAG`, `TARGET_SHA`, the bump reason, `NOTES_FILE`, the full notes, and any proposed README change. Ask: "Do this version, these notes, and the README change look right?"
6. End the turn. Continue only after the user says yes; apply any edits they request and re-confirm.

### Phase 4 — README prep PR, only if confirmed

Skip to Phase 5 when Phase 3 proposed no README change. The version itself comes from the tag (GoReleaser sets `main.version` via ldflags).

1. Confirm `git status --porcelain` is empty, then `git switch -c release/NEXT_TAG origin/main`.
2. Edit only the confirmed README lines and commit as `chore(release): prepare NEXT_TAG`.
3. Push, run `gh pr create --base main` filling `.github/PULL_REQUEST_TEMPLATE.md`, and end the turn with the PR URL.
4. When the user reports it merged, run `git fetch origin main`, then set `TARGET_SHA` to `gh pr view <N> --json mergeCommit --jq .mergeCommit.oid`. If `git log <old TARGET_SHA>..TARGET_SHA --no-merges` shows commits other than the prep commit, show them and re-confirm the notes. Re-run the Phase 1 CI check for the new `TARGET_SHA`.

### Phase 5 — Tag hand-off

1. Confirm `git ls-remote --tags origin NEXT_TAG` prints nothing and `git rev-parse -q --verify refs/tags/NEXT_TAG` fails (no stale local tag).
2. Show these commands, ask "May I run these two commands?", and end the turn:

   ```sh
   git tag -a NEXT_TAG TARGET_SHA -m "NEXT_TAG"
   git push origin NEXT_TAG
   ```

3. After the user approves, run exactly those two commands.
4. Poll `gh run list --workflow=release.yml --branch NEXT_TAG --json databaseId --jq '.[0].databaseId'` until it prints an ID, then run `gh run watch <id> --exit-status` in the background or with a 600000 ms timeout — the job takes several minutes.

If the run fails or the release lacks archives, stop and follow [./references/tag-handoff.md](./references/tag-handoff.md).

### Phase 6 — Replace the changelog body

1. If `NOTES_FILE` no longer exists (a long PR wait can outlive the temp directory), rewrite it from the notes the user confirmed.
2. `gh release view NEXT_TAG --json assets --jq '[.assets[].name]'` lists `coach_darwin_arm64.tar.gz`, `coach_darwin_x86_64.tar.gz`, `coach_linux_x86_64.tar.gz`, `coach_windows_x86_64.zip`, `checksums.txt`, and `checksums.txt.bundle`.
3. `gh release edit NEXT_TAG --title "NEXT_TAG — <headline>" --notes-file NOTES_FILE`
4. `diff <(gh release view NEXT_TAG --json body --jq .body) NOTES_FILE` shows no content difference (a trailing-newline difference is fine).
5. Delete `NOTES_FILE`.

## Done when

- An annotated tag `NEXT_TAG` on the confirmed `TARGET_SHA` exists on `origin`.
- The `release` job succeeded and the GitHub Release has all six assets from Phase 6.
- The published body matches the confirmed notes.
- The final message leads with the release URL, then lists anything skipped or left for the user (for example, an earlier release whose body is still a GoReleaser changelog).

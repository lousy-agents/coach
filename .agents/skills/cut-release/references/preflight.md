# Preflight fixes

What to report when a preflight or CI check fails. Each row ends the run until the user resolves it.

| Check | Failure | Report to the user |
| --- | --- | --- |
| Origin | Not `lousy-agents/coach` | Name the actual origin and ask whether it is the intended target. |
| Auth | `gh auth status` fails | Ask them to run `gh auth login` in their terminal. |
| Workflow trigger | `release.yml` no longer triggers on `v*.*.*` tags | Pushing a tag would publish nothing. Quote the current `on:` block. |
| Publish step | The `release` job no longer runs `goreleaser release --clean` | Quote the job; Phase 5 assumes that step publishes. |
| CI on `TARGET_SHA` | No `ci.yml` run, or not `completed`/`success` | Give the run URL. `ci.yml` cancels superseded runs on `main`, so a cancelled run means that commit was never fully tested; ask whether to cut from the newer tip instead. |
| Worktree (Phase 4 only) | `git status --porcelain` prints anything | List the dirty paths; the prep branch needs a clean tree. |

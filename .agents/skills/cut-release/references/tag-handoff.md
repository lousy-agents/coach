# Tag hand-off failures

Recovery goes through the CI `release` job (see Hard stops in `SKILL.md`).

| Case | Action |
| --- | --- |
| `NEXT_TAG` already exists on `origin` | Stop before tagging. Report it with `git show NEXT_TAG --no-patch` and ask whether to pick the next version. |
| A stale local `NEXT_TAG` exists | Report where it points. The user decides whether to delete the local tag. |
| A lightweight tag was pushed | Stop. Report it and leave it in place; the user decides what to do with it. |
| The `release` run failed before GoReleaser created the release | Read the failing step with `gh run view <id> --log-failed` and report it. Offer `gh run rerun <id> --failed` only for a transient failure (network, runner) and only if the user agrees. |
| The release exists but assets are missing, or a rerun is otherwise needed | Stop and report which assets are missing. A rerun reruns GoReleaser, which collides with the assets already uploaded, so the user decides whether to delete the release (never the tag) and fix the workflow or `.goreleaser.yaml` before rerunning. |

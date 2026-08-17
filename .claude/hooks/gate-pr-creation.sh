#!/bin/bash
# PreToolUse hook: blocks anything that publishes -- `git push`, `gh pr create`,
# and the GitHub MCP write tools -- on a dirty working tree, or when the cheap
# `mise run ci-gate` smoke check fails.
#
# This is no longer the exhaustive gate, and that is deliberate. It ran
# `mise run ci-all` when a warm ci-all was projected at ~41s. Measured on a CCR
# container it is ~910s -- against this hook's own 900s timeout -- while GitHub
# Actions runs the same atomic tasks as parallel required jobs, plus
# platform-smoke, in ~426s wall clock on compute that is not the session's.
# Re-running the suite here spent twice the wall clock, on the scarcer budget,
# to prove a strict subset of what GHA proves minutes later.
#
# The exhaustive gate is therefore GHA + branch protection, which is also the
# stronger placement: this hook runs inside the environment being gated, and a
# session where it never registered is indistinguishable from one where it did
# (see step 0 of .claude/commands/implement-issue.md). A required check runs
# where no agent can reach it.
#
# PRECONDITION: this is only safe while the CI jobs are required checks on the
# base branch. If branch protection is removed, nothing gates a red merge --
# restore `ci-all` here or re-protect the branch.
#
# What stays is what only a local check can do. GHA validates the *pushed
# commit*; it cannot see that the working tree differs from it -- a partial
# `git add`, a stray untracked file -- which would publish a tree nothing
# validated. That check is cheap git plumbing and runs first.
#
# Covers every publishing path, because which ones exist depends on the
# environment and none of them is redundant: `gh pr create` and `git push` via
# Bash (matcher: Bash, narrowed by the settings.json "if" clauses), and the
# GitHub MCP write tools -- create_pull_request, push_files,
# create_or_update_file, delete_file -- used where the gh CLI is absent. Local
# sessions have no GitHub MCP server at all; CCR has no gh. Denies via a JSON
# hookSpecificOutput permissionDecision, not the stderr+exit-2 convention
# validate-no-git-writes.sh uses.
set -euo pipefail
input=$(cat)
tool_name=$(jq -r '.tool_name // empty' <<<"$input")

. "$(dirname "${BASH_SOURCE[0]}")/lib/trace.sh"

# The other hooks trace `fired` before their filter, so that "ran and did not
# match" stays distinguishable from "never registered". This one cannot: its
# Bash matcher is unguarded, so a pre-filter trace appends a row for every shell
# command in the session and empties the trace of meaning -- the same file
# AGENTS.md sends a human to, and the same pollution fixed from another
# direction one change ago.
#
# The liveness signal survives the move, because step 0 probes this gate with an
# actual publish command, which traces below.
if [ "$tool_name" != "Bash" ]; then
  trace gate "${tool_name:-<unset>}" fired
fi

if [ "$tool_name" = "Bash" ]; then
  command=$(jq -r '.tool_input.command // empty' <<<"$input")
  if [ -z "$command" ]; then
    exit 0
  fi
  # Both publish. `gh pr create` opens the pull request; `git push` puts the
  # commits that pull request describes onto the remote. Gating only the first
  # left every push unchecked -- the initial one and, once the exhaustive suite
  # moved to CI, every repair push made while driving a red PR to green.
  #
  # This filter is why adding a Bash registration is not enough on its own: an
  # unmatched command exits 0, so a `git push` registration pointing at this
  # script would have allowed silently until this line learned the verb.
  #
  # Anchored to a command position -- start of a line, or after a separator --
  # rather than searched anywhere in the string. Observed: a commit whose
  # message discussed this gate was refused by it, because the hook receives one
  # flat command string in which an invocation and a mention look identical. In
  # a repository whose commit messages are mostly about its own tooling that
  # misfires constantly. A false deny is safe but infuriating, and nothing about
  # the safe direction depends on matching mentions.
  echo "$command" | grep -qE '(^|[;&|]|&&|\|\|)[[:space:]]*(gh[[:space:]]+pr[[:space:]]+create|git[[:space:]]+push)\b' || exit 0
  trace gate "$tool_name" fired
else
  # git is not the only way to reach the remote. The GitHub MCP server writes
  # commits over the API with no shell involved, so a Bash-only gate watches one
  # of two doors.
  case "$tool_name" in
    mcp__github__create_pull_request | \
    mcp__github__push_files | \
    mcp__github__create_or_update_file | \
    mcp__github__delete_file) ;;
    *) exit 0 ;;
  esac
fi

deny() {
  trace gate "$tool_name" deny
  jq -n --arg reason "$1" '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "deny",
      permissionDecisionReason: $reason
    }
  }'
  exit 0
}

# Fail closed when git itself fails. Swallowing the error would make "git is
# missing" and "the tree is clean" the same observation, and the gate would
# allow a PR having verified nothing.
if ! worktree_status=$(git status --porcelain 2>&1); then
  deny "Could not determine whether the working tree is clean (git status failed: ${worktree_status}). Refusing rather than publishing an unverified tree."
fi

if [ -n "$worktree_status" ]; then
  deny "The working tree is dirty, so validation would not describe what this publishes. Commit or stash the changes, then retry."
fi

if ! mise run ci-gate; then
  deny "mise run ci-gate failed; fix before publishing. (This is the cheap smoke check -- the full suite runs in GitHub Actions.)"
fi

trace gate "$tool_name" allow
exit 0

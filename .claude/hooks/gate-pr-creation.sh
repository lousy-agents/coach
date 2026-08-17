#!/bin/bash
# PreToolUse hook: blocks PR creation on a dirty working tree, or when the
# cheap `mise run ci-gate` smoke check fails.
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
# Covers both PR-creation paths,
# since which one is available depends on the Claude Code environment: a
# Bash `gh pr create` invocation (matcher: Bash, filtered via the settings.json
# "if" clause to just that command) and the GitHub MCP create_pull_request
# tool call (matcher: mcp__github__create_pull_request) used in environments
# where the gh CLI isn't available and PR creation goes through the MCP
# server instead. Denies via a JSON hookSpecificOutput permissionDecision,
# not the stderr+exit-2 convention validate-no-git-writes.sh uses.
set -euo pipefail
input=$(cat)
tool_name=$(jq -r '.tool_name // empty' <<<"$input")

. "$(dirname "${BASH_SOURCE[0]}")/lib/trace.sh"
trace gate "${tool_name:-<unset>}" fired

if [ "$tool_name" = "Bash" ]; then
  command=$(jq -r '.tool_input.command // empty' <<<"$input")
  if [ -z "$command" ]; then
    exit 0
  fi
  echo "$command" | grep -qE '\bgh[[:space:]]+pr[[:space:]]+create\b' || exit 0
elif [ "$tool_name" != "mcp__github__create_pull_request" ]; then
  exit 0
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
  deny "The working tree is dirty, so validation would not describe the commit this PR publishes. Commit or stash the changes, then retry."
fi

if ! mise run ci-gate; then
  deny "mise run ci-gate failed; fix before opening the PR. (This is the cheap smoke check -- the full suite runs in GitHub Actions.)"
fi

trace gate "$tool_name" allow
exit 0

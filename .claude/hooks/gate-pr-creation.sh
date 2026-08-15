#!/bin/bash
# PreToolUse hook: blocks PR creation unless `mise run ci-all` currently passes
# against a clean working tree, so a red suite can never be opened as a PR.
#
# `ci-all` rather than `ci`: `ci` reaches neither `wasm-build` nor a
# sidecar-built run of pkg/projectmodel's acceptance suite, which skips
# silently without it (see .github/workflows/ci.yml:52-56). A green `ci` is
# therefore not evidence that either was proven.
#
# Clean tree first: the suite validates the *working* tree, but a pull request
# publishes committed history. If they differ -- a partial `git add`, a stray
# untracked file -- the PR ships a tree nothing validated. The check is cheap
# git plumbing and short-circuits before the expensive suite runs.
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
  jq -n --arg reason "$1" '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "deny",
      permissionDecisionReason: $reason
    }
  }'
  exit 0
}

if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
  deny "The working tree is dirty, so validation would not describe the commit this PR publishes. Commit or stash the changes, then retry."
fi

if ! mise run ci-all; then
  deny "mise run ci-all failed; fix before opening the PR."
fi

exit 0

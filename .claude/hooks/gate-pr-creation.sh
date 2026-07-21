#!/bin/bash
# PreToolUse hook: blocks PR creation unless `mise run ci` currently passes,
# so a red suite can never be opened as a PR. Covers both PR-creation paths,
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

if ! mise run ci; then
  jq -n '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "deny",
      permissionDecisionReason: "mise run ci failed; fix before opening the PR."
    }
  }'
  exit 0
fi

exit 0

#!/bin/bash
# PreToolUse hook: blocks `gh pr create` unless `mise run ci` currently
# passes, so a red suite can never be opened as a PR. Mirrors
# validate-no-git-writes.sh's deny convention.
set -euo pipefail
input=$(cat)
command=$(jq -r '.tool_input.command // empty' <<<"$input")

if [ -z "$command" ]; then
  exit 0
fi

echo "$command" | grep -qE '\bgh[[:space:]]+pr[[:space:]]+create\b' || exit 0

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

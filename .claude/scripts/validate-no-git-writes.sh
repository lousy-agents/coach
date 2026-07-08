#!/bin/bash
# PreToolUse hook for the task-implementer subagent: blocks git/gh commands
# that publish or persist history, since the orchestrator owns git for
# multi-agent implementation workflows. Read-only git commands (status,
# diff, log, show) are allowed.

INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

if [ -z "$COMMAND" ]; then
  exit 0
fi

if echo "$COMMAND" | grep -iE '\bgit[[:space:]]+(commit|push|merge|rebase|reset|cherry-pick|tag)\b' > /dev/null; then
  echo "Blocked: git write/publish operations are reserved for the orchestrator." >&2
  exit 2
fi

if echo "$COMMAND" | grep -iE '\bgh[[:space:]]+(pr|release)[[:space:]]+(create|merge|edit|close)\b' > /dev/null; then
  echo "Blocked: opening or modifying PRs/releases is reserved for the orchestrator." >&2
  exit 2
fi

exit 0

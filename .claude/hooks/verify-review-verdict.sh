#!/bin/bash
# SubagentStop hook for the task-reviewer subagent: enforces that its final
# reply is exactly PASS or FINDINGS, per its own system prompt. This checks
# shape only, never verdict content — a malformed reply is blocked so the
# subagent re-emits a valid verdict instead of the orchestrator receiving
# something it can't parse.
set -euo pipefail
input=$(cat)
verdict=$(jq -r '.last_assistant_message // empty' <<<"$input")

if echo "$verdict" | grep -qE '^PASS\b'; then
  exit 0
fi
if echo "$verdict" | grep -qE '^FINDINGS\b'; then
  exit 0
fi

jq -n '{
  decision: "block",
  reason: "task-reviewer must return exactly PASS or FINDINGS, verbatim, per its system prompt. Re-emit a valid verdict in that exact shape."
}'
exit 0

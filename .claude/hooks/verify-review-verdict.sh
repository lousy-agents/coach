#!/bin/bash
# SubagentStop hook for the task-reviewer subagent: enforces that its final
# reply begins with PASS or FINDINGS, per its own system prompt. This checks
# shape only, never verdict content — a malformed reply is blocked so the
# subagent re-emits a valid verdict instead of the orchestrator receiving
# something it can't parse.
set -euo pipefail
input=$(cat)
verdict=$(jq -r '.last_assistant_message // empty' <<<"$input")

# grep's ^ anchors to the start of every line, not the start of the string, so
# checking the whole (possibly multi-line) message would let PASS/FINDINGS
# appearing after leading prose slip through. Anchor to the first non-empty
# line instead.
first_line=$(printf '%s\n' "$verdict" | sed -n '/[^[:space:]]/{p;q;}')

if echo "$first_line" | grep -qE '^PASS\b'; then
  exit 0
fi
if echo "$first_line" | grep -qE '^FINDINGS\b'; then
  exit 0
fi

jq -n '{
  decision: "block",
  reason: "task-reviewer must begin its reply with PASS or FINDINGS, verbatim, per its system prompt. Re-emit a valid verdict in that exact shape."
}'
exit 0

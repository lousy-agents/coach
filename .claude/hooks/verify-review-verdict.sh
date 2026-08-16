#!/bin/bash
# SubagentStop hook for the task-reviewer subagent: enforces that its final
# reply begins with PASS or FINDINGS, per its own system prompt. This checks
# shape only, never verdict content — a malformed reply is blocked so the
# subagent re-emits a valid verdict instead of the orchestrator receiving
# something it can't parse.
set -euo pipefail
input=$(cat)
# Optional trace. When COACH_HOOK_TRACE names a file, every invocation appends
# one line: which hook ran, what it matched, and what it decided. A hook that
# never fires is otherwise indistinguishable from one that passed -- silence
# looks identical either way -- so a run that needs evidence sets this.
trace() {
  [ -n "${COACH_HOOK_TRACE:-}" ] || return 0
  printf '%s\t%s\t%s\n' "$1" "$2" "$3" >> "$COACH_HOOK_TRACE" 2>/dev/null || true
}
agent_type=$(jq -r '.agent_type // empty' <<<"$input")
verdict=$(jq -r '.last_assistant_message // empty' <<<"$input")

# grep's ^ anchors to the start of every line, not the start of the string, so
# checking the whole (possibly multi-line) message would let PASS/FINDINGS
# appearing after leading prose slip through. Anchor to the first non-empty
# line instead.
trace verdict "${agent_type:-<unknown>}" "checking"
first_line=$(printf '%s\n' "$verdict" | sed -n '/[^[:space:]]/{p;q;}')

if echo "$first_line" | grep -qE '^PASS\b'; then
  trace verdict "${agent_type:-<unknown>}" allow
  exit 0
fi
if echo "$first_line" | grep -qE '^FINDINGS\b'; then
  trace verdict "${agent_type:-<unknown>}" allow
  exit 0
fi

trace verdict "${agent_type:-<unknown>}" block
jq -n '{
  decision: "block",
  reason: "task-reviewer must begin its reply with PASS or FINDINGS, verbatim, per its system prompt. Re-emit a valid verdict in that exact shape."
}'
exit 0

#!/bin/bash
# PreToolUse hook on the Agent tool: backstop for the orchestrator forgetting
# to relay reviewer findings entirely. If a task-implementer delegation
# mentions rework but doesn't contain the reviewer's "## Reviewer Findings"
# block verbatim, deny it. Presence-only — it cannot verify the forwarded
# block is accurate or complete, only that some block exists.
set -euo pipefail
input=$(cat)
subagent_type=$(jq -r '.tool_input.subagent_type // empty' <<<"$input")
prompt=$(jq -r '.tool_input.prompt // empty' <<<"$input")

# Optional trace -- see verify-review-verdict.sh for why. A hook that never
# fires looks exactly like one that passed; this makes the difference visible.
trace() {
  [ -n "${COACH_HOOK_TRACE:-}" ] || return 0
  printf '%s\t%s\t%s\n' "$1" "$2" "$3" >> "$COACH_HOOK_TRACE" 2>/dev/null || true
}

if [ "$subagent_type" != "task-implementer" ]; then
  exit 0
fi
trace relay "$subagent_type" checking

if echo "$prompt" | grep -qiE 'reviewer.{0,40}finding|re-?delegat|re-review'; then
  if ! echo "$prompt" | grep -qF '## Reviewer Findings'; then
    jq -n '{
      hookSpecificOutput: {
        hookEventName: "PreToolUse",
        permissionDecision: "deny",
        permissionDecisionReason: "Re-delegation after FINDINGS must include the reviewer'\''s \"## Reviewer Findings\" block verbatim, not a paraphrase."
      }
    }'
    trace relay "$subagent_type" deny
    exit 0
  fi
fi

trace relay "$subagent_type" allow
exit 0

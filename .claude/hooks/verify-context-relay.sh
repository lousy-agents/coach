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

if [ "$subagent_type" != "task-implementer" ]; then
  exit 0
fi

# Here-strings, not `echo | grep -q`: grep -q exits at the first match, the
# writer then dies of SIGPIPE, and under pipefail that reads as "no match" --
# a large rework prompt would slip past this gate (fail-open).
if grep -qiE 'reviewer.{0,40}finding|re-?delegat|re-review' <<<"$prompt"; then
  if ! grep -qF '## Reviewer Findings' <<<"$prompt"; then
    jq -n '{
      hookSpecificOutput: {
        hookEventName: "PreToolUse",
        permissionDecision: "deny",
        permissionDecisionReason: "Re-delegation after FINDINGS must include the reviewer'\''s \"## Reviewer Findings\" block verbatim, not a paraphrase."
      }
    }'
    exit 0
  fi
fi

exit 0

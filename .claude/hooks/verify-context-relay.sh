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

if echo "$prompt" | grep -qiE 'finding|re-?delegat|re-review'; then
  if ! echo "$prompt" | grep -qF '## Reviewer Findings'; then
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

#!/bin/bash
# SubagentStop hook: a mechanical ceiling on how many reviews one run may spend.
#
# The per-task cycle cap in .claude/commands/implement-issue.md is prose. An
# orchestrator that loses track of it -- or a harness that reads the file
# differently -- loops until the budget is gone, and nothing outside the model
# notices. This is the floor under that rule: a count the model cannot talk its
# way past, because it runs outside the model.
#
# It is deliberately cruder than the prose rule. The prose knows about tasks and
# progress; this only knows how many reviews the run has spent in total. It is a
# runaway backstop, not a replacement for the per-task cap -- a run that trips
# this has already gone wrong.
#
# The ceiling is generous on purpose. A four-task issue with a rework each is
# well inside it; only a loop that is not converging reaches it.
set -euo pipefail
input=$(cat)

. "$(dirname "${BASH_SOURCE[0]}")/lib/trace.sh"

CEILING="${COACH_CYCLE_CEILING:-24}"
state_dir="${COACH_CYCLE_STATE_DIR:-${COACH_REPO_ROOT}/.coach-cycle-state}"

# Per session: two runs sharing a checkout must not inherit each other's count,
# or the second starts pre-exhausted and the ceiling fires on a healthy loop.
session=$(jq -r '.session_id // "unknown"' <<<"$input" | tr -c 'A-Za-z0-9_-' '_')
counter="${state_dir}/${session}"

block() {
  trace ceiling "$session" block
  jq -n --arg reason "$1" '{decision: "block", reason: $reason}'
  exit 0
}

# Fail closed. A counter that cannot be written is a ceiling that is not
# enforcing, and an unenforced ceiling is indistinguishable from a healthy run
# -- the ambiguity this whole mechanism exists to remove.
if ! mkdir -p "$state_dir" 2>/dev/null; then
  block "Could not record the review count (${state_dir} is not writable), so the cycle ceiling cannot be enforced. Stop the run rather than looping uncounted."
fi

count=0
[ -f "$counter" ] && count=$(cat "$counter" 2>/dev/null || echo 0)
count=$((count + 1))
if ! printf '%s' "$count" > "$counter" 2>/dev/null; then
  block "Could not record the review count, so the cycle ceiling cannot be enforced. Stop the run rather than looping uncounted."
fi

trace ceiling "$session" "$count"

if [ "$count" -gt "$CEILING" ]; then
  block "This run has spent ${count} reviews, past the ceiling of ${CEILING}. A loop this long is not converging: stop, report the task it stalled on with reason repeated-finding, and do not open a PR."
fi

exit 0

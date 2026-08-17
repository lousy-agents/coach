#!/bin/bash
# PreToolUse hook: refuses to open a pull request from a dirty working tree.
#
# That is the whole hook, and the narrowness is deliberate. It has run
# `mise run ci-all`, then `mise run ci-gate`, and neither earned its place: both
# re-proved locally what GitHub Actions proves as required jobs, on compute that
# is not the session's. Since the `status` check became required on the base
# branch, a red tree cannot merge whatever this hook decides, so validating here
# buys latency, not safety.
#
# What is left is the one thing CI structurally cannot do. CI validates the
# *pushed commit*. Only something local can notice that the working tree differs
# from it -- a partial `git add`, a stray untracked file -- which does not make
# the merge unsafe, but does leave the pull request body's acceptance evidence
# and red-then-green proof describing a tree nobody pushed. This hook protects
# the integrity of that evidence, and nothing more.
#
# Scope note, learned the expensive way: an earlier revision also gated
# `git push` and the GitHub MCP write tools, on an unguarded Bash matcher. It
# ran on every shell command in the session (~17ms each), refused a commit whose
# *message* mentioned pushing, and flooded the hook trace. All of that was
# defending a property branch protection already provides. If you are tempted to
# widen this again, check first whether the thing you are protecting is not
# already a required check.
#
# Denies via a JSON hookSpecificOutput permissionDecision, not the stderr+exit-2
# convention validate-no-git-writes.sh uses.
set -euo pipefail
input=$(cat)
tool_name=$(jq -r '.tool_name // empty' <<<"$input")

. "$(dirname "${BASH_SOURCE[0]}")/lib/trace.sh"
trace gate "${tool_name:-<unset>}" fired

# Which path is available depends on the environment, and neither registration
# is redundant: local sessions have no GitHub MCP server at all, and CCR has no
# gh (exit 127).
if [ "$tool_name" = "Bash" ]; then
  command=$(jq -r '.tool_input.command // empty' <<<"$input")
  if [ -z "$command" ]; then
    exit 0
  fi
  # Anchored to a command position rather than searched anywhere in the string,
  # because the hook receives one flat command string in which an invocation and
  # a mention look identical -- and a commit message discussing this gate is a
  # mention. A false deny is safe but infuriating.
  echo "$command" | grep -qE '(^|[;&|]|&&|\|\|)[[:space:]]*gh[[:space:]]+pr[[:space:]]+create\b' || exit 0
elif [ "$tool_name" != "mcp__github__create_pull_request" ]; then
  exit 0
fi

deny() {
  trace gate "$tool_name" deny
  jq -n --arg reason "$1" '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "deny",
      permissionDecisionReason: $reason
    }
  }'
  exit 0
}

# Fail closed when git itself fails. Swallowing the error would make "git is
# missing" and "the tree is clean" the same observation, and the gate would
# allow a PR having verified nothing.
if ! worktree_status=$(git status --porcelain 2>&1); then
  deny "Could not determine whether the working tree is clean (git status failed: ${worktree_status}). Refusing rather than opening a pull request whose evidence may describe a tree nobody pushed."
fi

if [ -n "$worktree_status" ]; then
  deny "The working tree is dirty, so this pull request's evidence would describe a tree that was never pushed. Commit or stash the changes, then retry."
fi

trace gate "$tool_name" allow
exit 0

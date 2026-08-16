# Shared hook trace. Sourced, never executed.
#
# A hook that never fires looks exactly like one that passed -- both produce
# silence -- and that ambiguity is what has repeatedly let unenforced guarantees
# look enforced in this repository. When tracing is on, every invocation appends
# one row: hook, discriminator, decision.
#
# Two ways to turn it on, because one of them does not work where it matters.
# COACH_HOOK_TRACE is convenient locally, but an environment variable cannot be
# set in a CCR session without machine-local or account-level configuration --
# the exact thing the clone test rules out, and the reason setup-mise.sh is a
# committed hook rather than a setup script. So a gitignored marker file works
# too: any session can create it with one command, and it travels with the
# checkout rather than the machine.
#
# The discriminator column is best-effort. The verdict hook wants the agent
# type, since one script is registered under two SubagentStop matchers, but
# nothing here has been observed supplying `.agent_type` -- it records `<unset>`
# until a real run shows otherwise. Read the column as "what this hook could
# see", not as a guaranteed identity.
#
# Each hook emits a `fired` row *before* its early-exit filter. Without that,
# "the hook ran and the payload did not match" and "the hook was never
# registered" both leave an empty file -- the exact distinction being bought.

# Resolved once, at source time: inside a function BASH_SOURCE names the file
# the function was defined in, and dirname of a bare invocation is ".", so
# deferring this produces paths that depend on how the hook was called.
COACH_HOOK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COACH_REPO_ROOT="$(cd "${COACH_HOOK_DIR}/../.." && pwd)"

trace() {
  local target="${COACH_HOOK_TRACE:-}"
  if [ -z "$target" ]; then
    [ -f "${COACH_HOOK_DIR}/.trace-enabled" ] || return 0
    target="${COACH_REPO_ROOT}/.coach-hook-trace.tsv"
  fi
  # The redirection is brace-grouped because bash reports a failed `>>` on the
  # shell's own stderr *before* a trailing 2>/dev/null applies; without the
  # group, an unwritable path makes every hook invocation spray diagnostics.
  { printf '%s\t%s\t%s\n' "$1" "$2" "$3" >> "$target"; } 2>/dev/null || true
}

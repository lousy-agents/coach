# Shared hook trace. Sourced, never executed.
#
# A hook that never fires looks exactly like one that passed -- both produce
# silence -- and that ambiguity is what has repeatedly let unenforced guarantees
# look enforced in this repository. When COACH_HOOK_TRACE names a writable file,
# every invocation appends one row: hook, discriminator, decision.
#
# The discriminator column is best-effort. The verdict hook wants the agent
# type, since one script is registered under two SubagentStop matchers, but
# nothing in this repository has been observed supplying `.agent_type` -- it
# records `<unset>` until a real run shows otherwise. Read the column as "what
# this hook could see", not as a guaranteed identity.
#
# Each hook emits a `fired` row *before* its early-exit filter. Without that,
# "the hook ran and the payload did not match" and "the hook was never
# registered" both leave an empty file -- the exact distinction being bought.
trace() {
  [ -n "${COACH_HOOK_TRACE:-}" ] || return 0
  # The redirection is brace-grouped because bash reports a failed `>>` on the
  # shell's own stderr *before* a trailing 2>/dev/null applies; without the
  # group, an unwritable path makes every hook invocation spray diagnostics.
  { printf '%s\t%s\t%s\n' "$1" "$2" "$3" >> "$COACH_HOOK_TRACE"; } 2>/dev/null || true
}

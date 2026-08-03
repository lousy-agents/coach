#!/usr/bin/env bash
# SessionStart hook: make the repository's pinned toolchain available in a
# Claude Code cloud session.
#
# A cloud container starts without this repository's mise-pinned Go and Node.
# Without this hook, Bash tools either fail or silently use the wrong toolchain.
#
# Local sessions are left alone: developers manage their own mise install, and
# rewriting their PATH from a session hook would be an unpleasant surprise.
#
# This deliberately does NOT run project dependency installs or build steps.
# Its job is the toolchain only; pulling module/npm deps into every session
# start would slow sessions that only read code.
#
# mise itself is installed with npm (global prefix ~/.local) when missing or
# older than min_version in mise.toml — not via curl, cargo, or apt.
set -euo pipefail

if [[ "${CLAUDE_CODE_REMOTE:-}" != "true" ]]; then
  exit 0
fi

# Default rather than expand bare: under `set -u` an unset CLAUDE_PROJECT_DIR
# aborts the hook before it can report anything useful, and the whole toolchain
# bootstrap silently does not happen. The harness does set it, but a hook that
# hard-fails on a missing environment variable is a hook that fails invisibly.
project_dir="${CLAUDE_PROJECT_DIR:-$PWD}"
# Redirect: `cd` writes the resolved directory to stdout when CDPATH is set, and
# SessionStart stdout is injected into the conversation context.
cd "$project_dir" >/dev/null

# Name the directory that was actually searched. Without this the next command
# fails as `sed: mise.toml: No such file or directory`, which does not say which
# directory the hook resolved — the one fact needed to tell a missing
# CLAUDE_PROJECT_DIR apart from a wrong one.
if [[ ! -f mise.toml ]]; then
  echo "mise.toml not found in $project_dir; skipping toolchain bootstrap." >&2
  exit 1
fi

# The cloud environment may cache ~/.local on disk but does not persist PATH, so
# a mise installed by an earlier session is present but not yet reachable.
export PATH="$HOME/.local/bin:$PATH"

mise_version="$(sed -n 's/^min_version[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' mise.toml)"

if [[ ! "$mise_version" =~ ^[0-9]{4}\.[0-9]+\.[0-9]+$ ]]; then
  echo "mise.toml must define min_version as YYYY.M.PATCH" >&2
  exit 1
fi

# Returns 0 when $1 >= $2 for YYYY.M.PATCH versions.
#
# Every field is compared with an explicit 10# radix. Bash arithmetic reads a
# leading zero as octal, so a zero-padded month like 2026.08.0 would otherwise
# raise "value too great for base", and `[[ ]]` returns 2 — which `if` treats as
# false, silently skipping the month comparison and deciding a stale mise is
# current. Both callers' inputs are digit-only by the time they reach here, so
# 10# cannot fail.
mise_version_ge() {
  local installed="$1" required="$2"
  local i1 i2 i3 r1 r2 r3
  IFS=. read -r i1 i2 i3 <<< "$installed"
  IFS=. read -r r1 r2 r3 <<< "$required"
  if [[ "10#$i1" -ne "10#$r1" ]]; then
    [[ "10#$i1" -gt "10#$r1" ]]
    return
  fi
  if [[ "10#$i2" -ne "10#$r2" ]]; then
    [[ "10#$i2" -gt "10#$r2" ]]
    return
  fi
  [[ "10#$i3" -ge "10#$r3" ]]
}

needs_install=false
if ! command -v mise >/dev/null 2>&1; then
  needs_install=true
else
  # grep exits 1 when no version token matches; keep that non-fatal so the
  # empty-version branch below can trigger a reinstall under pipefail.
  installed_version="$(mise --version 2>/dev/null | grep -oE '[0-9]{4}\.[0-9]+\.[0-9]+' | head -n1 || true)"
  if [[ -z "$installed_version" ]] || ! mise_version_ge "$installed_version" "$mise_version"; then
    needs_install=true
  fi
fi

if [[ "$needs_install" == true ]]; then
  # SessionStart stdout is injected into the conversation context, so npm's
  # progress output goes to stderr.
  #
  # Best-effort for the same reason as `mise trust` below: when a stale but
  # working mise is already on PATH, a transient npm failure (registry blip,
  # network policy) must not cost the session its PATH export entirely.
  npm install \
    --global \
    --prefix "$HOME/.local" \
    --no-audit \
    --no-fund \
    "mise@$mise_version" >&2 ||
    echo "npm install of mise@$mise_version failed; continuing with the mise already on PATH." >&2
fi

if ! command -v mise >/dev/null 2>&1; then
  echo "no mise on PATH after install attempt; skipping toolchain bootstrap." >&2
  exit 1
fi

# Best-effort: confirmed `mise trust` can exit non-zero (e.g. an unreadable or
# missing config), and under this script's `set -e`, an unguarded failure here
# would abort the toolchain bootstrap entirely before `mise install` ever runs
# — the one step this whole hook exists to perform.
mise trust mise.toml >/dev/null 2>&1 || echo "mise trust failed; continuing without it" >&2

# Install every tool in [tools]. If the full install fails (e.g. a future
# optional tool), fall back to the tools every coach session needs so one bad
# pin cannot leave the session with no Go or Node at all.
if ! mise install >/dev/null 2>&1; then
  echo "mise install did not complete for every tool; installing go and node." >&2
  # Best-effort for the same reason as `mise trust` above: an unguarded failure
  # here aborts before the PATH export below, which would leave the session with
  # no PATH entry at all — strictly worse than the partial toolchain this
  # fallback exists to salvage.
  mise install go node >/dev/null 2>&1 ||
    echo "mise install go node failed; the session toolchain may be incomplete." >&2
fi

if [[ -n "${CLAUDE_ENV_FILE:-}" ]]; then
  # `mise bin-paths` exits non-zero when a tool in [tools] is not installed —
  # exactly the state the fallback above leaves behind. Persisting ~/.local/bin
  # alone still beats writing nothing, because it keeps mise itself reachable so
  # the session can retry the install by hand.
  bin_paths="$(mise bin-paths 2>/dev/null | paste -sd: - || true)"

  # An empty PATH element resolves to the current working directory, so only
  # emit tool bin paths when mise actually reports some.
  if [[ -n "$bin_paths" ]]; then
    path_line="$(printf 'export PATH=%q:%q:$PATH' "$HOME/.local/bin" "$bin_paths")"
  else
    path_line="$(printf 'export PATH=%q:$PATH' "$HOME/.local/bin")"
  fi

  # SessionStart fires on start and on every resume. This guard is exact-line,
  # so it only suppresses an identical re-append on resume; a toolchain bump
  # that changes bin_paths intentionally appends a new (shadowing) line.
  if ! grep -qxF "$path_line" "$CLAUDE_ENV_FILE" 2>/dev/null; then
    printf '%s\n' "$path_line" >> "$CLAUDE_ENV_FILE"
  fi
fi

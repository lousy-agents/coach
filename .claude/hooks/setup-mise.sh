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
cd "${CLAUDE_PROJECT_DIR:-$PWD}"

# The cloud environment may cache ~/.local on disk but does not persist PATH, so
# a mise installed by an earlier session is present but not yet reachable.
export PATH="$HOME/.local/bin:$PATH"

mise_version="$(sed -n 's/^min_version = "\([^"]*\)".*/\1/p' mise.toml)"

if [[ ! "$mise_version" =~ ^[0-9]{4}\.[0-9]+\.[0-9]+$ ]]; then
  echo "mise.toml must define min_version as YYYY.M.PATCH" >&2
  exit 1
fi

# Returns 0 when $1 >= $2 for YYYY.M.PATCH versions.
mise_version_ge() {
  local installed="$1" required="$2"
  local i1 i2 i3 r1 r2 r3
  IFS=. read -r i1 i2 i3 <<< "$installed"
  IFS=. read -r r1 r2 r3 <<< "$required"
  if [[ "$i1" -ne "$r1" ]]; then
    [[ "$i1" -gt "$r1" ]]
    return
  fi
  if [[ "$i2" -ne "$r2" ]]; then
    [[ "$i2" -gt "$r2" ]]
    return
  fi
  [[ "$i3" -ge "$r3" ]]
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
  npm install \
    --global \
    --prefix "$HOME/.local" \
    --no-audit \
    --no-fund \
    "mise@$mise_version" >&2
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
  mise install go node >/dev/null
fi

if [[ -n "${CLAUDE_ENV_FILE:-}" ]]; then
  # An empty PATH element resolves to the current working directory, so only
  # emit tool bin paths when mise actually reports some.
  bin_paths="$(mise bin-paths | paste -sd: -)"
  if [[ -n "$bin_paths" ]]; then
    printf 'export PATH=%q:%q:$PATH\n' "$HOME/.local/bin" "$bin_paths" >> "$CLAUDE_ENV_FILE"
  else
    printf 'export PATH=%q:$PATH\n' "$HOME/.local/bin" >> "$CLAUDE_ENV_FILE"
  fi
fi

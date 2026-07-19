#!/usr/bin/env bash
set -euo pipefail

if [[ "${CLAUDE_CODE_REMOTE:-}" != "true" ]]; then
  exit 0
fi

cd "$CLAUDE_PROJECT_DIR"

# Cloud env setup installs mise under ~/.local/bin and caches the binary on
# disk, but does not persist PATH. Prefer that location before probing.
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

mise trust mise.toml >/dev/null
mise install >/dev/null

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

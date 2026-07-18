#!/usr/bin/env bash
set -euo pipefail

if [[ "${CLAUDE_CODE_REMOTE:-}" != "true" ]]; then
  exit 0
fi

cd "$CLAUDE_PROJECT_DIR"

if ! command -v mise >/dev/null 2>&1; then
  mise_version="$(sed -n 's/^min_version = "\([^"]*\)".*/\1/p' mise.toml)"

  if [[ ! "$mise_version" =~ ^[0-9]{4}\.[0-9]+\.[0-9]+$ ]]; then
    echo "mise.toml must define min_version as YYYY.M.PATCH" >&2
    exit 1
  fi

  npm install     --global     --prefix "$HOME/.local"     --no-audit     --no-fund     "mise@$mise_version"

  export PATH="$HOME/.local/bin:$PATH"
fi

mise trust mise.toml
mise install

if [[ -n "${CLAUDE_ENV_FILE:-}" ]]; then
  bin_paths="$(mise bin-paths | paste -sd: -)"
  printf 'export PATH=%q:%q:$PATH\n'     "$HOME/.local/bin"     "$bin_paths"     >> "$CLAUDE_ENV_FILE"
fi

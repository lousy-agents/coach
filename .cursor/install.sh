#!/usr/bin/env bash
# Cloud Agent bootstrap for the coach repository.
#
# Idempotent: safe to re-run and safe when a build snapshot has already
# prepared part of the state. All project tasks run through `mise run`, so the
# pinned toolchain (Go/Node/pnpm/bun from mise.toml) is the source of truth for
# both local runs and CI.
set -euo pipefail

MISE_BIN="${HOME}/.local/bin/mise"

# 1. mise: the single tool-version source of truth (mise.toml pins go/node/pnpm/bun).
if [ ! -x "${MISE_BIN}" ]; then
  curl -fsSL https://mise.run | sh
fi

# Activate mise for the remainder of this script and for future interactive shells.
if ! grep -q 'mise activate bash' "${HOME}/.bashrc" 2>/dev/null; then
  printf '\neval "$(%s activate bash)"\n' "${MISE_BIN}" >>"${HOME}/.bashrc"
fi
eval "$("${MISE_BIN}" activate bash)"

# 2. Git signing: the cloud VM enables SSH commit signing globally. The
# cmd/coach acceptance suites create throwaway repos under the system temp dir
# and run `git commit`; inheriting global signing makes those non-interactive
# commits hang (10-minute test timeout). Scope a signing-off override to repos
# under /tmp/ via includeIf so real commits in the workspace stay signed.
NOSIGN_INC="${HOME}/.gitconfig-notmpsign"
if [ ! -f "${NOSIGN_INC}" ]; then
  cat >"${NOSIGN_INC}" <<'EOF'
[commit]
	gpgsign = false
[tag]
	gpgsign = false
EOF
fi
git config --global "includeIf.gitdir:/tmp/.path" "${NOSIGN_INC}"

# 3. strace: required by the ts-project-backend acceptance suite (file-syscall tracer).
if ! command -v strace >/dev/null 2>&1 && command -v sudo >/dev/null 2>&1; then
  sudo apt-get update -qq
  sudo apt-get install -y -qq strace
fi

# 4. Install the pinned toolchain declared in mise.toml.
"${MISE_BIN}" install

# 5. Warm dependency caches so the first `mise run` is fast and offline-safe.
"${MISE_BIN}" exec -- go mod download
"${MISE_BIN}" run js-install

#!/usr/bin/env bash
# Cloud environment setup script for Claude Code cloud sessions.
#
# Paste this script into the Claude Code cloud environment settings. It runs
# before Claude Code launches and its filesystem output is cached by Anthropic,
# so the tools below are already on disk in later sessions.
#
# The repository uses mise.toml as the source of truth. Keep the versions in
# this script aligned with [tools] and min_version in mise.toml. A CI parity
# test (internal/claudehooks/setup_mise_test.go) fails the build if they drift.
set -euo pipefail

export PATH="$HOME/.local/bin:$PATH"

mise_version="2026.7.13"
go_version="1.26.5"
node_version="24"

npm install \
  --global \
  --prefix "$HOME/.local" \
  --no-audit \
  --no-fund \
  "mise@${mise_version}"

mise --version
mise trust mise.toml
mise install "go@${go_version}" "node@${node_version}"

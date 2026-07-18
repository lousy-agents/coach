# Claude Code cloud development

Claude Code cloud sessions use the repository's `SessionStart` hook to
prepare the toolchain declared in `mise.toml`.

The hook:

- runs only when `CLAUDE_CODE_REMOTE=true`, leaving local sessions unchanged;
- installs the exact `mise` minimum version from `mise.toml` when `mise`
  is not already available;
- trusts the checked-out project configuration and installs its pinned tools;
- persists the active tool paths through `CLAUDE_ENV_FILE`, making commands
  such as `go` and `node` available to later Bash tool calls.

## Recommended cloud environment setup

Installing the `mise` binary in the cloud environment's setup script avoids
reinstalling it in each fresh session. Claude caches the setup script's
filesystem output, while the repository hook remains a fallback and handles
project-specific tools.

In the Claude Code cloud environment settings, use this setup script:

```bash
#!/usr/bin/env bash
set -euo pipefail

npm install --global \
  --no-audit \
  --no-fund \
  mise@2026.7.7

mise --version
```

Keep the installed version aligned with `min_version` in `mise.toml`. The
default Trusted network policy permits npm registry access, so this does not
require unrestricted network access.

## Verification

Start a new cloud session and ask Claude to run:

```bash
mise --version
go version
node --version
mise run ci
```

The expected project tool versions are defined only in `mise.toml`; do not
duplicate Go or Node versions in Claude configuration.

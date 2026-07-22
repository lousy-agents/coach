# Claude Code cloud development

Claude Code cloud sessions use a repository `SessionStart` hook and a cloud
environment setup script to prepare the toolchain declared in `mise.toml`.

There are two layers by design:

1. **Cloud environment setup script** — runs once while the environment cache
   builds. It installs `mise` and the pinned toolchains into the cached
   filesystem, so fresh sessions start with Go and Node already on disk.
2. **Repository `SessionStart` hook** — runs on every session start and resume
   after Claude Code launches. It verifies the `mise` version, trusts the
   checked-out project configuration, installs any missing tools, and persists
   the active tool paths through `CLAUDE_ENV_FILE` so `go`, `node`, and
   `mise run` later work through the Bash tool.

The hook only runs when `CLAUDE_CODE_REMOTE=true`, leaving local sessions
unchanged.

## Recommended cloud environment setup

Installing `mise` and its pinned tools in the cloud environment setup script
makes every subsequent session start fast. Claude caches the setup script's
filesystem output, while the repository hook remains a fallback and handles
project-specific reconciliation.

The setup script is committed at `.claude/cloud-env-setup.sh`. Paste that
file's contents into the Claude Code cloud environment settings — do not
re-copy version pins from this page. The script is the paste source of truth;
`mise.toml` is the pin source of truth. A CI parity test rejects PRs that
update one without the other.

After Renovate (or a human) bumps versions in those two files, rebuild the
cloud environment cache by re-pasting the updated script in the cloud UI (for
example, changing the `mise_version` line triggers a cache rebuild on the next
session).

The default Trusted network policy permits npm registry access and the hosts
mise uses for Go and Node downloads, so this does not require unrestricted
network access.

## Repository `SessionStart` hook

The hook at `.claude/hooks/setup-mise.sh` is triggered by `.claude/settings.json`.
It handles:

- walking away cleanly when `CLAUDE_CODE_REMOTE` is not `true`;
- parsing `min_version` from `mise.toml` and installing or upgrading `mise`
  through npm when the version on `PATH` is missing or too old;
- running `mise trust` and `mise install`;
- appending `export PATH=...` into `CLAUDE_ENV_FILE` so later Bash calls use the
  pinned tools by default.

Install noise from `mise` is redirected so that the hook produces empty stdout
on success. SessionStart stdout is otherwise injected into the conversation
context.

## Verification

Start a new cloud session and ask Claude to run:

```bash
mise --version
go version
node --version
mise run ci
```

The expected `mise`, Go, and Node versions are defined only in `mise.toml` and
reflected in `.claude/cloud-env-setup.sh`; do not duplicate them in Claude
configuration. If any verification shows a different version, update the
environment setup script and rebuild the cache.

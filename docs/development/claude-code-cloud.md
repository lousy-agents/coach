# Claude Code cloud development

Claude Code cloud sessions use a repository `SessionStart` hook to prepare the
toolchain declared in `mise.toml`.

There is a single bootstrap layer by design:

1. **Repository `SessionStart` hook** — runs on every session start and resume
   after Claude Code launches. When `CLAUDE_CODE_REMOTE=true`, it ensures `mise`
   is installed at the `min_version` pinned in `mise.toml` (via npm into
   `~/.local`), trusts the project config, installs the pinned tools (`go`,
   `node`), and persists active tool paths through `CLAUDE_ENV_FILE` so later
   Bash calls can run `go`, `node`, and `mise run`.

The hook only runs when `CLAUDE_CODE_REMOTE=true`, leaving local sessions
unchanged. Local developers manage their own mise install; the hook does not
rewrite PATH on a laptop.

## No cloud environment paste script

Do **not** paste a custom environment setup script that installs mise, Go, or
Node pins. Version pins live only in `mise.toml`. If an older coach cloud
environment still has a pasted setup script (for example a former
`cloud-env-setup.sh`), clear it in the Claude Code cloud environment settings
so it does not keep installing stale or duplicate pins beside the SessionStart
hook.

The default Trusted network policy permits npm registry access and the hosts
mise uses for Go and Node downloads, so this does not require unrestricted
network access.

## Repository `SessionStart` hook

The hook at `.claude/hooks/setup-mise.sh` is triggered by `.claude/settings.json`.
It handles:

- walking away cleanly when `CLAUDE_CODE_REMOTE` is not `true`;
- parsing `min_version` from `mise.toml` and installing or upgrading `mise`
  through npm (`npm install --global --prefix ~/.local mise@…`) when the
  version on PATH is missing or too old;
- best-effort `mise trust` (a trust failure does not abort bootstrap);
- `mise install`, with a fallback to `mise install go node` if the full
  install does not complete;
- appending `export PATH=...` into `CLAUDE_ENV_FILE` so later Bash calls use the
  pinned tools by default.

Install noise is redirected so that the hook produces empty stdout on success.
SessionStart stdout is otherwise injected into the conversation context.

## Verification

Start a new cloud session and ask Claude to run:

```bash
mise --version
go version
node --version
mise run ci
```

The expected `mise`, Go, and Node versions are defined only in `mise.toml`. If
any verification shows a different version, update `mise.toml` (and let the
next SessionStart reconcile).

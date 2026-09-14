# coach

`coach` is an experimental AI coach for people (and agents) who write software.
The thing you can use today is a local CLI: it reads a Git checkout, analyzes
Go / TypeScript / TSX source, and prints advisory structural signals.

Coach stays on the 0.x line on purpose. It does not contact GitHub or call a
model on this path. File-local analysis is Tree-sitter over committed Git
objects. TypeScript `--project-config` additionally runs a host Node process
and the TypeScript compiler; that is still local, not the platform preview.

## Contract

- **Languages:** Go, TypeScript, TSX. Other files get an `unsupported_language` diagnostic and are skipped.
- **Input:** committed Git objects at the selected revision, not the dirty worktree. Uncommitted edits are invisible.
- **Git:** `git` must be on `PATH`. No Docker, no API key, no network for the default CLI path.
- **Advisory:** a completed analysis exits `0` even when it found signals. Coach does not gate merges.
- **Absence is not a clean bill of health.** An empty signal set means no matched rule for the inputs that were analyzed.

Full flags, JSON schema, exit codes, rule IDs, and library quickstarts:
[CLI contract](./docs/cli-codesignal.md).

## Run

From any Git worktree. Prefer a one-shot that does not mutate a global toolchain:

```sh
mise exec github:lousy-agents/coach -- coach codesignal --base main
```

```sh
coach codesignal --base main          # diff HEAD against main
coach codesignal --baseline           # every tracked Go/TS/TSX file at HEAD
coach codesignal --base main --format json
```

`--base` can be any ref Git can resolve (branch, tag, or SHA). Default output is text. `--base` and `--baseline` are mutually exclusive.

Default `--scope` is `production`. `--build-target <pattern>` further limits Go production reachability; it is a no-op under `--scope all`.

### Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Analysis completed (signals or none). Not a quality gate. |
| `1` | Operational failure (not a git repo, missing `git`, unresolvable `--base`, empty repo, I/O). |
| `2` | Usage or invalid `--project-config`, or a TypeScript scan that cannot resolve a supported compiler or host Node (empty stdout, one stderr line). |

### What you should see

A short text report per signal (`path`, `line`, `lifecycle`, `changed`,
`evidence`, why it matters, recommendation), or a quiet report if nothing
matched. `lifecycle` is `introduced` / `existing` / `resolved` / `unknown`.

Diff-mode limits (do not treat these as a clean PR):

- Added files are analyzed but classified `unknown`; they do not increment `introduced_signals`.
- Renames and copies are skipped with an `unsupported_change_type` diagnostic.

Text `line` is 1-based. JSON `location.start_row` is 0-based.

## Install

Install [mise](https://mise.jdx.dev/) if you do not already have it, then pin
the CLI from [GitHub Releases](https://github.com/lousy-agents/coach/releases):

```sh
curl https://mise.run | sh
~/.local/bin/mise use -g github:lousy-agents/coach
```

That mutates a **global** mise config. Hook mise into your shell so `coach` is
on `PATH` (pick the one you use):

```sh
echo 'eval "$(~/.local/bin/mise activate bash)"' >> ~/.bashrc && eval "$(~/.local/bin/mise activate bash)"
# echo 'eval "$(~/.local/bin/mise activate zsh)"' >> ~/.zshrc && eval "$(~/.local/bin/mise activate zsh)"
# ~/.local/bin/mise activate fish | source
```

Pin a release if you want a fixed binary (latest published tag is `v0.6.0`):

```sh
mise use -g github:lousy-agents/coach@v0.6.0
```

Release archives exist for `darwin_arm64`, `darwin_x86_64`, `linux_x86_64`, and
`windows_x86_64`. There is no `linux_arm64` build yet.

<details>
<summary>From source instead of the release binary</summary>

This path uses the repo's `mise.toml` to pin the **dev toolchain** (Go 1.27.1,
Node 24). That is not the same as installing the published `coach` binary.

```sh
git clone https://github.com/lousy-agents/coach.git
cd coach
mise install
mise exec -- go install ./cmd/coach
```

</details>

<details>
<summary>Signed archive without mise</summary>

Download the archive for your platform from
[Releases](https://github.com/lousy-agents/coach/releases), verify
`checksums.txt` with cosign, then put `coach` on your `PATH`. Commands:
[CLI contract](./docs/cli-codesignal.md#signed-archive-without-mise).

</details>

## Preview — experimental, try it now

These commands work on the current CLI. They may change without a 1.0 promise.
Absence of findings is not compliance.

### 1. Project-level architecture policy

Opt into cross-file layer checks with a committed JSON config. Coach does not
guess your architecture — you declare layers and forbidden import edges.

```sh
# Go: discover module/workspace roots (candidate only — review, add layers, commit)
coach codesignal --baseline --suggest-project-config

# after the file is committed at the analyzed revision
coach codesignal --base main --project-config project.json
coach codesignal --baseline --project-config project.json --project-language typescript
```

`--project-config` is read from the **analyzed revision**, not your dirty
worktree. An uncommitted candidate is invisible to the scan. A valid config
switches the report to `schema_version: "2"` and can emit
`architecture.layer_violation`. `required_layer` can also emit
`architecture.layer_bypass` (Go and TypeScript) on a narrow handler→SQL
registry.

TypeScript `--project-config` uses Coach's embedded analyzer and a host-resolved
compiler whose own `package.json` version is in the supported set (`7.0.2`
today). Version ranges (`^`, `~`, `latest`) never count. If you previously
vendored Coach's analyzer under the analyzed repository, you can delete that
copy. `--suggest-project-config` never invents `layers` or `forbidden_imports`,
and it never auto-applies a config.

Go `--suggest-project-config` prints a candidate. TypeScript
`--suggest-project-config --project-language typescript` is an **interactive
TTY session** and writes nothing until you confirm. Do not run it unattended.

### 2. TypeScript project readiness

```sh
coach codesignal --baseline --check-project --project-language typescript
coach codesignal --baseline --suggest-project-config --project-language typescript
```

Run `--check-project` first. It is read-only: it reports compiler, Node, and
policy fit and exits `0` so you inspect the result instead of treating process
status as a gate. It is not a scan. A **scan** that cannot resolve a supported
compiler or host Node exits `2` with empty stdout and one stderr line.

Coach's supported Node majors are exactly `24` and `26`. Any other major is the
`node_unsupported` readiness gap. `@lousy-agents/coach-semantics` declares this
as `engines.node: "^24 || ^26"`.

`--prepare-compiler` is a consented, interactive mise TypeScript compiler-setup
session (requires a TTY; refuses otherwise). It prompts before installing.

Still being built — do not expect these to work yet: unattended package-manager
setup for the scanned project, Bun as a project runtime, and a packaged
foreign-repository TypeScript journey.

## Separate lab: local platform

The CLI above is the default product. A Docker API + worker stack can add
agent judgments on a baseline scan. That path uses Docker, may call a model,
and may read GitHub. It is not the default install.

Verified in CI: Path A (`mise run platform-up` / `platform-smoke`) with the
stub model. Paths B (real local model) and C (live GitHub.com) are operator
labs. Findings are tagged `source=deterministic` or `source=agent`; agent
output cannot suppress a deterministic finding.

[Local Coach quickstart](./docs/pilot-local-quickstart.md).

## Libraries

Analyzer as a library rather than the CLI — not required to run `coach`:

- [`pkg/semantics`](./pkg/semantics) — Go/TypeScript/TSX source bytes. [Quickstart](./docs/cli-codesignal.md#pkgsemantics-quickstart).
- `@lousy-agents/coach-semantics` — Node ESM bindings. Not published to npm; clone and build locally. [JS/TS Quickstart](./docs/cli-codesignal.md#javascript--typescript-quickstart).

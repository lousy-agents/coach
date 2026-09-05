# coach

`coach` is an experimental AI coach for people (and agents) who write software.
The thing you can use today is a local CLI: it reads a Git checkout, analyzes
Go / TypeScript / TSX source, and prints advisory structural signals — code
smells, design issues, and architecture-policy findings you opted into.

It does not execute your code, contact GitHub, or call a model unless you stand
up the separate [local platform preview](#3-local-platform-including-agent-judgments).
Coach stays on the 0.x line on purpose.

## Quick start

Install [mise](https://mise.jdx.dev/) if you do not already have it, then install
the `coach` CLI from [GitHub Releases](https://github.com/lousy-agents/coach/releases)
with mise's GitHub backend:

```sh
curl https://mise.run | sh
~/.local/bin/mise use -g github:lousy-agents/coach
```

Hook mise into your current shell so `coach` is on `PATH` (pick the one you use):

```sh
echo 'eval "$(~/.local/bin/mise activate bash)"' >> ~/.bashrc && eval "$(~/.local/bin/mise activate bash)"
# echo 'eval "$(~/.local/bin/mise activate zsh)"' >> ~/.zshrc && eval "$(~/.local/bin/mise activate zsh)"
# ~/.local/bin/mise activate fish | source
```

Pin a release if you want a fixed binary (latest published tag is `v0.4.0`):

```sh
mise use -g github:lousy-agents/coach@v0.4.0
```

From any Git worktree:

```sh
coach codesignal --base main          # diff HEAD against main
coach codesignal --baseline           # every tracked Go/TS/TSX file at HEAD
coach codesignal --base main --format json
```

`--base` can be any ref Git can resolve (branch, tag, or SHA). Text output is
the default. A completed analysis exits `0` even when it found signals — this
is advisory, not a gate.

You need `git` on `PATH`. No Docker, no API key, no network.

One-shot without writing a mise config or activating your shell:

```sh
mise exec github:lousy-agents/coach -- coach codesignal --base main
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
`checksums.txt` with cosign, then put `coach` on your `PATH`. The exact
commands live in the [CLI contract](./docs/cli-codesignal.md).

</details>

### What you should see

A short text report per signal (`path`, `line`, `lifecycle`, `changed`,
`evidence`, why it matters, recommendation), or a quiet report if the analyzed
files produced no active signals. `lifecycle` is `introduced` / `existing` /
`resolved` / `unknown` relative to the merge-base.

- **Advisory only.** Coach does not judge correctness or block merges.
- **Go, TypeScript, and TSX only.** Other languages are skipped with an
  `unsupported_language` diagnostic.
- **Static and local.** It reads source bytes via `git show`. File-local unless
  you pass `--project-config`.
- **Absence is not a clean bill of health.** An empty signal set means no
  matched rule for the inputs that were analyzed.

Full flag, schema, and exit-status contract: [CLI contract](./docs/cli-codesignal.md).

## Preview — experimental, try it now

These commands work on the current CLI or local stack. They are under active
development, may change without a 1.0 promise, and are here so you can kick
the tires. Absence of findings is not compliance.

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
`architecture.layer_violation`. For Go only, `required_layer` can also emit
`architecture.layer_bypass` on a narrow handler→SQL registry.

TypeScript `--project-config` still expects this repository's analyzer sidecar
layout. It is a preview, not a packaged foreign-repository TypeScript product
yet. `--suggest-project-config` never invents `layers` or `forbidden_imports`,
and it never auto-applies a config.

### 2. TypeScript project readiness

A guided on-ramp for first-class TypeScript project analysis
([epic #280](https://github.com/lousy-agents/coach/issues/280), in flight).

```sh
coach codesignal --baseline --check-project --project-language typescript
coach codesignal --baseline --suggest-project-config --project-language typescript
```

`--check-project` is read-only: it reports readiness gaps and exits `0` so you
inspect the result instead of treating the process status as a gate. The
interactive authoring session writes nothing until you say so.

Still being built — do not expect these to work yet: consented package-manager
/ mise compiler setup for the scanned project, Bun as a project runtime, and a
packaged foreign-repository TypeScript journey.

### 3. Local platform, including agent judgments

The CLI above never calls a model. The local API + worker stack can, if you
want a preview of deterministic findings *plus* agent judgments on a baseline
scan.

```sh
git clone https://github.com/lousy-agents/coach.git && cd coach
mise install
mise run platform-up
mise run platform-smoke    # expect: platform-smoke: ok
```

Full paths (stub model, local Qwen, GitHub.com scan without cloning):
[Local Coach quickstart](./docs/pilot-local-quickstart.md).
This is a pilot, not the default install. Findings are tagged
`source=deterministic` or `source=agent`; agent output cannot suppress a
deterministic finding.

Working in a Claude Code cloud session? See
[Claude Code cloud development](./docs/development/claude-code-cloud.md).

## Libraries

If you want the analyzer as a library rather than the CLI:

- [`pkg/semantics`](./pkg/semantics) — deterministic structural analysis of Go,
  TypeScript, and TSX source bytes. [Quickstart](./docs/cli-codesignal.md#pkgsemantics-quickstart).
- [`pkg/githubingest`](./pkg/githubingest) — optional GitHub App-authenticated
  single-file reader.
- `@lousy-agents/coach-semantics` — Node ESM bindings. Not published to npm yet;
  clone and build locally. [JS/TS Quickstart](./docs/cli-codesignal.md#javascript--typescript-quickstart).

```sh
go get github.com/lousy-agents/coach/pkg/semantics
go get github.com/lousy-agents/coach/pkg/githubingest
```

---

## `coach codesignal` CLI Preview

The full flag, schema, exit-status, and `--project-config` contract now lives in
[`docs/cli-codesignal.md`](./docs/cli-codesignal.md).

# coach

`coach` is an experimental AI coach for people (and agents) who write software.
The thing you can use today is a local CLI: it reads a Git checkout, analyzes
Go / TypeScript / TSX source, and prints advisory structural signals.

Coach stays on the 0.x line on purpose. It does not contact GitHub or call a
model on this path. File-local analysis is Tree-sitter over committed Git
objects. TypeScript `--project-config` additionally runs a host Node process
and the TypeScript compiler; that is still local, not the platform preview.

## Quick start (TypeScript)

Scan committed `.ts` / `.tsx`. No API key, no model, no merge gate. Uncommitted edits are invisible.

```sh
mise exec github:lousy-agents/coach -- coach codesignal --baseline
```

File-local mode is Tree-sitter over committed Git objects. It reports structural signals (`structure.react_component_orchestration_density`, `coupling.deep_relative_import`, `security.toctou_check_then_act`). It does not know your layers.

Project scanning is a different contract from ESLint boundary plugins, dependency-cruiser, or repo-chat. Those lint the worktree, infer a module graph, or guess architecture from retrieved files. Coach does not guess. You declare `roots`, `layers`, and `forbidden_imports`. Coach evaluates those edges with a host-resolved TypeScript compiler against the **revision you named** and emits versioned `architecture.layer_violation` signals — JSON `schema_version: "2"` that an agent can parse.

```sh
coach codesignal --baseline --check-project --project-language typescript
coach codesignal --baseline --suggest-project-config --project-language typescript
# commit the JSON at the analyzed revision, then:
coach codesignal --baseline --project-config project.json --project-language typescript --format json
```

`--check-project` is readiness, not a scan (exit `0` even with gaps). `--suggest-project-config` is an interactive TTY and writes nothing until you confirm. Project mode needs that committed policy plus a supported compiler and Node 24 or 26. An empty `signals` array is not compliance.

Full flags and schema: [CLI contract](./docs/cli-codesignal.md).

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
| `2` | Usage or invalid `--project-config`, or a TypeScript scan that cannot resolve a supported compiler or host Node (empty stdout except the policy-candidate document on the guided-authoring path; one or more stderr lines). |
| `3` | `--prepare-compiler` only: operational failure resolving the revision or computing readiness — a documented exception to the scan contract's `1`, kept with the flag's own suggestion-mode table. |

### What you should see

A short text report per signal (`path`, `line`, `lifecycle`, `changed`,
`evidence`, why it matters, recommendation), or a quiet report if nothing
matched. `lifecycle` is `introduced` / `existing` / `resolved` / `unknown`
(and `baseline` under `--baseline`).

Diff-mode limits (do not treat these as a clean PR):

- Diff mode runs `git diff --name-status` with `--find-renames` (git's
  default 50% similarity) and `--find-copies-harder` (also considers
  unmodified files as copy sources — more aggressive than git's defaults).
  Rename (`R`) and copy (`C`) **new paths** are analyzed. A pure git
  rename or copy without old-path continuity yields `lifecycle: unknown`
  plus a `continuity_not_determined` path diagnostic — coach does not
  diff a rename against its old path, so a pure move is not classified
  `existing` or `introduced`. Adopters who only watch
  `introduced_signals` shall treat that path as incomplete, not green.
- Statuses the CLI does not analyze (`T`, `U`, `X`, `B`) stay
  `unsupported_change_type`. Those paths increment
  `summary.files_unanalyzed` and `summary.files_with_diagnostics`, and the
  text headline qualifies the all-clear with the numeric unanalyzed count.

Text `line` is 1-based. JSON `location.start_row` is 0-based.

### Lifecycle and counters

JSON `summary` counts every `signals[]` entry in exactly one of
`introduced_signals`, `existing_signals`, `resolved_signals`,
`baseline_signals`, `unknown_signals`. `active_signals` is `len(signals)`
after the include-resolved filter. That relationship holds for both
`--base` and `--baseline`:

- `--base` includes `resolved` in `signals[]` by default, so
  `active_signals` includes them. That total is not "problems present at
  HEAD".
- `--baseline` does not include `resolved`. Signals there are `baseline`.

Problems present at HEAD are `introduced_signals` + `existing_signals` +
`unknown_signals` (and `baseline_signals` under `--baseline`). Do not trust
`active_signals` alone in diff mode.

Added (`A`) files in `--base` mode are `lifecycle: introduced` and count in
`introduced_signals`. `unknown` is residual classification, not "new in this
diff." An unanalyzable merge-base carries a `base_read_failed`,
`base_syntax_errors`, or `base_analysis_failed` diagnostic. A rename or
copy without old-path continuity is also `unknown` and carries
`continuity_not_determined`; that is not a green `introduced_signals`
miss.

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
compiler or host Node exits `2` with empty stdout except the policy-candidate
document on the guided-authoring path, and one or more stderr lines. On a
controlling terminal, a TypeScript scan whose `--project-config` names a
policy that was never committed instead opens a guided-authoring session;
with `--format json` this session's exit-2 output is
a JSON object carrying `"schema_version": "1"` and `roots`, not a CodeSignal
report — the absent `scope`/`signals`/`coverage` keys distinguish it.

Coach's supported Node majors are exactly `24` and `26`. Any other major is the
`node_unsupported` readiness gap. `@lousy-agents/coach-semantics` declares this
as `engines.node: "^24 || ^26"`.

`--prepare-compiler` is a consented, interactive mise TypeScript compiler-setup
session (requires a TTY; refuses otherwise). It prompts before installing. It
also refuses, naming the gap without opening a menu, when the readiness gap is
a runtime boundary (missing or unsupported host Node): Coach has no setup
command for a Node version problem, and the flag shares that gate with the
scan's own compiler-setup offer so the two paths cannot disagree on it. The
flag remains **mise-only**, so it is narrower than the scan's offer in what it
can install; Coach never names it as remediation for a gap only the project
package manager could resolve.

A **scan** (`--baseline`/`--base` with `--project-config`, `--project-language
typescript`, a committed policy, and an unresolved compiler) offers a superset
of that setup interactively when it runs on a controlling terminal: it names
the failing check and what happens after a successful install, then shows a
menu of up to four choices —
`project_package` (run the project's own package-manager install, e.g. `npm
ci --ignore-scripts`), `project_mise`/`global_mise` (run `mise install` for a
project- or globally-pinned supported TypeScript version), and `cancel`. A
choice that would open only to report nothing to do is withheld from the
menu, and a gap with nothing installable never opens a prompt at all. Picking
`project_package` shows a preview (exact executable, arguments, working
directory, expected on-disk changes, network use, and a bounded 5-minute
timeout) before a separate, single-use `confirm`/`install` answer is
required to actually run it — typing anything else, or a second leftover
answer, cancels or is ignored rather than running twice. On cancel, nothing
runs and the scan exits 2 with the original remediation. On failure, the scan
exits 2, reports the failure, and never modifies the repository's tracked
files. On success, Coach reruns the full readiness check and only continues
the same scan through to a report if that rerun is clear; a rerun that still
reports a gap still exits 2 with no report.

Pass `--no-interactive`, or set a non-empty `CI` environment variable, to make
Coach treat the run as unattended even when a real controlling terminal is
attached — for example, a pty-allocating CI runner (`docker run -t`, `ssh -t`)
with nobody at the keyboard. A scan then falls straight through to the same
message-only remediation a piped invocation gets, and
`--suggest-project-config` / `--prepare-compiler` refuse with exit `2` rather
than opening a prompt nobody will answer.

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

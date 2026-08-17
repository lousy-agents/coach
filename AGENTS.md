# AGENTS.md

Canonical project instructions for coding agents working in this repository (Codex, and any other Agent Skills-compatible harness). Claude Code loads this file via the `@./AGENTS.md` import at the top of `CLAUDE.md` — edit guidance here, not there, so the two never drift apart.

## What this is

Experimental AI coach for humans making software with agents. Currently two independent Go packages, plus a TypeScript wrapper for one of them:

- `pkg/semantics` — deterministic structural analysis of raw Go/TypeScript/TSX source bytes (syntax validity, imports, branching metrics, constructor-like patterns) via Tree-sitter. No GitHub dependency.
- `pkg/githubingest` — optional GitHub App-authenticated single-file reader via the GitHub Contents API.
- `js/semantics` — Node/TS bindings for `pkg/semantics` (`@lousy-agents/coach-semantics`, not published to npm), talking newline-delimited JSON to a Go binary over stdin/stdout.

**Dependency rule**: `pkg/semantics` never imports `pkg/githubingest` (or `go-github`/`ghinstallation`), and `pkg/githubingest` never imports `pkg/semantics` back. Keep it that way — this is what lets a consumer that only needs source analysis avoid pulling in a GitHub client.

The `coach` CLI (`cmd/coach`, plumbing in `internal/codesignalcli`) currently exposes one subcommand, `codesignal`, which produces deterministic signal reports for a git diff (`--base`) or a repository baseline (`--baseline`) via the `pkg/semantics` → `pkg/codesignal` pipeline. Product direction lives in `docs/product/prd.md`; system design in `docs/architecture/system-overview.md`.

## Agent Skills (`.agents/skills/`)

- `feature-to-plan` — turn a feature request, PRD, or backlog issue into a structured EARS-format spec.
- `go-testable-design` — guidance for writing/refactoring testable Go (table tests, constructor injection, boundaries, concurrency tests).
- `mutation-hunter` — find TypeScript test-coverage gaps via semantic mutation testing.
- `rugged-evil-tester` — generate adversarial/negative/chaos tests for TypeScript code.
- `product-quality-evaluation` — get a candid, evidence-grounded product/release-readiness assessment via the `product-sme` subagent.
- `skill-reviewer` — lint and review Agent Skills `SKILL.md` files across harnesses.
- `spec-auditor` — adversarially review specs/PRDs/plans before coding.
- `triaging-pr-reviews` — classify and triage PR review comments, including automated reviewer (e.g. Copilot) suggestions.
- `correctness-review` — perform an evidence-backed GitHub pull-request correctness review against its linked issue's acceptance criteria, repository architecture, and downstream specs.
- `issue-refine-loop` — refine an unrefined GitHub issue in place into an implementation-ready epic (problem statement, personas, EARS acceptance criteria, design, tasks, scope boundaries), then decompose it into child issues.

## Custom subagents

Some skills delegate to a named subagent rather than doing the work inline. Each harness defines subagents in its own format:

- `.claude/agents/*.md` — **canonical** Claude Code subagents (YAML frontmatter + markdown body as the system prompt). `task-implementer`/`task-reviewer` back the `implement-issue` command; `product-sme` backs `product-quality-evaluation`. Edit these files when changing agent instructions.
- OpenCode — no separate agent/command body mirrors. `.opencode/plugin/claude-agents.ts` loads `.claude/agents/*.md` and `.claude/commands/*.md` at config time (agents: Claude `tools` → OpenCode `permission`, `maxTurns` → `steps`, `mode: subagent`; commands: frontmatter `description` + body as `template`). Explicit entries in `opencode.json` / `.opencode/agents/` / `.opencode/command(s)/` win over the loader. `.opencode/plugin/implement-issue-gates.ts` enforces implement-issue checkpoints: PR create gated on `mise run ci`; `task` → `task-implementer` rework requires literal `## Reviewer Findings`; `task` → `task-reviewer` results are soft-gated so the first non-empty line is `PASS` or `FINDINGS`. Restart OpenCode after agent, command, or plugin changes.
- `.codex/agents/*.toml` — Codex custom subagents (`name`, `description`, `sandbox_mode`, `developer_instructions`). Codex cannot import Claude markdown, so instruction text is mirrored from `.claude/agents/` and marked with a one-line sync comment — don't build codegen for a two-file mirror.
- `.agents/skills/*/agents/<harness>.yaml` — optional, separate from subagent definitions: a per-harness "interface" declaration (e.g. `display_name`/`default_prompt`) for how a skill surfaces in that harness's UI. Only add one if the harness actually reads it — Claude Code has no such mechanism today.

### Workflows (`.claude/workflows/`) — Claude Code only

Scripts the Claude Code Workflow tool executes to orchestrate subagents deterministically (`.claude/workflows/implement-issue-plan.js` plans an issue). **No other harness has an equivalent**, and the OpenCode loader above mirrors agents and commands only — so a command that delegates work to a workflow must also state that work inline, or it is broken the moment OpenCode loads it. `.opencode/plugin/claude-agents.test.ts` enforces this.

Two invariants apply to anything a workflow does:

- **Hooks do not reach inside.** `SubagentStop` and `PreToolUse` fire on agents the main session spawns and on its own tool calls. An agent spawned inside a workflow reaches neither, so `verify-review-verdict.sh` and `gate-pr-creation.sh` do not run for it — a workflow that took over the implement/review loop or PR creation would look identical and enforce nothing.
- **Nothing else executes these scripts**, so a syntax error or renamed binding surfaces only when a human runs the command. `mise run workflow-test` imports each one under a fake harness; it is wired into `js-ci` rather than `verify` because the `verify` job has no Node and the check would skip silently there.

## Commands

All tasks are defined in `mise.toml`; use `mise run <task>` (mise also pins `go` and `node` versions — CI installs mise so both share one tool-version source of truth).

```sh
mise run ci-gate          # what the PR hook runs: gofmt/vet/style only, no tests (~1s warm)
mise run ci-all           # everything CI proves locally: ci + wasm-build + sidecar-built projectmodel suite
mise run ci-fast          # per-cycle loop check: Go slice + agent-tooling suites, sidecar built first
mise run ci               # gofmt/vet/tidy/style/test/examples/js-ci -- NOT wasm-build, and see ci-fast on skips
mise run gofmt             # gofmt -l . (must be empty)
mise run go-vet
mise run tidy-check        # go mod tidy && diff go.mod/go.sum
mise run test              # go test -race ./...
mise run test-examples     # go test -run Example ./...
mise run test-acceptance-fast # runs the fast, in-process Ginkgo/Gomega acceptance suites (offline, no real credentials)
mise run acceptance-style-check # fails if any *_acceptance_test.go lacks ginkgo/v2 (except allowlist)
mise run test-queue-conformance # runs the queue conformance harness self-test; real Redis Streams/LocalStack SQS legs land with Baseline Task 3a
mise run thinproof-build    # vendors deps + builds the thin offline Compose proof's Docker images (run once, online)
mise run test-acceptance-thin-proof # runs the offline thin Compose proof: fake GitHub -> pkg/githubingest -> CodeSignal, no image pull, no egress
mise run js-ci              # -> js-test -> js-build -> backend-build/js-install
mise run wasm-build         # proves GOOS=js GOARCH=wasm compiles (pure-Go engine, grammar-subset tags)
mise run workflow-test      # .claude/workflows scripts under a fake harness (also their only parse check)
mise run opencode-plugin-test # the loader that mirrors .claude/agents + .claude/commands into OpenCode
```

Single test, Go side:

```sh
go test ./pkg/semantics/... -run TestName -v
```

Single test, agent tooling (Node, from the repo root):

```sh
node --test ".claude/workflows/**/*.test.mjs" --test-name-pattern "cycle"
```

Single test, JS side (from `js/semantics/`):

```sh
npm run build:backend && npm run build && npm run build:test
node --test "dist-test/**/*.test.js"
```

### Parsing engine

`pkg/semantics` parses purely in Go via `github.com/odvcencio/gotreesitter` — no CGO, no C toolchain, and no dual-backend selection required.

## Architecture: `pkg/semantics`

Pipeline (`analyzer.go`): `AnalyzeBytes` = validate -> parse -> syntax-check -> extract imports -> compute metrics/findings -> `Result`.

- **Backend seam** (`internal/engine/engine.go`): a deliberately narrow interface (`Node`, `Tree`, `Parser`, `Query`, `QueryCursor`, `Language`) exposing only the Tree-sitter operations the package actually uses (no `NamedChild`, no `TreeCursor`, no query predicates, no incremental parsing). This package is `internal`, so it's only importable from within `pkg/semantics`. There is exactly one implementation: `internal/engine/gotreesitter.go` (pure-Go, always compiled, no build tag).
- **Registry selection** (`language.go`): `languageSpec` bundles a backend-bound `engine.Language` handle with language-specific `extractImports`/`computeFeatures` functions. `languageRegistry` (`map[Language]languageSpec`) is defined unconditionally in `language.go` — no build tags, no per-backend variants. Adding a language means extending the registry plus its own `extract*Imports`/`compute*Features` pair (mirroring the Go or TS implementations), not touching `parser.go`/`analyzer.go`.
- **Concurrency**: `*Analyzer` holds no backend resources between calls — every `AnalyzeBytes` call creates and closes its own `Parser`/`Tree`/`Query`/`QueryCursor` — so a single `*Analyzer` is safe for concurrent use regardless of engine backend.
- **Error contract**: syntax errors return a partial `*Result` (`ParseStatus == "syntax_errors"`) *and* a non-nil error satisfying `errors.Is(err, ErrSyntax)` (use `errors.As` for `*SyntaxError.Issues`). Other sentinels: `ErrEmptyContent`, `ErrUnsupportedLanguage`, `ErrFileTooLarge`, `ErrBinaryContent`, `ErrParseFailure`.
- **JSON stability**: `Result` and nested types use frozen `snake_case` JSON field names, locked by a golden-file test (`result_test.go`). Field names and error identities (`Err*` sentinels, `*SyntaxError`) are treated as stable pre-1.0 API surface; other surface may still change.
- `internal/jsbridge` (repo-root `internal/`, not under `pkg/semantics`) implements the newline-delimited JSON protocol consumed by `cmd/semantics-json` (the stdio backend binary `js/semantics` shells out to) and mirrored by `js/semantics/src/protocol.ts`. A parity test suite (`js/semantics/test/parity.test.ts`) replays shared fixtures through both the Go API and the JS package to keep them byte-identical.

## Architecture: `js/semantics`

TypeScript package with a `Backend` seam (`src/backend.ts`) abstracting the transport; `src/backend-cli.ts`/`backend-default.ts` spawn the compiled `coach-semantics-json` Go binary and speak the jsbridge protocol over stdio. A WASM backend (`backend-wasm.ts`) is not yet wired up even though `pkg/semantics` now builds for `GOOS=js GOARCH=wasm` (see `wasm-build`/`cmd/semantics-wasm-smoke`) — swapping transports is meant to stay behind the `Backend` seam without changing the public API. `npm install`/`prepare` builds the Go backend binary and the TS package, so Go is required even for JS-only work.

## Architecture: `pkg/githubingest`

Single entry point `ReadFile`, authenticated via a GitHub App installation (`ghinstallation` + `go-github`). Each call issues two Contents API requests: the file fetch, plus a listing of the parent directory to detect in-repo symlinks GitHub's Contents API would otherwise silently resolve as a plain file (`reader.go`'s `rejectIfPathIsSymlink`). That listing is capped at GitHub's 1,000-entries-per-directory limit with no truncation signal, so a symlink in a very large directory can go undetected — an accepted, documented limitation for v1. Error sentinels: `ErrNotFound`, `ErrAuth`, `ErrUnsupportedContent`, `ErrTooLarge` (>1 MiB), `ErrEmptyContent`.

## Validation

### Validation Suite (mandatory before commit)

These are the exact checks CI runs in `.github/workflows/ci.yml`, so a clean local run here means CI passes:

```sh
gofmt -l .                      # must print nothing
go vet ./...
go mod tidy && git diff --exit-code go.mod go.sum
go run ./cmd/acceptance-style-guard   # or: mise run acceptance-style-check
go test -race ./...
go test -run Example ./...
mise run js-ci
mise run wasm-build
```

`mise run ci` runs the Go-side checks **and `js-ci`** (`mise.toml` lists `{ task = "js-ci" }` in the `ci` task). It does **not** run `wasm-build`.

Two gaps `mise run ci` alone does not close:

- `wasm-build` is in no task's closure, so a `GOOS=js GOARCH=wasm` break can pass `ci`.
- `ci` runs `test` **before** `js-ci`, so `pkg/projectmodel`'s TypeScript sidecar acceptance suite has no built sidecar when it executes and **skips silently** — see the comment at `.github/workflows/ci.yml:52-56`. A green `ci` does not mean that suite ran.

Use **`mise run ci-fast`** inside an implement/review loop — sidecar-first ordering, without the wasm and full-CI legs. **`mise run ci-all`** chains `ci`, `wasm-build`, and the sidecar-built `pkg/projectmodel` suite; run it when you want the whole thing locally, but it is no longer the pre-PR gate.

**The exhaustive gate is GitHub Actions plus branch protection, not a local run.** `gate-pr-creation.sh` used to run `ci-all` — chosen when a warm `ci-all` was projected at ~41s. Measured on a CCR container it is ~910s, against that hook's own 900s timeout, while CI proves a strict superset (the same atomic tasks as parallel required jobs, **plus `platform-smoke`**) in ~426s wall clock on compute that is not the session's. The hook now runs `ci-gate` (~1s warm) and the clean-worktree check, which is the one thing CI structurally cannot do: CI validates the *pushed commit* and cannot see a working tree that differs from it.

`ci-gate` deliberately runs no tests and **no `tidy-check`** — the latter rewrites `go.mod`/`go.sum` in place, which would dirty the very tree the hook just certified clean. CI runs both. This split is only safe while those jobs are **required checks** on the base branch; if branch protection is removed, nothing gates a red merge.

`ci-all` deliberately excludes `test-acceptance-fast` (its ambient-credential preflight cannot pass where `GITHUB_TOKEN`/`GH_TOKEN` or `~/.aws/config` are present, and `test` already runs every acceptance suite unfiltered) and `platform-smoke` (Docker + live services).

### Acceptance-test-first (required policy)

Every new feature and every bug fix **must begin with a failing acceptance test** before production implementation changes are made.

- For a feature, write an acceptance test that demonstrates the requested externally observable behavior is absent, run it, and confirm that it fails. Only then implement the feature until that same test passes.
- For a bug fix, write an acceptance test that reproduces the reported incorrect behavior, run it, and confirm that it fails for the bug. Only then implement the fix until that same test passes.
- The test must exercise the relevant public behavior at the most meaningful available boundary; a unit test alone is not an acceptance test unless that unit is itself the public contract.
- Do not treat an unrun test, a test written after implementation, or a test that already passes as satisfying this policy. If the required test cannot be made to fail before implementation, stop and resolve the discrepancy with the requester rather than proceeding.

**Go acceptance form (mandatory):**

- Use Ginkgo v2 + Gomega (`github.com/onsi/ginkgo/v2`, `github.com/onsi/gomega`).
- Spec style: `Describe` / `When` / `It` (and `DescribeTable` when useful) that read as EARS/acceptance-criteria statements.
- Layout: `*_acceptance_test.go` plus `acceptance_suite_test.go` with a `TestXxxAcceptance` entrypoint so `mise run test-acceptance-fast` (`go test … -run Acceptance`) picks them up.
- Reference examples: `cmd/coach/baseline_acceptance_test.go`, `pkg/githubingest/acceptance_test.go`.
- Plain unit tests (`*_test.go` without the acceptance suite role) may use stdlib `testing` + table tests; that is **not** a substitute for acceptance coverage of new features/bug fixes.
- Exception: thin stdlib `Test*Acceptance` **wrappers** that only call a shared harness (e.g. `internal/acceptanceharness/queueconformance/acceptance_test.go`) are allowed when they are not the behavioral specs themselves.
- Mechanical guard (when present): `mise run acceptance-style-check`.

**False-green rule:** a test only counts if it exercises the intended branch/failure mode. Shared clocks/fakes that make a different path produce the same status/outcome are invalid (e.g. advancing time so a "denylisted" case actually fails on expiry).

For delegated work, the `task-implementer`/`task-reviewer` subagent pair (`.claude/agents/`) operationalizes this policy step-by-step: the implementer must write and fail a Ginkgo acceptance test before implementing, and the reviewer gates on red-then-green evidence plus the form/false-green rules above. Subagent prompts must not relax AGENTS.md — do not tell implementers that stdlib table tests substitute for Ginkgo acceptance tests; copy conventions from here, don't invent weaker ones.

### Outbound HTTP (required policy)

Production defaults for upstream HTTP clients must use a finite `Timeout`. Do not use bare `http.DefaultClient` for request paths that can hang.

### Store/dependency fail-closed (required policy)

When a required store/dependency errors (not a clean miss/not-found), protected/auth paths return **503** with the stable JSON error envelope — fail closed. Do not skip the check or treat store errors as soft 500 inconsistently across analogous paths.

### Go comments (required policy)

Default is **no comment** unless it helps a human or coding agent use or change the code correctly. This policy applies to Go only.

**Keep / write** when the comment encodes a non-local contract:

- Exported API behavior callers cannot infer from the name (errors, auth, zero value, concurrency, special cases)
- Intentional simplifications and external wire quirks (e.g. go-github response shapes, GitHub API limits)
- Invariants that tests or agents will otherwise “fix” wrongly (race guards, auth-mode recording, false-green traps)

**Form (godoc):**

- Doc comments sit immediately above the declaration; complete sentences; start with the symbol name (`Package foo…`, `ClassifyToken reports…`)
- Prefer short paragraphs; use end-of-line comments for map keys / enum values when enough
- Attach notes to a declaration (no orphan `// NOTE` blocks)
- Follow [Go doc comments](https://go.dev/doc/comment); do not put epic/issue narrative in code — that belongs in `docs/`, the PR, or the commit message

**Delete / never add:**

- Restating the identifier or the next line of code
- Step-by-step narration of obvious control flow
- Long essays duplicated across handlers (factor one shared helper/doc or package comment)
- Test comments that only paraphrase `It("…")` / subtest names — prefer structure and names (see `go-testable-design`); keep only subtle assertion traps

**Unexported** symbols: comment only for the contracts/traps above, not routine helpers.

### Verification

Passing checks proves nothing broke; it doesn't prove new behavior is correct. For a `pkg/semantics` extraction/metric change, add or extend a case in the relevant `*_test.go` (`features_test.go`, `ts_features_test.go`, `query_test.go`, …) with a concrete before/after `Result`, not just a "does it run" assertion. For `js/semantics` changes, extend `parity.test.ts` so the Go and JS outputs are checked byte-identical, not just independently plausible.

### Feedback Loop

After a failing check, fix and rerun that specific command rather than the whole suite — `go test -race ./... -run TestName` narrows to one test. Don't move on to the next validation step until the current one is clean.

### What `/implement-issue` guarantees, and what it does not

The command was designed to move orchestration out of the model's conversational memory and into a committed script, on the reasoning that memory is the fragile part. **That is only partly what shipped**, and the difference matters when reading a run's output.

**Deterministic, held by code or hooks:**

- **Planning.** `.claude/workflows/implement-issue-plan.js` produces the task DAG. Its arg validation, null guards, cycle and dangling-reference checks are JS, covered by `mise run workflow-test`, and mutation-tested.
- **The gates.** A reviewer's verdict shape, verbatim findings relay, a clean worktree, and a green `ci-gate` are enforced by hooks that run outside the model's control — fail-closed on a git error, a second registration so a new reviewer cannot escape, and a re-entry guard so a blocked reviewer is not retried forever. The exhaustive suite moved to required CI checks, which is a *stronger* placement: a hook runs inside the environment being gated, and a session where it never registered is indistinguishable from one where it did (hence step 0). A required check runs where no agent can reach it.
- **A ceiling on reviewer cycles** (`enforce-cycle-ceiling.sh`), which is a blunt total-invocation backstop, not the per-task rule.

**Prose, held by the orchestrator following instructions:**

- The per-task cycle cap and the no-progress rule
- Dependency ordering — that a task waits for its `dependsOn`
- Which task receives a validation failure, and the invalidation rule after rework
- That the `conventions` string reaches implementers unweakened

Those are instructions in `.claude/commands/implement-issue.md`. The specs in `internal/agentworkflows/` assert that **the instruction is present and says the right thing** — they cannot assert that a run obeyed it. A paragraph cannot be fault-injected or mutation-tested, which are the two methods that have actually found defects in this repository.

Treat a clean run as evidence the gates held, not as evidence the loop was bounded. When a run's behavior matters, read the hook trace (see `.claude/hooks/lib/trace.sh`) rather than inferring from the absence of a complaint.

**And a clean run is only evidence the gates held if they were registered at all.** Claude Code binds `.claude/` — hooks, agents, and workflows alike — to the session's project directory at session start. A repository cloned into a session whose project directory is elsewhere never registers any of them, and attaching it mid-session reloads CLAUDE.md and skills but not hooks, agents, or workflows. A run in that state is indistinguishable from a gated one from the inside: the agents answer, the reviews return verdicts, the PR opens. This is not hypothetical — a proving run reached exactly that state, and caught it by reasoning about the environment rather than by any mechanism.

Step 0 of the command is the mechanism: it provokes a denial from `verify-context-relay.sh` and stops with `environment-failure` if the call is allowed, or if `task-implementer` does not resolve. Being denied is the pass. Inspecting the hook files proves nothing, because they are on disk either way — which is why the check is a probe and not a file existence test. It is prose like the rules above, so it constrains an orchestrator that follows it and nothing else; what it removes is the *ambiguity*, not the possibility of skipping the step.

## Pull requests

Before `gh pr create` / `create_pull_request`, read and fill every section of [`.github/PULL_REQUEST_TEMPLATE.md`](.github/PULL_REQUEST_TEMPLATE.md). That file is the PR contract for coding agents: linked issue, single concern, acceptance-criteria → evidence table, red-then-green acceptance proof, and the validation commands you actually ran. Do not open a PR with blank sections or placeholder text.

### Commit types (required policy)

Conventional Commits, chosen by **who the change is for** — GoReleaser builds release notes from commit subjects, so the type decides whether a change is described to `coach` users as part of the CLI.

- `feat` / `fix` — behavior a `coach` user can invoke: the CLI, `pkg/semantics`, `pkg/githubingest`, `js/semantics`. These reach the release notes.
- `chore` / `ci` / `build` / `refactor` / `style` / `test` / `docs` — everything else, including **agent tooling**: `.claude/` and `.agents/` definitions, hooks, subagent and workflow files, `mise.toml` tasks, and CI workflows. These are filtered out of the release notes by `.goreleaser.yaml`.

A PR title follows the same rule as its commits. Agent tooling changes the way this repository is *built*, not what it *does*, so labelling it `feat` publishes a feature that does not exist.

## CI shape (`.github/workflows/ci.yml`)

Four independent jobs:

- `verify` — gofmt/vet/tidy/acceptance-style-check/test/examples. Deliberately has **no** Node on PATH, so `pkg/projectmodel`'s TS sidecar suite skips here and contributes no signal.
- `js-verify` — `mise run js-ci`, **plus** `js-install`, `project-sidecar-build`, and `go test -race ./pkg/projectmodel/... -run Acceptance`. Those extra steps are what actually exercise the sidecar suite; `js-ci` alone does not.
- `wasm-build` — proves the `GOOS=js GOARCH=wasm` grammar-subset build compiles under the sole pure-Go engine.
- `platform-smoke` — Docker-based `platform-up` / `platform-smoke` / `platform-down`.

`mise run ci-all` mirrors the first three locally. `platform-smoke` has no local mirror — it is one reason CI is a strict superset of any local run.

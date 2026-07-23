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

## Custom subagents

Some skills delegate to a named subagent rather than doing the work inline. Each harness defines subagents in its own format:

- `.claude/agents/*.md` — **canonical** Claude Code subagents (YAML frontmatter + markdown body as the system prompt). `task-implementer`/`task-reviewer` back the `implement-issue` command; `product-sme` backs `product-quality-evaluation`. Edit these files when changing agent instructions.
- OpenCode — no separate agent/command body mirrors. `.opencode/plugin/claude-agents.ts` loads `.claude/agents/*.md` and `.claude/commands/*.md` at config time (agents: Claude `tools` → OpenCode `permission`, `maxTurns` → `steps`, `mode: subagent`; commands: frontmatter `description` + body as `template`). Explicit entries in `opencode.json` / `.opencode/agents/` / `.opencode/command(s)/` win over the loader. `.opencode/plugin/implement-issue-gates.ts` enforces implement-issue checkpoints: PR create gated on `mise run ci`; `task` → `task-implementer` rework requires literal `## Reviewer Findings`; `task` → `task-reviewer` results are soft-gated so the first non-empty line is `PASS` or `FINDINGS`. Restart OpenCode after agent, command, or plugin changes.
- `.codex/agents/*.toml` — Codex custom subagents (`name`, `description`, `sandbox_mode`, `developer_instructions`). Codex cannot import Claude markdown, so instruction text is mirrored from `.claude/agents/` and marked with a one-line sync comment — don't build codegen for a two-file mirror.
- `.agents/skills/*/agents/<harness>.yaml` — optional, separate from subagent definitions: a per-harness "interface" declaration (e.g. `display_name`/`default_prompt`) for how a skill surfaces in that harness's UI. Only add one if the harness actually reads it — Claude Code has no such mechanism today.

## Commands

All tasks are defined in `mise.toml`; use `mise run <task>` (mise also pins `go` and `node` versions — CI installs mise so both share one tool-version source of truth).

```sh
mise run ci               # everything CI runs, in order
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
```

Single test, Go side:

```sh
go test ./pkg/semantics/... -run TestName -v
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

`mise run ci` runs all of the Go-side checks (not `js-ci`/`wasm-build`, which are separate CI jobs — run those explicitly when touching `js/semantics` or WASM build tags).

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

## CI shape (`.github/workflows/ci.yml`)

Three independent jobs: `verify` (gofmt/vet/tidy/test/examples), `js-verify` (`mise run js-ci`), `wasm-build` (proves the `GOOS=js GOARCH=wasm` grammar-subset build compiles under the sole pure-Go engine).

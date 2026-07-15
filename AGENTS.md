# AGENTS.md

Canonical project instructions for coding agents working in this repository (Codex, and any other Agent Skills-compatible harness). Claude Code loads this file via the `@./AGENTS.md` import at the top of `CLAUDE.md` — edit guidance here, not there, so the two never drift apart.

## What this is

Experimental AI coach for humans making software with agents. Currently two independent Go packages, plus a TypeScript wrapper for one of them:

- `pkg/semantics` — deterministic structural analysis of raw Go/TypeScript/TSX source bytes (syntax validity, imports, branching metrics, constructor-like patterns) via Tree-sitter. No GitHub dependency.
- `pkg/githubingest` — optional GitHub App-authenticated single-file reader via the GitHub Contents API.
- `js/semantics` — Node/TS bindings for `pkg/semantics` (`@lousy-agents/coach-semantics`, not published to npm), talking newline-delimited JSON to a Go binary over stdin/stdout.

**Dependency rule**: `pkg/semantics` never imports `pkg/githubingest` (or `go-github`/`ghinstallation`), and `pkg/githubingest` never imports `pkg/semantics` back. Keep it that way — this is what lets a consumer that only needs source analysis avoid pulling in a GitHub client.

There is no `coach` CLI yet.

## Agent Skills (`.agents/skills/`)

- `feature-to-plan` — turn a feature request, PRD, or backlog issue into a structured EARS-format spec.
- `go-testable-design` — guidance for writing/refactoring testable Go (table tests, constructor injection, boundaries, concurrency tests).
- `mutation-hunter` — find TypeScript test-coverage gaps via semantic mutation testing.
- `rugged-evil-tester` — generate adversarial/negative/chaos tests for TypeScript code.
- `skill-reviewer` — lint and review Agent Skills `SKILL.md` files across harnesses.
- `spec-auditor` — adversarially review specs/PRDs/plans before coding.
- `triaging-pr-reviews` — classify and triage PR review comments, including automated reviewer (e.g. Copilot) suggestions.

## Commands

All tasks are defined in `mise.toml`; use `mise run <task>` (mise also pins `go` and `node` versions — CI installs mise so both share one tool-version source of truth).

```sh
mise run ci               # everything CI runs, in order
mise run gofmt             # gofmt -l . (must be empty)
mise run go-vet
mise run tidy-check        # go mod tidy && diff go.mod/go.sum
mise run test              # go test -race ./...
mise run test-examples     # go test -run Example ./...
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
go test -race ./...
go test -run Example ./...
mise run js-ci
mise run wasm-build
```

`mise run ci` runs all of the Go-side checks (not `js-ci`/`wasm-build`, which are separate CI jobs — run those explicitly when touching `js/semantics` or WASM build tags).

### Verification

Passing checks proves nothing broke; it doesn't prove new behavior is correct. For a `pkg/semantics` extraction/metric change, add or extend a case in the relevant `*_test.go` (`features_test.go`, `ts_features_test.go`, `query_test.go`, …) with a concrete before/after `Result`, not just a "does it run" assertion. For `js/semantics` changes, extend `parity.test.ts` so the Go and JS outputs are checked byte-identical, not just independently plausible.

### Feedback Loop

After a failing check, fix and rerun that specific command rather than the whole suite — `go test -race ./... -run TestName` narrows to one test. Don't move on to the next validation step until the current one is clean.

## CI shape (`.github/workflows/ci.yml`)

Three independent jobs: `verify` (gofmt/vet/tidy/test/examples), `js-verify` (`mise run js-ci`), `wasm-build` (proves the `GOOS=js GOARCH=wasm` grammar-subset build compiles under the sole pure-Go engine).

# AGENTS.md

Canonical project instructions for coding agents working in this repository (Codex, and any other Agent Skills-compatible harness). Claude Code loads this file via the `@./AGENTS.md` import at the top of `CLAUDE.md` — edit guidance here, not there, so the two never drift apart.

Obligation in this file is carried by three words and nothing else. `shall` marks a requirement or invariant; `should` marks a recommendation you may depart from with a reason; `will` states a fact or an external condition. Bare imperatives ("run", "do not") are requirements too. Emphasis is never a priority marker, so do not read bold as urgency or add it to create urgency.

## What this is

Experimental AI coach for humans making software with agents. Two independent Go packages, plus a TypeScript wrapper for one of them:

- `pkg/semantics` — deterministic structural analysis of raw Go/TypeScript/TSX source bytes (syntax validity, imports, branching metrics, constructor-like patterns) via Tree-sitter. No GitHub dependency.
- `pkg/githubingest` — optional GitHub App-authenticated single-file reader via the GitHub Contents API.
- `js/semantics` — Node/TS bindings for `pkg/semantics` (`@lousy-agents/coach-semantics`, not published to npm), talking newline-delimited JSON to a Go binary over stdin/stdout.

**Dependency rule**: `pkg/semantics` shall not import `pkg/githubingest` (or `go-github`/`ghinstallation`), and `pkg/githubingest` shall not import `pkg/semantics` back. Keep it that way — this is what lets a consumer that only needs source analysis avoid pulling in a GitHub client.

The `coach` CLI (`cmd/coach`, plumbing in `internal/codesignalcli`) exposes one subcommand, `codesignal`, which produces deterministic signal reports for a git diff (`--base`) or a repository baseline (`--baseline`) via the `pkg/semantics` → `pkg/codesignal` pipeline. Product direction lives in `docs/product/prd.md`; system design in `docs/architecture/system-overview.md`.

**Living product evaluation**: `docs/product/evaluations/codesignal-pilot-readiness.html` is the current leave-pilot evidence, not a historical snapshot. Closing a #282 child or merging user-facing `coach codesignal` behavior means updating it: re-run the affected claim against HEAD, move closed gaps to the archive (do not delete them), restamp date / HEAD / `#282 · N / 24`, and re-rank only after a run. GitHub `CLOSED` is not sufficient evidence.

## Agent Skills (`.agents/skills/`)

Most are sourced from `lousy-agents/skills` and pinned by `skills-lock.json`: change those upstream and re-run `npx skills add`, because edits made here are overwritten. `correctness-review`, `product-quality-evaluation`, and `optimize-prompt-loop` are not in the lockfile and are edited here directly. Check `skills-lock.json` before editing a skill, so a local edit is not silently reverted by the next sync.

- `feature-to-plan` — turn a feature request, PRD, or backlog issue into a structured EARS-format spec.
- `go-testable-design` — guidance for writing/refactoring testable Go (table tests, constructor injection, boundaries, concurrency tests).
- `mutation-hunter` — find TypeScript test-coverage gaps via semantic mutation testing.
- `rugged-evil-tester` — generate adversarial/negative/chaos tests for TypeScript code.
- `product-quality-evaluation` — get a candid, evidence-grounded product/release-readiness assessment via the `product-sme` subagent.
- `designing-for-intent` — review a UX/onboarding/consent artifact for intent map, delegation boundary, and agency risks before implementation. Complementary to the `ux-advocate` roster seat (customer-reading of copy/sequencing), not a wrapper around it.
- `skill-reviewer` — lint and review Agent Skills `SKILL.md` files across harnesses.
- `spec-auditor` — adversarially review specs/PRDs/plans before coding.
- `triaging-pr-reviews` — classify and triage PR review comments, including automated reviewer (e.g. Copilot) suggestions.
- `correctness-review` — perform an evidence-backed GitHub pull-request correctness review against its linked issue's acceptance criteria, repository architecture, and downstream specs.
- `issue-refine-loop` — refine an unrefined GitHub issue in place into an implementation-ready epic (problem statement, personas, EARS acceptance criteria, design, tasks, scope boundaries), then decompose it into child issues.
- `plan-to-graph` — convert an approved spec or epic issue into a GitHub issue dependency graph with native sub-issues and blocking relationships.
- `curate-release` — rewrite the commits on a PR's head branch so the generated changelog and release notes read as a coherent story.
- `optimize-prompt-loop` — turn a fuzzy or overly broad request into a concise, reusable task prompt tuned to the current model and harness.

## Custom subagents

Some skills delegate to a named subagent rather than doing the work inline. Route product/roadmap questions to `product-sme`, experience/journey/agency questions to `ux-advocate`, architecture/design questions to `system-design-expert`, and — only once a spec has actually been drafted — spec-audit questions to `spec-review-agent`. Where a spec needs a convergence loop that edits it rather than a read-only audit, use `epic-reviewer`, which drives those seats to agreement and writes the result back. The orchestrator delegates to these seats rather than impersonating them, because a seat's value is the independent read, which is lost when one agent plays every part.

Each harness defines subagents in its own format:

- `.claude/agents/*.md` — canonical Claude Code subagents (YAML frontmatter plus a markdown body used as the system prompt). `task-implementer`/`task-reviewer`, and `workflow-integration-reviewer` for the integrated diff, back the `implement-issue` command; `product-sme` backs `product-quality-evaluation`; `ux-advocate` is a roster peer for customer-facing journey reading and does not back `designing-for-intent`. Edit these files when changing agent instructions. Exception: `ux-advocate` is a Coach host-binding wrapper over the lockfile-sourced `designing-for-intent` agent (host peer map, then the published charter inlined). Do not fork the charter here — change `lousy-agents/skills`, re-run `npx skills add`, and refresh the inlined body from `.agents/skills/designing-for-intent/agents/ux-advocate.md`. Edit only the host peer map in this wrapper.
- OpenCode — no separate agent or command body mirrors. `.opencode/plugin/claude-agents.ts` loads `.claude/agents/*.md` and `.claude/commands/*.md` at config time (agents: Claude `tools` → OpenCode `permission`, `maxTurns` → `steps`, `mode: subagent`; commands: frontmatter `description` plus body as `template`). Explicit entries in `opencode.json` / `.opencode/agents/` / `.opencode/command(s)/` win over the loader. `.opencode/plugin/implement-issue-gates.ts` mirrors the two review-loop hooks: `task` → `task-implementer` rework requires the literal `## Reviewer Findings` heading, and `task` → `task-reviewer` results are soft-gated so the first non-empty line is `PASS` or `FINDINGS`. Restart OpenCode after agent, command, or plugin changes.
- `.codex/agents/*.toml` — Codex custom subagents (`name`, `description`, `sandbox_mode`, `developer_instructions`). Codex cannot import Claude markdown, so instruction text is mirrored from `.claude/agents/` and marked with a one-line sync comment — do not build codegen for a two-file mirror. Only the five roster seats are mirrored. `task-implementer`, `task-reviewer`, and `workflow-integration-reviewer` are deliberately absent, because they exist only to back `/implement-issue` and Codex has no command surface for them to serve; do not add TOMLs for them. `ux-advocate.toml` mirrors the Coach host-binding wrapper (refresh the inlined charter after a skills-lock update).
- `.gemini/agents/*.md` — Gemini/antigravity subagents (`name`, `description`, `kind`, `tools`, `model`). `doc-expert` is the only seat here and has no Claude or Codex counterpart, so the mirror rule above does not apply to it; nothing in the repository references it, so treat it as gemini-only rather than as a mirror that has drifted.
- `.agents/skills/*/agents/<harness>.yaml` — optional, and separate from subagent definitions: a per-harness interface declaration (e.g. `display_name`/`default_prompt`) for how a skill surfaces in that harness's UI. Only add one where the harness actually reads it — Claude Code has no such mechanism today.

### Workflows (`.claude/workflows/`) — Claude Code only

Scripts the Claude Code Workflow tool executes to orchestrate subagents deterministically (`.claude/workflows/implement-issue-plan.js` plans an issue). No other harness has an equivalent, and the OpenCode loader above mirrors agents and commands only — so a command that delegates work to a workflow shall also state that work inline, or it is broken the moment OpenCode loads it. `.opencode/plugin/claude-agents.test.ts` enforces this.

Two invariants apply to anything a workflow does:

- **Hooks do not reach inside.** `SubagentStop` and `PreToolUse` fire on agents the main session spawns and on its own tool calls. An agent spawned inside a workflow reaches neither, so `verify-review-verdict.sh` and `verify-context-relay.sh` do not run for it — a workflow that took over the implement/review loop would look identical while the review loop silently lost its fidelity checks.
- **Nothing else executes these scripts**, so a syntax error or renamed binding surfaces only when a human runs the command. `mise run workflow-test` imports each one under a fake harness; it is wired into `js-ci` rather than `verify` because the `verify` job has no Node and the check would skip silently there.

## Commands

All tasks are defined in `mise.toml`; run them with `mise run <task>`, and list them with `mise tasks` (each carries its own description). mise also pins `go` and `node`, and CI installs mise, so both share one tool-version source of truth. The tasks worth knowing before you read that list:

| Task | Use it for |
| --- | --- |
| `ci-gate` | fast local smoke (gofmt/vet/style, no tests, ~1s warm) |
| `ci-fast` | the implement/review loop's per-cycle check: Go slice plus agent-tooling suites, sidecar built first. Narrower than CI, but not quick: `cmd/coach` alone runs minutes |
| `ci` | `ci-go` + `js-ci`. No `wasm-build`, and it builds the sidecar only after `test` has already run |
| `ci-all` | everything CI proves except `platform-smoke` |
| `test` | `go test -race ./...` |
| `test-acceptance-fast` | the fast in-process Ginkgo/Gomega suites (offline, no real credentials) |

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

## Architecture

Full design lives in `docs/architecture/system-overview.md`. What follows is only what a reader cannot infer from the code.

### `pkg/semantics`

`pkg/semantics` parses purely in Go via `github.com/odvcencio/gotreesitter` — no CGO, no C toolchain, and no dual-backend selection. Pipeline (`analyzer.go`): `AnalyzeBytes` = validate → parse → syntax-check → extract imports → compute metrics/findings → `Result`.

- **Backend seam** (`internal/engine/engine.go`): a deliberately narrow interface (`Node`, `Tree`, `Parser`, `Query`, `QueryCursor`, `Language`) exposing only the Tree-sitter operations the package actually uses (no `NamedChild`, no `TreeCursor`, no query predicates, no incremental parsing). The package is `internal`, so it is importable only from within `pkg/semantics`. There is exactly one implementation: `internal/engine/gotreesitter.go` (pure-Go, always compiled, no build tag).
- **Registry selection** (`language.go`): `languageSpec` bundles a backend-bound `engine.Language` handle with language-specific `extractImports`/`computeFeatures` functions. `languageRegistry` (`map[Language]languageSpec`) is defined unconditionally — no build tags, no per-backend variants. Adding a language means extending the registry plus its own `extract*Imports`/`compute*Features` pair (mirroring the Go or TS implementations), not touching `parser.go`/`analyzer.go`.
- **Concurrency**: an `*Analyzer` holds no backend resources between calls — every `AnalyzeBytes` call creates and closes its own `Parser`/`Tree`/`Query`/`QueryCursor` — so a single `*Analyzer` is safe for concurrent use.
- **Error contract**: syntax errors return a partial `*Result` (`ParseStatus == "syntax_errors"`) *and* a non-nil error satisfying `errors.Is(err, ErrSyntax)` (use `errors.As` for `*SyntaxError.Issues`). Other sentinels: `ErrEmptyContent`, `ErrUnsupportedLanguage`, `ErrFileTooLarge`, `ErrBinaryContent`, `ErrParseFailure`.
- **JSON stability**: `Result` and nested types use frozen `snake_case` JSON field names, locked by a golden-file test (`result_test.go`). Field names and error identities (`Err*` sentinels, `*SyntaxError`) are treated as stable pre-1.0 API surface; other surface may still change.

`internal/jsbridge` (repo-root `internal/`, not under `pkg/semantics`) implements the newline-delimited JSON protocol consumed by `cmd/semantics-json` (the stdio backend binary `js/semantics` shells out to) and mirrored by `js/semantics/src/protocol.ts`. `js/semantics/test/parity.test.ts` replays shared fixtures through both the Go API and the JS package to keep them byte-identical.

### `js/semantics`

A `Backend` seam (`src/backend.ts`) abstracts the transport; `src/backend-cli.ts`/`backend-default.ts` spawn the compiled `coach-semantics-json` Go binary and speak the jsbridge protocol over stdio. A WASM backend (`backend-wasm.ts`) is not yet wired up even though `pkg/semantics` now builds for `GOOS=js GOARCH=wasm` (see `wasm-build`/`cmd/semantics-wasm-smoke`) — swapping transports is meant to stay behind the `Backend` seam without changing the public API. `npm install`/`prepare` builds the Go backend binary and the TS package, so Go is required even for JS-only work.

### `pkg/githubingest`

Single entry point `ReadFile`, authenticated via a GitHub App installation (`ghinstallation` + `go-github`). Each call issues two Contents API requests: the file fetch, plus a listing of the parent directory to detect in-repo symlinks GitHub's Contents API would otherwise silently resolve as a plain file (`reader.go`'s `rejectIfPathIsSymlink`). That listing is capped at GitHub's 1,000-entries-per-directory limit with no truncation signal, so a symlink in a very large directory can go undetected — an accepted, documented limitation for v1. Error sentinels: `ErrNotFound`, `ErrAuth`, `ErrUnsupportedContent`, `ErrTooLarge` (>1 MiB), `ErrEmptyContent`.

## Validation

### Validation Suite (mandatory before commit)

Run these before commit. Each is an atomic `mise run <task>` step CI also runs in `.github/workflows/ci.yml`; CI additionally runs `platform-up`/`platform-smoke`/`platform-down`, which are left out here because they need Docker and live services:

```sh
mise run gofmt
mise run go-vet
mise run tidy-check
mise run acceptance-style-check
mise run test
mise run test-examples
mise run js-ci
mise run projectmodel-sidecar-acceptance
mise run ts-project-backend-acceptance
mise run wasm-build
```

Run `ci-fast` inside an implement/review loop and `ci-all` when you want the broadest local run. Neither is the pre-PR gate. `ci-all` reaches `js-install` through its sidecar build, so the sidecar and TypeScript-backend specs do run inside `test` — but `test` carries no `-ginkgo.fail-on-empty`, so if their compiler preconditions fail they skip and `ci-all` stays green. The `projectmodel-sidecar` and `ts-project-backend` leaves exist to make that skip a failure; `platform-smoke` is the only leaf with no local coverage at all.

The exhaustive gate is GitHub Actions plus branch protection, not a local run. Since the `status` aggregator became a required check, a red tree cannot merge whatever any local check decides — so nothing gates PR creation locally. Commit and push everything before opening a PR so its evidence describes the tree you pushed; that is a discipline, not a mechanism. This arrangement is only safe while `status` is a required check on the base branch; if branch protection is removed, nothing gates a red merge.

Task closure, the gaps between `ci` and `ci-all`, the measured wall times, and the GHA job graph are in [`docs/development/validation-and-ci.md`](docs/development/validation-and-ci.md). Read it before concluding that a green local run means CI will pass.

### Acceptance-test-first (required policy)

Every new feature and every bug fix shall begin with a failing acceptance test, written before production implementation changes are made.

- For a feature, write an acceptance test that demonstrates the requested externally observable behavior is absent, run it, and confirm that it fails. Only then implement the feature until that same test passes.
- For a bug fix, write an acceptance test that reproduces the reported incorrect behavior, run it, and confirm that it fails for the bug. Only then implement the fix until that same test passes.
- The test shall exercise the relevant public behavior at the most meaningful available boundary; a unit test alone is not an acceptance test unless that unit is itself the public contract.
- An unrun test, a test written after implementation, and a test that already passes do not satisfy this policy. If the required test cannot be made to fail before implementation, stop and resolve the discrepancy with the requester rather than proceeding.

**Go acceptance form (mandatory)**:

- Use Ginkgo v2 + Gomega (`github.com/onsi/ginkgo/v2`, `github.com/onsi/gomega`).
- Spec style: `Describe` / `When` / `It` (and `DescribeTable` when useful) that read as EARS/acceptance-criteria statements.
- Layout: `*_acceptance_test.go` plus `acceptance_suite_test.go` with a `TestXxxAcceptance` entrypoint, so `mise run test-acceptance-fast` (`go test … -run Acceptance`) picks them up.
- Reference examples: `cmd/coach/baseline_acceptance_test.go`, `pkg/githubingest/acceptance_test.go`.
- Plain unit tests (`*_test.go` without the acceptance suite role) may use stdlib `testing` + table tests; that is not a substitute for acceptance coverage of new features or bug fixes.
- Exception: thin stdlib `Test*Acceptance` wrappers that only call a shared harness (e.g. `internal/acceptanceharness/queueconformance/acceptance_test.go`) are allowed where they are not the behavioral specs themselves.
- Mechanical guard (when present): `mise run acceptance-style-check`.

**False-green rule**: a test counts only if it exercises the intended branch or failure mode. Shared clocks and fakes that make a different path produce the same status or outcome are invalid — for example, advancing time so a "denylisted" case actually fails on expiry.

For delegated work, the `task-implementer`/`task-reviewer` pair (`.claude/agents/`) operationalizes this policy step by step: the implementer writes and fails a Ginkgo acceptance test before implementing, and the reviewer gates on red-then-green evidence plus the form and false-green rules above. Subagent prompts shall not relax AGENTS.md — do not tell implementers that stdlib table tests substitute for Ginkgo acceptance tests. Copy conventions from here rather than inventing weaker ones.

### Outbound HTTP (required policy)

Production defaults for upstream HTTP clients shall set a finite `Timeout`, so a hung upstream cannot pin a caller indefinitely. Do not use a bare `http.DefaultClient` for a request path that can hang.

Inbound servers are the mirror of this rule: every production `http.Server` shall set `ReadHeaderTimeout`, because the Go default is unbounded and a slowloris-style client can otherwise hold an idle listener open. See `cmd/coach-api/main.go` and `internal/modelgateway/httpstub.go`.

### Store/dependency fail-closed (required policy)

Applies to the `coach-api` request paths. When a required store or dependency errors — as distinct from a clean miss or not-found — protected and auth paths shall return 503 with the stable JSON error envelope, so a partial read never reads as an authorized empty result. Do not skip the check, and do not treat store errors as a soft 500 in one path while failing closed in an analogous one.

### Go comments (required policy)

Applies to Go only. Default to no comment unless it helps a human or coding agent use or change the code correctly.

Keep or write a comment where it encodes a non-local contract:

- Exported API behavior callers cannot infer from the name (errors, auth, zero value, concurrency, special cases)
- Intentional simplifications and external wire quirks (e.g. go-github response shapes, GitHub API limits)
- Invariants that tests or agents will otherwise "fix" wrongly (race guards, auth-mode recording, false-green traps)

Form (godoc):

- Doc comments sit immediately above the declaration, are complete sentences, and start with the symbol name (`Package foo…`, `ClassifyToken reports…`)
- Prefer short paragraphs; use end-of-line comments for map keys and enum values where that is enough
- Attach notes to a declaration — no orphan `// NOTE` blocks
- Follow [Go doc comments](https://go.dev/doc/comment). Epic and issue narrative belongs in `docs/`, the PR, or the commit message, not in code

Delete, and never add:

- Restatements of the identifier or of the next line of code
- Step-by-step narration of obvious control flow
- Long essays duplicated across handlers — factor one shared helper, doc, or package comment
- Test comments that only paraphrase `It("…")` or subtest names; prefer structure and names (see `go-testable-design`), and keep only subtle assertion traps

Comment unexported symbols only for the contracts and traps above, not for routine helpers.

### Verification

Passing checks prove nothing broke; they do not prove new behavior is correct. For a `pkg/semantics` extraction or metric change, add or extend a case in the relevant `*_test.go` (`features_test.go`, `ts_features_test.go`, `query_test.go`, …) with a concrete before/after `Result`, not just a "does it run" assertion. For `js/semantics` changes, extend `parity.test.ts` so the Go and JS outputs are checked byte-identical rather than independently plausible.

### Feedback loop

After a failing check, fix and rerun that specific command rather than the whole suite — `go test -race ./... -run TestName` narrows to one test. Do not move on to the next validation step until the current one is clean.

### What `/implement-issue` guarantees, and what it does not

The command's job is continuous review: every change is written by one agent and adversarially reviewed by another before it counts, and the integrated diff is reviewed again before a PR opens. Two things are load-bearing while a run is in progress. The review-fidelity hooks (`verify-review-verdict.sh`, `verify-context-relay.sh`) fire only on agents the main session spawns, which is why the implement/review loop stays there rather than moving into a workflow. And merge safety belongs to branch protection plus the required `status` check, which run where no agent can reach them — so a run that ignores its own prose rules produces a worse PR, not an unsafe merge.

Read a PR from this flow as a well-evidenced proposal, not a verified one: it asserts reviewer `PASS` per task and an integration review of the whole diff, not that the exhaustive suite passed locally. The full split between what code holds and what prose holds, and the session-binding caveat that silently disables the hooks, are in [`docs/development/agent-review-loop.md`](docs/development/agent-review-loop.md).

## Pull requests

Before `gh pr create` / `create_pull_request`, read and fill every section of [`.github/PULL_REQUEST_TEMPLATE.md`](.github/PULL_REQUEST_TEMPLATE.md). That file is the PR contract for coding agents: linked issue, single concern, acceptance-criteria → evidence table, red-then-green acceptance proof, and the validation commands you actually ran. Do not open a PR with blank sections or placeholder text.

### Commit types (required policy)

Conventional Commits, chosen by who the change is for — GoReleaser builds release notes from commit subjects, so the type decides whether a change is described to `coach` users as part of the CLI.

- `feat` / `fix` — behavior a `coach` user can invoke: the CLI, `pkg/semantics`, `pkg/githubingest`, `js/semantics`. These reach the release notes.
- `chore` / `ci` / `build` / `refactor` / `style` / `test` / `docs` — everything else, including agent tooling: `.claude/` and `.agents/` definitions, hooks, subagent and workflow files, `mise.toml` tasks, and CI workflows. `.goreleaser.yaml` filters these out of the release notes.

A PR title follows the same rule as its commits. Agent tooling changes the way this repository is built, not what it does, so labelling it `feat` publishes a feature that does not exist.

## CI shape (`.github/workflows/ci.yml`)

Six independent leaf jobs (`verify`, `js-verify`, `projectmodel-sidecar`, `ts-project-backend`, `wasm-build`, `platform-smoke`) plus a `status` aggregator, which is the single required check. Branch protection should require `status` only. Adding a leaf means adding it to `status.needs`, or the new leaf can fail while the required check stays green. The job graph is detailed in [`docs/development/validation-and-ci.md`](docs/development/validation-and-ci.md).

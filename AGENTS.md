# AGENTS.md

Canonical project instructions for coding agents working in this repository (Codex, and any other Agent Skills-compatible harness). Claude Code loads this file via the `@./AGENTS.md` import at the top of `CLAUDE.md` — edit guidance here, not there, so the two never drift apart.

## What this is

Experimental AI coach for humans making software with agents. Currently two independent Go packages, plus a TypeScript wrapper for one of them:

- `pkg/semantics` — deterministic structural analysis of raw Go/TypeScript/TSX source bytes (syntax validity, imports, branching metrics, constructor-like patterns) via Tree-sitter. No GitHub dependency.
- `pkg/githubingest` — optional GitHub App-authenticated single-file reader via the GitHub Contents API.
- `js/semantics` — Node/TS bindings for `pkg/semantics` (`@lousy-agents/coach-semantics`, not published to npm), talking newline-delimited JSON to a Go binary over stdin/stdout.

**Dependency rule**: `pkg/semantics` never imports `pkg/githubingest` (or `go-github`/`ghinstallation`), and `pkg/githubingest` never imports `pkg/semantics` back. Keep it that way — this is what lets a consumer that only needs source analysis avoid pulling in a GitHub client.

The `coach` CLI (`cmd/coach`, plumbing in `internal/codesignalcli`) currently exposes one subcommand, `codesignal`, which produces deterministic signal reports for a git diff (`--base`) or a repository baseline (`--baseline`) via the `pkg/semantics` → `pkg/codesignal` pipeline. Product direction lives in `docs/product/prd.md`; system design in `docs/architecture/system-overview.md`.

**Living product evaluation:** `docs/product/evaluations/codesignal-pilot-readiness.html` is the current leave-pilot evidence, not a historical snapshot. Closing a #282 child or merging user-facing `coach codesignal` behavior means updating it: re-run the affected claim against HEAD, move closed gaps to the archive (do not delete them), restamp date / HEAD / `#282 · N / 24`, and re-rank only after a run. GitHub `CLOSED` is not sufficient evidence.

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
- OpenCode — no separate agent/command body mirrors. `.opencode/plugin/claude-agents.ts` loads `.claude/agents/*.md` and `.claude/commands/*.md` at config time (agents: Claude `tools` → OpenCode `permission`, `maxTurns` → `steps`, `mode: subagent`; commands: frontmatter `description` + body as `template`). Explicit entries in `opencode.json` / `.opencode/agents/` / `.opencode/command(s)/` win over the loader. `.opencode/plugin/implement-issue-gates.ts` mirrors the two review-loop hooks: `task` → `task-implementer` rework requires the literal `## Reviewer Findings` heading, and `task` → `task-reviewer` results are soft-gated so the first non-empty line is `PASS` or `FINDINGS`. Restart OpenCode after agent, command, or plugin changes.
- `.codex/agents/*.toml` — Codex custom subagents (`name`, `description`, `sandbox_mode`, `developer_instructions`). Codex cannot import Claude markdown, so instruction text is mirrored from `.claude/agents/` and marked with a one-line sync comment — don't build codegen for a two-file mirror.
- `.agents/skills/*/agents/<harness>.yaml` — optional, separate from subagent definitions: a per-harness "interface" declaration (e.g. `display_name`/`default_prompt`) for how a skill surfaces in that harness's UI. Only add one if the harness actually reads it — Claude Code has no such mechanism today.

### Workflows (`.claude/workflows/`) — Claude Code only

Scripts the Claude Code Workflow tool executes to orchestrate subagents deterministically (`.claude/workflows/implement-issue-plan.js` plans an issue). **No other harness has an equivalent**, and the OpenCode loader above mirrors agents and commands only — so a command that delegates work to a workflow must also state that work inline, or it is broken the moment OpenCode loads it. `.opencode/plugin/claude-agents.test.ts` enforces this.

Two invariants apply to anything a workflow does:

- **Hooks do not reach inside.** `SubagentStop` and `PreToolUse` fire on agents the main session spawns and on its own tool calls. An agent spawned inside a workflow reaches neither, so `verify-review-verdict.sh` and `verify-context-relay.sh` do not run for it — a workflow that took over the implement/review loop would look identical while the review loop silently lost its fidelity checks.
- **Nothing else executes these scripts**, so a syntax error or renamed binding surfaces only when a human runs the command. `mise run workflow-test` imports each one under a fake harness; it is wired into `js-ci` rather than `verify` because the `verify` job has no Node and the check would skip silently there.

## Commands

All tasks are defined in `mise.toml`; use `mise run <task>` (mise also pins `go` and `node` versions — CI installs mise so both share one tool-version source of truth).

```sh
mise run ci-gate          # fast local smoke: gofmt/vet/style only, no tests (~1s warm)
mise run ci-all           # everything CI proves locally: sidecar-first ci-go + js-ci + wasm-build
mise run ci-fast          # per-cycle loop check: Go slice + agent-tooling suites, sidecar built first
mise run ci               # ci-go + js-ci -- NOT wasm-build, and see ci-fast on skips
mise run ci-go             # verify job atoms: gofmt/vet/tidy/style/test/examples
mise run gofmt             # fail if gofmt -l . prints any file
mise run projectmodel-sidecar-acceptance  # real TS sidecar specs only (GHA job of the same role)
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

These are the exact checks CI runs in `.github/workflows/ci.yml` (atomic `mise run <task>` steps). A clean local `mise run ci-all` covers every job except `platform-smoke`:

```sh
mise run gofmt
mise run go-vet
mise run tidy-check
mise run acceptance-style-check
mise run test
mise run test-examples
mise run js-ci
mise run projectmodel-sidecar-acceptance
mise run wasm-build
```

`mise run ci` runs `ci-go` **and `js-ci`**. It does **not** run `wasm-build`.

Two gaps `mise run ci` alone does not close:

- `wasm-build` is in no task's closure, so a `GOOS=js GOARCH=wasm` break can pass `ci`.
- `ci` runs `test` before anything builds the sidecar, so `pkg/projectmodel`'s TypeScript sidecar acceptance suite **skips silently** unless Node and the binary are already present. A green `ci` does not mean that suite ran.

Use **`mise run ci-fast`** inside an implement/review loop — sidecar-first ordering, without the wasm and full-CI legs. **`mise run ci-all`** builds the sidecar first, then `ci-go` (so `test` actually runs the projectmodel suite), `js-ci`, and `wasm-build`; run it when you want the whole thing locally, but it is **no longer the pre-PR gate**.

**The exhaustive gate is GitHub Actions plus branch protection, not a local run.** A serial local `ci-all` measured ~910s on a CCR container, while CI proves a strict superset (the same atomic tasks as parallel leaf jobs, **plus `platform-smoke`**) in ~426s wall clock on compute that is not the session's. Since the `status` aggregator became a required check, a red tree cannot merge no matter what any local check decides — so nothing gates PR creation locally. Commit and push everything before opening a PR so its evidence describes the tree you pushed; that is a discipline, not a mechanism.

`ci-gate` is a fast local smoke check you can run yourself. It deliberately runs no tests and **no `tidy-check`** — the latter rewrites `go.mod`/`go.sum` in place. This whole arrangement is only safe while `status` is a **required check** on the base branch; if branch protection is removed, nothing gates a red merge.

`ci-all` deliberately excludes `test-acceptance-fast` (its ambient-credential preflight cannot pass where `GITHUB_TOKEN`/`GH_TOKEN` or `~/.aws/config` are present, and `test` already runs every acceptance suite unfiltered) and `platform-smoke` (Docker + live services).

A cross-language parity or coverage/failure acceptance gate — one whose acceptance criteria assert on the rendered CLI report across both the Go and TypeScript project backends — exercises this same mandatory Validation Suite, plus `mise run test-acceptance-fast` run directly rather than only through `mise run test`: it names and scopes the acceptance-suite layer explicitly (`docs/architecture/acceptance-harness.md`) and carries its own preflight guard (`go run ./cmd/acceptance-guard-preflight`), independent of the wider `test` superset. In a credentialed environment, `test-acceptance-fast`'s ambient-credential preflight refusal is the documented, expected result, not a gate failure — `mise run test` already ran the same `*Acceptance` suites unfiltered. This gate also puts a ~4-minute floor under `cmd/coach`'s suite (`go test -race ./cmd/coach/...` measured ~239s): roughly 180s of that is deliberate blocking on `tsSidecarWallTime` (60s, `internal/codesignalcli/project_ts_backend.go`) and `snapshotGitTimeout` (30s, `internal/codesignalcli/project_snapshot.go`), each paid twice across the project-backend and no-findings-verdict acceptance specs, so `cmd/coach` now dominates `mise run test`'s wall time and the `ci-fast` "per-cycle loop check" description above should not be read as implying this suite is quick.

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

The command's job is **continuous review**: every change is written by one agent and adversarially reviewed by another before it counts, and the integrated diff is reviewed again before a PR opens. It deliberately does *not* try to box agents in with control mechanisms — an earlier revision carried a git-write jail, a cycle-ceiling counter, a PR-creation gate, a hook trace, and a five-probe liveness ritual to prove them all live, and that apparatus spent more effort watching itself than reviewing code. It was removed. Merge safety belongs to **branch protection plus the required `status` check**, which run where no agent can reach them.

**Deterministic, held by code:**

- **Planning.** `.claude/workflows/implement-issue-plan.js` produces the task DAG. Its arg validation, null guards, cycle and dangling-reference checks are JS, covered by `mise run workflow-test`, and mutation-tested.
- **Review-loop fidelity.** Two small hooks, both scoped to the loop's conversation rather than to what agents may do: `verify-review-verdict.sh` (a reviewer's reply must begin `PASS` or `FINDINGS`, so the orchestrator always receives a parseable verdict) and `verify-context-relay.sh` (a rework delegation must carry the literal `## Reviewer Findings` block, so findings cannot be paraphrased away). They fire on agents this session spawns — agents inside a workflow never reach them, which is why the implement/review loop stays in the main session.
- **Merge safety.** Branch protection and the required `status` aggregator on the base branch. This is the only gate that matters for an unattended run, and it runs on GitHub's side.

**Prose, held by the orchestrator following instructions:**

- The per-task cycle cap (3), the integration and repair caps, and the no-progress rule
- Dependency ordering — that a task waits for its `dependsOn`
- Which task receives a validation failure, and the invalidation rule after rework
- That the `conventions` string reaches implementers unweakened
- That implementers do not commit, push, or open PRs — the orchestrator owns git

Those are instructions in `.claude/commands/implement-issue.md`. The specs in `internal/agentworkflows/` assert that **the instruction is present and says the right thing** — they cannot assert that a run obeyed it. A run that ignores them produces a worse PR, not an unsafe merge: the required checks still gate the merge.

**What a PR opened by this flow asserts.** That every task reached reviewer `PASS` and the integration reviewer passed the whole diff, and that the body records the per-task `ci-fast` output as its test evidence.

**What it does not assert.** That the exhaustive suite passed locally — it no longer runs locally. Exhaustive verification lives in GitHub Actions and gates **merge**, not PR creation. Nor `platform-smoke` or `test-acceptance-fast` results, nor that any prose-held rule above was obeyed. The practical reading: **a PR from this flow is a well-evidenced proposal, not a verified one.** Branch protection is what makes it safe to open one unattended.

One environment caveat worth knowing: Claude Code binds `.claude/` — hooks, agents, and workflows alike — to the session's project directory at session start. A repository cloned into a session whose project directory is elsewhere never registers any of them, and attaching it mid-session reloads CLAUDE.md and skills but not hooks, agents, or workflows. In that state the two review-fidelity hooks are silently absent and the run degrades to prose-only review discipline — still merge-safe, because branch protection does not care, but weaker. Sessions for this repo should be created with the repo as the project directory.

## Pull requests

Before `gh pr create` / `create_pull_request`, read and fill every section of [`.github/PULL_REQUEST_TEMPLATE.md`](.github/PULL_REQUEST_TEMPLATE.md). That file is the PR contract for coding agents: linked issue, single concern, acceptance-criteria → evidence table, red-then-green acceptance proof, and the validation commands you actually ran. Do not open a PR with blank sections or placeholder text.

### Commit types (required policy)

Conventional Commits, chosen by **who the change is for** — GoReleaser builds release notes from commit subjects, so the type decides whether a change is described to `coach` users as part of the CLI.

- `feat` / `fix` — behavior a `coach` user can invoke: the CLI, `pkg/semantics`, `pkg/githubingest`, `js/semantics`. These reach the release notes.
- `chore` / `ci` / `build` / `refactor` / `style` / `test` / `docs` — everything else, including **agent tooling**: `.claude/` and `.agents/` definitions, hooks, subagent and workflow files, `mise.toml` tasks, and CI workflows. These are filtered out of the release notes by `.goreleaser.yaml`.

A PR title follows the same rule as its commits. Agent tooling changes the way this repository is *built*, not what it *does*, so labelling it `feat` publishes a feature that does not exist.

## CI shape (`.github/workflows/ci.yml`)

GHA is a parallel scheduler of atomic `mise run <task>` steps. Local `ci` / `ci-fast` / `ci-all` are serial bundles of the same tasks. The workflow does not invoke those composites.

Five independent leaf jobs plus a `status` aggregator (the single required check):

- `verify` — `gofmt` / `go-vet` / `tidy-check` / `acceptance-style-check` / `test` / `test-examples` (`ci-go`). mise installs only Go; the runner image may still have Node, so the sidecar suite usually skips because the sidecar is not built, not because `node` is missing. Toolchain is `mise.toml` (`go = "1.26.6"`), not `go.mod`'s language version.
- `js-verify` — `mise run js-ci` only.
- `projectmodel-sidecar` — `mise run projectmodel-sidecar-acceptance` (builds the sidecar, then the real suite). Parallel with `js-verify`.
- `wasm-build` — `mise run wasm-build`.
- `platform-smoke` — `platform-up` / `platform-smoke` / `platform-down` as three steps so teardown still runs on failure.
- `status` — `if: always()` + `needs` every leaf; inspects `toJSON(needs)` and fails unless each result is `success`. Branch protection should require this job only. Adding a leaf means adding it to `status.needs`, or it can fail while the required check is green.

`mise run ci-all` mirrors the first four locally (sidecar-first, so the suite runs inside `test` rather than as a second invocation). `platform-smoke` has no local composite; run the three platform tasks directly. The GHA `status` check is stricter than `ci-all` because it includes `platform-smoke`.

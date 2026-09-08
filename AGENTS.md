# AGENTS.md

This file is the canonical instruction set for coding agents in this repository.
Claude Code shall load this file through the `@./AGENTS.md` import in `CLAUDE.md`.
The agent shall edit guidance here. The agent shall not edit `CLAUDE.md` for project guidance.

## What this is

Coach is an experimental AI coach for humans who make software with agents.
This repository contains two independent Go packages and one TypeScript wrapper.

- `pkg/semantics` analyses Go, TypeScript, and TSX source bytes with Tree-sitter.
- `pkg/semantics` reports syntax validity, imports, branching metrics, and constructor-like patterns.
- `pkg/semantics` has no GitHub dependency.
- `pkg/githubingest` reads one file through the GitHub Contents API with GitHub App auth.
- `js/semantics` is the Node/TS binding for `pkg/semantics` (`@lousy-agents/coach-semantics`).
- `js/semantics` is not published to npm.
- `js/semantics` sends newline-delimited JSON to a Go binary on stdin and stdout.

**Dependency rule**

The `pkg/semantics` package shall not import `pkg/githubingest`, `go-github`, or `ghinstallation`.
The `pkg/githubingest` package shall not import `pkg/semantics`.
This rule lets a consumer that needs only source analysis omit a GitHub client.

The `coach` CLI lives in `cmd/coach`. Plumbing lives in `internal/codesignalcli`.
The CLI exposes one subcommand: `codesignal`.
The `codesignal` command produces signal reports for a git diff (`--base`) or a repository baseline (`--baseline`).
The pipeline is `pkg/semantics` then `pkg/codesignal`.
Product direction lives in `docs/product/prd.md`.
System design lives in `docs/architecture/system-overview.md`.

**Living product evaluation**

`docs/product/evaluations/codesignal-pilot-readiness.html` is the current leave-pilot evidence.
That file is not a historical snapshot.
When the agent closes a #282 child, the agent shall update that file.
When the agent merges user-facing `coach codesignal` behavior, the agent shall update that file.
The agent shall re-run the affected claim against HEAD.
The agent shall move closed gaps to the archive. The agent shall not delete them.
The agent shall restamp date, HEAD, and `#282 · N / 24`.
The agent shall re-rank only after a run.
GitHub `CLOSED` is not sufficient evidence.

## Agent Skills (`.agents/skills/`)

- `feature-to-plan` — turn a feature request, PRD, or backlog issue into a structured EARS-format spec.
- `go-testable-design` — guidance for writing or refactoring testable Go.
- `mutation-hunter` — find TypeScript test-coverage gaps via semantic mutation testing.
- `rugged-evil-tester` — generate adversarial, negative, and chaos tests for TypeScript code.
- `product-quality-evaluation` — get an evidence-grounded product assessment via the `product-sme` subagent.
- `designing-for-intent` — review a UX, onboarding, or consent artifact for intent map, delegation boundary, and agency risks.
- `designing-for-intent` is complementary to the `ux-advocate` roster seat. It is not a wrapper around that seat.
- `skill-reviewer` — lint and review Agent Skills `SKILL.md` files across harnesses.
- `spec-auditor` — review specs, PRDs, and plans before coding.
- `triaging-pr-reviews` — classify and triage PR review comments, including automated reviewer suggestions.
- `correctness-review` — review a GitHub pull request against linked acceptance criteria, architecture, and specs.
- `issue-refine-loop` — refine a GitHub issue in place into an implementation-ready epic, then decompose it.

## Custom subagents

Some skills delegate to a named subagent. Each harness defines subagents in its own format.

- `.claude/agents/*.md` — canonical Claude Code subagents (YAML frontmatter plus markdown body as the system prompt).
- `task-implementer` and `task-reviewer` back the `implement-issue` command.
- `product-sme` backs `product-quality-evaluation`.
- `ux-advocate` is a roster peer for customer-facing journey reading. It does not back `designing-for-intent`.
- The orchestrator shall route product and roadmap questions to `product-sme`.
- The orchestrator shall route experience, journey, and agency questions to `ux-advocate`.
- The orchestrator shall route architecture and design questions to `system-design-expert`.
- When a spec exists, the orchestrator shall route spec-audit questions to `spec-review-agent`.
- The orchestrator shall delegate to these seats. The orchestrator shall not impersonate them.
- The agent shall edit these files when it changes agent instructions.
- **Exception:** `ux-advocate` is a Coach host-binding wrapper over the lockfile-sourced `designing-for-intent` agent.
- The wrapper holds a host peer map, then the published charter inlined.
- The agent shall not fork the charter in this wrapper.
- The agent shall change `lousy-agents/skills`, then re-run `npx skills add`.
- The agent shall refresh the inlined body from `.agents/skills/designing-for-intent/agents/ux-advocate.md`.
- The agent shall edit only the host peer map in this wrapper.
- OpenCode has no separate agent or command body mirrors.
- `.opencode/plugin/claude-agents.ts` loads `.claude/agents/*.md` and `.claude/commands/*.md` at config time.
- Agents map Claude `tools` to OpenCode `permission`, `maxTurns` to `steps`, and `mode: subagent`.
- Commands use frontmatter `description` plus body as `template`.
- Explicit entries in `opencode.json`, `.opencode/agents/`, or `.opencode/command(s)/` win over the loader.
- `.opencode/plugin/implement-issue-gates.ts` mirrors the two review-loop hooks.
- When `task` targets `task-implementer` for rework, the prompt shall include the literal `## Reviewer Findings` heading.
- When `task` targets `task-reviewer`, the first non-empty line shall be `PASS` or `FINDINGS`.
- After agent, command, or plugin changes, the operator shall restart OpenCode.
- `.codex/agents/*.toml` — Codex custom subagents (`name`, `description`, `sandbox_mode`, `developer_instructions`).
- Codex cannot import Claude markdown, so instruction text is mirrored from `.claude/agents/`.
- Each Codex file shall carry a one-line sync comment.
- The agent shall not build codegen for a two-file mirror.
- `ux-advocate.toml` mirrors the Coach host-binding wrapper.
- After a skills-lock update, the agent shall refresh the inlined charter.
- `.agents/skills/*/agents/<harness>.yaml` is an optional per-harness interface declaration.
- Add one only if the harness reads it. Claude Code has no such mechanism today.

### Workflows (`.claude/workflows/`) — Claude Code only

The Claude Code Workflow tool executes these scripts to orchestrate subagents.
`.claude/workflows/implement-issue-plan.js` plans an issue.
No other harness has an equivalent.
The OpenCode loader mirrors agents and commands only.
If a command delegates work to a workflow, then the command shall also state that work inline.
If it does not, then the command fails the moment OpenCode loads it.
`.opencode/plugin/claude-agents.test.ts` enforces this.

Two invariants apply to anything a workflow does:

- **Hooks do not reach inside.** `SubagentStop` and `PreToolUse` fire on agents the main session spawns and on its own tool calls.
- An agent spawned inside a workflow reaches neither hook.
- `verify-review-verdict.sh` and `verify-context-relay.sh` do not run for that agent.
- If a workflow took over the implement/review loop, then the review loop would lose its fidelity checks.
- **Nothing else executes these scripts.** A syntax error or renamed binding surfaces only when a human runs the command.
- `mise run workflow-test` imports each script under a fake harness.
- That check is wired into `js-ci` rather than `verify`.
- The `verify` job has no Node. If the check lived there, then it would skip silently.

## Commands

All tasks are defined in `mise.toml`.
The agent shall use `mise run <task>`.
mise pins `go` and `node` versions.
CI installs mise so both share one tool-version source of truth.

```sh
mise run ci-gate          # local smoke: gofmt/vet/style only, no tests (~1s warm)
mise run ci-all           # sidecar-first ci-go + js-ci + wasm-build
mise run ci-fast          # per-cycle loop check: Go slice + agent-tooling suites, sidecar built first
mise run ci               # ci-go + js-ci -- NOT wasm-build, and see ci-fast on skips
mise run ci-go             # verify job atoms: gofmt/vet/tidy/style/test/examples
mise run gofmt             # fail if gofmt -l . prints any file
mise run projectmodel-sidecar-acceptance  # real TS sidecar specs only (GHA job of the same role)
mise run go-vet
mise run tidy-check        # go mod tidy && diff go.mod/go.sum
mise run test              # go test -race ./...
mise run test-examples     # go test -run Example ./...
mise run test-acceptance-fast # in-process Ginkgo/Gomega acceptance suites (offline, no real credentials)
mise run acceptance-style-check # fails if any *_acceptance_test.go lacks ginkgo/v2 (except allowlist)
mise run test-queue-conformance # queue conformance harness self-test; Redis Streams/LocalStack SQS legs land with Baseline Task 3a
mise run thinproof-build    # vendors deps + builds the thin offline Compose proof's Docker images (run once, online)
mise run test-acceptance-thin-proof # offline thin Compose proof: fake GitHub -> pkg/githubingest -> CodeSignal, no image pull, no egress
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

`pkg/semantics` will parse in Go through `github.com/odvcencio/gotreesitter`.
The package will not use CGO. The package will not require a C toolchain.
The package will not select a dual backend.

## Architecture: `pkg/semantics`

Pipeline (`analyzer.go`): `AnalyzeBytes` = validate -> parse -> syntax-check -> extract imports -> compute metrics/findings -> `Result`.

- **Backend seam** (`internal/engine/engine.go`): a narrow interface (`Node`, `Tree`, `Parser`, `Query`, `QueryCursor`, `Language`).
- The interface exposes only the Tree-sitter operations the package uses.
- The interface has no `NamedChild`, `TreeCursor`, query predicates, or incremental parsing.
- This package is `internal`. Only `pkg/semantics` may import it.
- There is exactly one implementation: `internal/engine/gotreesitter.go` (pure-Go, always compiled, no build tag).
- **Registry selection** (`language.go`): `languageSpec` bundles a backend-bound `engine.Language` handle with `extractImports` and `computeFeatures`.
- `languageRegistry` (`map[Language]languageSpec`) is defined unconditionally in `language.go`.
- The registry has no build tags and no per-backend variants.
- When the agent adds a language, the agent shall extend the registry plus its `extract*Imports`/`compute*Features` pair.
- The agent shall not touch `parser.go` or `analyzer.go` for that change.
- **Concurrency**: `*Analyzer` holds no backend resources between calls.
- When the caller invokes `AnalyzeBytes`, the Analyzer shall create and close its own `Parser`, `Tree`, `Query`, and `QueryCursor`.
- A single `*Analyzer` is safe for concurrent use.
- **Error contract**: If the parser finds syntax errors, then `AnalyzeBytes` shall return a partial `*Result` with `ParseStatus == "syntax_errors"`.
- If the parser finds syntax errors, then `AnalyzeBytes` shall also return a non-nil error that satisfies `errors.Is(err, ErrSyntax)`.
- Use `errors.As` for `*SyntaxError.Issues`.
- Other sentinels: `ErrEmptyContent`, `ErrUnsupportedLanguage`, `ErrFileTooLarge`, `ErrBinaryContent`, `ErrParseFailure`.
- **JSON stability**: `Result` and nested types use frozen `snake_case` JSON field names, locked by `result_test.go`.
- Field names and error identities (`Err*` sentinels, `*SyntaxError`) are a stable pre-1.0 API surface.
- Other surface may still change.
- `internal/jsbridge` (repo-root `internal/`, not under `pkg/semantics`) implements the newline-delimited JSON protocol.
- `cmd/semantics-json` consumes that protocol. `js/semantics` shells out to that binary.
- `js/semantics/src/protocol.ts` mirrors the protocol.
- `js/semantics/test/parity.test.ts` replays shared fixtures through the Go API and the JS package.
- The two outputs shall stay byte-identical.

## Architecture: `js/semantics`

This TypeScript package has a `Backend` seam (`src/backend.ts`) that abstracts transport.
`src/backend-cli.ts` and `backend-default.ts` spawn the compiled `coach-semantics-json` Go binary.
They speak the jsbridge protocol over stdio.
A WASM backend (`backend-wasm.ts`) is not yet wired up.
`pkg/semantics` now builds for `GOOS=js GOARCH=wasm` (see `wasm-build` and `cmd/semantics-wasm-smoke`).
When the agent swaps transports, the agent shall keep the change behind the `Backend` seam.
The public API shall not change for a transport swap.
`npm install` and `prepare` build the Go backend binary and the TS package.
Go is required even for JS-only work.

## Architecture: `pkg/githubingest`

The single entry point is `ReadFile`.
Auth uses a GitHub App installation (`ghinstallation` plus `go-github`).
Each call issues two Contents API requests: the file fetch, plus a listing of the parent directory.
The listing detects in-repo symlinks that GitHub's Contents API would resolve as a plain file (`reader.go`'s `rejectIfPathIsSymlink`).
That listing is capped at GitHub's 1,000-entries-per-directory limit with no truncation signal.
If a directory has more than 1,000 entries, then a symlink in that directory can go undetected.
That limit is an accepted, documented limitation for v1.
Error sentinels: `ErrNotFound`, `ErrAuth`, `ErrUnsupportedContent`, `ErrTooLarge` (>1 MiB), `ErrEmptyContent`.

## Validation

### Validation Suite (mandatory before commit)

These are the exact checks CI runs in `.github/workflows/ci.yml` (atomic `mise run <task>` steps).
A clean local `mise run ci-all` covers every job except `platform-smoke`:

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
- `ci` runs `test` before anything builds the sidecar.
- If Node and the sidecar binary are absent, then `pkg/projectmodel`'s TypeScript sidecar suite **skips silently**.
- A green `ci` does not mean that suite ran.

The agent shall use **`mise run ci-fast`** inside an implement/review loop.
`ci-fast` uses sidecar-first ordering. It omits the wasm and full-CI legs.
**`mise run ci-all`** builds the sidecar first, then `ci-go`, `js-ci`, and `wasm-build`.
Because the sidecar is built first, `test` actually runs the projectmodel suite.
The agent should run `ci-all` when it wants the whole local set.
`ci-all` is **no longer the pre-PR gate**.

**The exhaustive gate is GitHub Actions plus branch protection, not a local run.**
A serial local `ci-all` measured ~910s on a CCR container.
CI proves a strict superset in ~426s wall clock on compute that is not the session's.
CI runs the same atomic tasks as parallel leaf jobs, **plus `platform-smoke`**.
The `status` aggregator is a required check.
If the tree is red, then merge shall fail no matter what any local check decides.
Nothing gates PR creation locally.
The agent shall commit and push everything before it opens a PR.
The PR evidence shall describe the tree that was pushed.
That rule is a discipline, not a mechanism.

`ci-gate` is a local smoke check. It completes in about 1s when warm.
It runs no tests and **no `tidy-check`**.
`tidy-check` rewrites `go.mod`/`go.sum` in place.
This arrangement is only safe while `status` is a **required check** on the base branch.
If branch protection is removed, then nothing gates a red merge.

`ci-all` excludes `test-acceptance-fast`.
That task's ambient-credential preflight cannot pass where `GITHUB_TOKEN`, `GH_TOKEN`, or `~/.aws/config` are present.
`test` already runs every acceptance suite unfiltered.
`ci-all` also excludes `platform-smoke` (Docker plus live services).

A cross-language parity or coverage/failure acceptance gate shall exercise this Validation Suite.
That gate shall also run `mise run test-acceptance-fast` directly, not only through `mise run test`.
It names and scopes the acceptance-suite layer (`docs/architecture/acceptance-harness.md`).
It carries its own preflight guard (`go run ./cmd/acceptance-guard-preflight`).
In a credentialed environment, `test-acceptance-fast`'s ambient-credential preflight refusal is the expected result.
That refusal is not a gate failure. `mise run test` already ran the same `*Acceptance` suites unfiltered.
This gate also puts a ~4-minute floor under `cmd/coach`'s suite.
`go test -race ./cmd/coach/...` measured ~239s.
About 180s of that is blocking on `tsSidecarWallTime` (60s, `internal/codesignalcli/project_ts_backend.go`) and `snapshotGitTimeout` (30s, `internal/codesignalcli/project_snapshot.go`).
Each timeout is paid twice across the project-backend and no-findings-verdict acceptance specs.
`cmd/coach` now dominates `mise run test`'s wall time.
The agent shall not read the `ci-fast` "per-cycle loop check" label as a claim that this suite is quick.

### Acceptance-test-first (required policy)

When the agent adds a feature or fixes a bug, the agent shall begin with a failing acceptance test.
The agent shall not make production implementation changes before that test fails.

- When the agent adds a feature, the agent shall write an acceptance test that shows the requested public behavior is absent.
- The agent shall run that test and confirm that it fails.
- Then the agent shall implement the feature until that same test passes.
- When the agent fixes a bug, the agent shall write an acceptance test that reproduces the incorrect behavior.
- The agent shall run that test and confirm that it fails for the bug.
- Then the agent shall implement the fix until that same test passes.
- The test shall exercise the relevant public behavior at the most meaningful available boundary.
- A unit test alone is not an acceptance test unless that unit is itself the public contract.
- An unrun test, a test written after implementation, or a test that already passes shall not satisfy this policy.
- If the required test cannot be made to fail before implementation, then the agent shall stop.
- The agent shall resolve the discrepancy with the requester rather than proceed.

**Go acceptance form (mandatory):**

- The agent shall use Ginkgo v2 + Gomega (`github.com/onsi/ginkgo/v2`, `github.com/onsi/gomega`).
- Spec style: `Describe` / `When` / `It` (and `DescribeTable` when useful) that read as EARS statements.
- Layout: `*_acceptance_test.go` plus `acceptance_suite_test.go` with a `TestXxxAcceptance` entrypoint.
- `mise run test-acceptance-fast` (`go test … -run Acceptance`) shall pick them up.
- Reference examples: `cmd/coach/baseline_acceptance_test.go`, `pkg/githubingest/acceptance_test.go`.
- Plain unit tests (`*_test.go` without the acceptance suite role) may use stdlib `testing` plus table tests.
- Those unit tests shall not substitute for acceptance coverage of new features or bug fixes.
- Exception: thin stdlib `Test*Acceptance` **wrappers** that only call a shared harness are allowed.
- Example: `internal/acceptanceharness/queueconformance/acceptance_test.go`.
- Those wrappers are allowed only when they are not the behavioral specs themselves.
- Mechanical guard (when present): `mise run acceptance-style-check`.

**False-green rule:** a test counts only if it exercises the intended branch or failure mode.
If shared clocks or fakes make a different path produce the same status, then that test is invalid.
Example: advancing time so a "denylisted" case actually fails on expiry.

For delegated work, the `task-implementer`/`task-reviewer` pair (`.claude/agents/`) operationalizes this policy.
The implementer shall write and fail a Ginkgo acceptance test before implementing.
The reviewer shall gate on red-then-green evidence plus the form and false-green rules above.
Subagent prompts shall not relax AGENTS.md.
The orchestrator shall not tell implementers that stdlib table tests substitute for Ginkgo acceptance tests.
The orchestrator shall copy conventions from here. The orchestrator shall not invent weaker ones.

### Outbound HTTP (required policy)

Production defaults for upstream HTTP clients shall use a finite `Timeout`.
The agent shall not use bare `http.DefaultClient` for request paths that can hang.

### Store/dependency fail-closed (required policy)

If a required store or dependency errors (not a clean miss), then protected auth paths shall return **503**.
The response shall use the stable JSON error envelope. The path shall fail closed.
The agent shall not skip the check.
The agent shall not treat store errors as soft 500 inconsistently across analogous paths.

### Go comments (required policy)

Default is **no comment** unless it helps a human or coding agent use or change the code correctly.
This policy applies to Go only.

**Keep / write** when the comment encodes a non-local contract:

- Exported API behavior callers cannot infer from the name (errors, auth, zero value, concurrency, special cases)
- Intentional simplifications and external wire quirks (for example, go-github response shapes, GitHub API limits)
- Invariants that tests or agents will otherwise “fix” wrongly (race guards, auth-mode recording, false-green traps)

**Form (godoc):**

- Doc comments sit immediately above the declaration.
- They are complete sentences. They start with the symbol name (`Package foo…`, `ClassifyToken reports…`).
- Prefer short paragraphs. Use end-of-line comments for map keys or enum values when enough.
- Attach notes to a declaration. Do not write orphan `// NOTE` blocks.
- Follow [Go doc comments](https://go.dev/doc/comment).
- Do not put epic or issue narrative in code. That belongs in `docs/`, the PR, or the commit message.

**Delete / never add:**

- Restating the identifier or the next line of code
- Step-by-step narration of obvious control flow
- Long essays duplicated across handlers (factor one shared helper, doc, or package comment)
- Test comments that only paraphrase `It("…")` or subtest names
- Prefer structure and names (see `go-testable-design`). Keep only subtle assertion traps.

**Unexported** symbols: comment only for the contracts and traps above, not routine helpers.

### Verification

Passing checks prove nothing broke. They do not prove new behavior is correct.
When the agent changes a `pkg/semantics` extraction or metric, the agent shall add or extend a case in the relevant `*_test.go`.
Examples: `features_test.go`, `ts_features_test.go`, `query_test.go`.
The case shall include a concrete before/after `Result`, not only a "does it run" assertion.
When the agent changes `js/semantics`, the agent shall extend `parity.test.ts`.
The Go and JS outputs shall be byte-identical.

### Feedback Loop

After a failing check, the agent shall fix and rerun that specific command.
The agent shall not rerun the whole suite first.
`go test -race ./... -run TestName` narrows to one test.
The agent shall not move to the next validation step until the current one is clean.

### What `/implement-issue` guarantees, and what it does not

The command's job is **continuous review**.
One agent writes each change. Another agent reviews that change before it counts.
The integrated diff is reviewed again before a PR opens.
The command shall not box agents with control mechanisms.
An earlier revision carried a git-write jail, a cycle-ceiling counter, a PR-creation gate, a hook trace, and a five-probe liveness ritual.
That apparatus spent more effort watching itself than reviewing code. It was removed.
Merge safety belongs to **branch protection plus the required `status` check**.
Those checks run where no agent can reach them.

**Deterministic, held by code:**

- **Planning.** `.claude/workflows/implement-issue-plan.js` produces the task DAG.
- Its arg validation, null guards, cycle checks, and dangling-reference checks are JS.
- `mise run workflow-test` covers them. They are mutation-tested.
- **Review-loop fidelity.** Two small hooks, both scoped to the loop's conversation.
- `verify-review-verdict.sh`: a reviewer's reply shall begin `PASS` or `FINDINGS`.
- `verify-context-relay.sh`: a rework delegation shall carry the literal `## Reviewer Findings` block.
- They fire on agents this session spawns. Agents inside a workflow never reach them.
- That is why the implement/review loop stays in the main session.
- **Merge safety.** Branch protection and the required `status` aggregator on the base branch.
- This is the only gate that matters for an unattended run. It runs on GitHub's side.

**Prose, held by the orchestrator following instructions:**

- The per-task cycle cap (3), the integration and repair caps, and the no-progress rule
- Dependency ordering — that a task waits for its `dependsOn`
- Which task receives a validation failure, and the invalidation rule after rework
- That the `conventions` string reaches implementers unweakened
- That implementers do not commit, push, or open PRs — the orchestrator owns git

Those are instructions in `.claude/commands/implement-issue.md`.
The specs in `internal/agentworkflows/` assert that **the instruction is present and says the right thing**.
They cannot assert that a run obeyed it.
A run that ignores them produces a worse PR, not an unsafe merge.
The required checks still gate the merge.

**What a PR opened by this flow asserts.** Every task reached reviewer `PASS`.
The integration reviewer passed the whole diff.
The body records the per-task `ci-fast` output as its test evidence.

**What it does not assert.** That the exhaustive suite passed locally — it no longer runs locally.
Exhaustive verification lives in GitHub Actions and gates **merge**, not PR creation.
It does not assert `platform-smoke` or `test-acceptance-fast` results.
It does not assert that any prose-held rule above was obeyed.
The practical reading: **a PR from this flow is a well-evidenced proposal, not a verified one.**
Branch protection is what makes it safe to open one unattended.

Claude Code binds `.claude/` — hooks, agents, and workflows — to the session's project directory at session start.
If a repository is cloned into a session whose project directory is elsewhere, then none of them register.
Attaching it mid-session reloads CLAUDE.md and skills but not hooks, agents, or workflows.
In that state the two review-fidelity hooks are silently absent.
The run degrades to prose-only review discipline.
It is still merge-safe, because branch protection does not care, but weaker.
Sessions for this repo should be created with the repo as the project directory.

## Pull requests

Before `gh pr create` / `create_pull_request`, the agent shall read and fill every section of [`.github/PULL_REQUEST_TEMPLATE.md`](.github/PULL_REQUEST_TEMPLATE.md).
That file is the PR contract for coding agents.
It requires a linked issue, a single concern, an acceptance-criteria → evidence table, red-then-green proof, and the validation commands that actually ran.
The agent shall not open a PR with blank sections or placeholder text.

### Commit types (required policy)

Conventional Commits, chosen by **who the change is for**.
GoReleaser builds release notes from commit subjects.
The type decides whether a change is described to `coach` users as part of the CLI.

- `feat` / `fix` — behavior a `coach` user can invoke: the CLI, `pkg/semantics`, `pkg/githubingest`, `js/semantics`.
- These reach the release notes.
- `chore` / `ci` / `build` / `refactor` / `style` / `test` / `docs` — everything else, including **agent tooling**.
- Agent tooling includes `.claude/` and `.agents/` definitions, hooks, subagent and workflow files, `mise.toml` tasks, and CI workflows.
- `.goreleaser.yaml` filters these out of the release notes.

A PR title follows the same rule as its commits.
Agent tooling changes the way this repository is *built*, not what it *does*.
If the agent labels it `feat`, then release notes publish a feature that does not exist.

## CI shape (`.github/workflows/ci.yml`)

GHA is a parallel scheduler of atomic `mise run <task>` steps.
Local `ci` / `ci-fast` / `ci-all` are serial bundles of the same tasks.
The workflow does not invoke those composites.

Five independent leaf jobs plus a `status` aggregator (the single required check):

- `verify` — `gofmt` / `go-vet` / `tidy-check` / `acceptance-style-check` / `test` / `test-examples` (`ci-go`).
- mise installs only Go. The runner image may still have Node.
- The sidecar suite usually skips because the sidecar is not built, not because `node` is missing.
- Toolchain is `mise.toml` (`go = "1.26.6"`), not `go.mod`'s language version.
- `js-verify` — `mise run js-ci` only.
- `projectmodel-sidecar` — `mise run projectmodel-sidecar-acceptance` (builds the sidecar, then the real suite). Parallel with `js-verify`.
- `wasm-build` — `mise run wasm-build`.
- `platform-smoke` — `platform-up` / `platform-smoke` / `platform-down` as three steps so teardown still runs on failure.
- `status` — `if: always()` plus `needs` every leaf.
- It inspects `toJSON(needs)` and fails unless each result is `success`.
- Branch protection should require this job only.
- When the agent adds a leaf, the agent shall add it to `status.needs`.
- If it does not, then the leaf can fail while the required check is green.

`mise run ci-all` mirrors the first four locally (sidecar-first).
The suite runs inside `test` rather than as a second invocation.
`platform-smoke` has no local composite. Run the three platform tasks directly.
The GHA `status` check is stricter than `ci-all` because it includes `platform-smoke`.

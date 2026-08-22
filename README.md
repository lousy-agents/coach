# coach

`coach` is an experimental AI coach designed to help software engineers and autonomous agents build better software. It parses source code syntactically and flags code smells, design issues, and structural metrics.

## Packages

- [`pkg/semantics`](./pkg/semantics) — Deterministic structural analysis of Go, TypeScript, and TSX source bytes (validates syntax, extracts imports, computes branching metrics, and detects constructor-like patterns).
- [`pkg/githubingest`](./pkg/githubingest) — Optional GitHub App-authenticated single-file reader via the GitHub Contents API.

## Run locally (Docker)

[Local Coach quickstart](./docs/pilot-local-quickstart.md): Docker Compose API + worker, smoke test without GitHub, optional local Qwen (`qwen3.5:4b` / `qwen3.5:4b-mlx`), optional scan of a GitHub.com repo without cloning it. Local knobs: [`.env.example`](./.env.example) → `.env`.

For deterministic signals on a local checkout with no Docker, see [`coach codesignal`](#coach-codesignal-cli-preview).

Working in a Claude Code cloud session? See
[Claude Code cloud development](./docs/development/claude-code-cloud.md) for how
the toolchain is bootstrapped and what to do when it fails.

---

## Installation

### Go Packages
To install the Go packages:

```sh
go get github.com/lousy-agents/coach/pkg/semantics

# Optional: for GitHub App content ingestion
go get github.com/lousy-agents/coach/pkg/githubingest
```

### JavaScript / TypeScript Bindings (`@lousy-agents/coach-semantics`)
The JS/TS bindings are currently packaged for Node.js (ESM-only). 

> [!NOTE]
> Because `coach` is in an active experimental phase, the npm package is not yet published to the public npm registry. To consume it, you must clone the repository and build the library locally:

> [!IMPORTANT]
> **Build Prerequisites:** Because the package compiles its underlying parser engine locally during installation, you must have:
> - **Node.js** (>= 20)
> - **Go** (>= 1.25.0)

1. **Clone and Build:**
   ```sh
   git clone https://github.com/lousy-agents/coach.git
   cd coach/js/semantics
   npm install   # Compiles the underlying parser engine and packages TS code
   ```

2. **Link or Reference the Package:**
   In your client application, add the local path as a dependency:
   ```sh
   cd ~/your-app
   npm install /path/to/coach/js/semantics
   ```

---

## `coach codesignal` CLI Preview

`cmd/coach` provides a `codesignal` subcommand: a local, deterministic preview of `pkg/codesignal` you can run directly against a Git checkout, without any GitHub App, worker, or model/LLM configuration.

Download the latest `coach` binary for your platform from the [GitHub Releases page](https://github.com/lousy-agents/coach/releases). Each tagged release publishes archives for macOS (Apple silicon `darwin_arm64` and Intel `darwin_x86_64`), Linux (`linux_x86_64`), and Windows (`windows_x86_64`), a `checksums.txt` file, and a cosign signature bundle.

```sh
ARCH=darwin_arm64  # or darwin_x86_64, linux_x86_64, windows_x86_64
EXT=tar.gz; [ "$ARCH" = windows_x86_64 ] && EXT=zip

curl -LO https://github.com/lousy-agents/coach/releases/latest/download/coach_${ARCH}.${EXT}
curl -LO https://github.com/lousy-agents/coach/releases/latest/download/checksums.txt
curl -LO https://github.com/lousy-agents/coach/releases/latest/download/checksums.txt.bundle

# Verify the checksums file was signed by this repository's release workflow
cosign verify-blob \
  --bundle checksums.txt.bundle \
  --certificate-identity-regexp '^https://github.com/lousy-agents/coach/.github/workflows/release.yml@refs/tags/v.*$' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt

# Verify the archive against the signed checksums
shasum -a 256 -c --ignore-missing checksums.txt

# Extract the binary
if [ "$EXT" = zip ]; then
  unzip coach_${ARCH}.${EXT}
else
  tar -xzf coach_${ARCH}.${EXT}
fi
```

Move the extracted `coach` binary somewhere on your `PATH`.

> [!NOTE]
> From a local clone, you can still build/install it with:
> ```sh
> go install ./cmd/coach
> ```

**Prerequisites:** a local Git checkout and the `git` executable in `PATH`. Building from source also requires Go 1.25+ (matching `go.mod`).

### Usage

Run it from inside a Git worktree, pointing `--base` at the revision you want to diff against:

```sh
coach codesignal --base <ref>
```

- `--base` is required; it can be any ref Git can resolve to a commit (a branch, tag, or SHA).
- `--format` defaults to `text`; pass `--format=json` for machine-readable output.

**Text example** (`coach codesignal --base <ref>`), after a commit adds a function that mutates a caller-owned pointer:

```
files analyzed: 1, active signals: 1, diagnostics: 0
path: config.go
line: 12
lifecycle: introduced
changed: true
evidence: cfg.Timeout
why it matters: Mutating a caller-owned input can create behavior that is not visible from the function signature, make outcomes dependent on call ordering, introduce temporal coupling, make tests and local reasoning more difficult, and surprise callers that expect an input to remain unchanged.
recommendation: Return a copy instead of mutating the caller's value, or document/rename this function to make the in-place mutation explicit.
```

**JSON example** (`coach codesignal --base <ref> --format=json`), for the same change:

```json
{
  "schema_version": "1",
  "scope": { "revision": "3474e2c...", "base": "ece3690..." },
  "summary": {
    "files_analyzed": 1,
    "files_with_diagnostics": 0,
    "active_signals": 1,
    "introduced_signals": 1,
    "existing_signals": 0,
    "resolved_signals": 0,
    "baseline_signals": 0
  },
  "signals": [
    {
      "id": "sig_88ec28c6...",
      "fingerprint": "fp_dcf2afc2...",
      "rule_id": "state.hidden_input_mutation",
      "rule_version": "1",
      "kind": "hidden_input_mutation",
      "category": "state_management",
      "severity": "medium",
      "confidence": "medium",
      "lifecycle": "introduced",
      "changed": true,
      "path": "config.go",
      "subject": "ApplyDefaults:cfg",
      "location": { "start_byte": 137, "end_byte": 148, "start_row": 11, "start_col": 1, "end_row": 11, "end_col": 12 },
      "evidence": "cfg.Timeout",
      "why_it_matters": "Mutating a caller-owned input can create behavior that is not visible from the function signature, make outcomes dependent on call ordering, introduce temporal coupling, make tests and local reasoning more difficult, and surprise callers that expect an input to remain unchanged.",
      "recommendation": "Return a copy instead of mutating the caller's value, or document/rename this function to make the in-place mutation explicit.",
      "suggested_skill": "refactor-hidden-mutation",
      "provenance": { "producer": "semantics", "finding_kind": "mutates_input" }
    }
  ]
}
```

This example's `scope` and top-level object omit `repository`, `applied_scope`, `baseline`, `diagnostics`, `coverage`, and the `project_*` fields because they're empty for this run: those specific fields carry `omitempty` (they are not the only `omitempty` fields — `signals`, `revision`, and `base` do too, and are simply non-empty here; see `pkg/codesignal/report.go` and `input.go`), so an empty collection or unset value disappears from JSON output rather than appearing as `[]`/`{}`/`""`. [Issue #269](https://github.com/lousy-agents/coach/issues/269) tracks changing that so empty collections are always present; until it lands, don't read this example's key set as the complete set of keys `Report` can ever emit. `summary`'s fields (including `baseline_signals`) carry no `omitempty` tag and are always present, even at `0` — a zero count there is presence, not omission, and should not be read as evidence for the same `omitempty` behavior described above.

### Exit status

- `0` — the CLI completed its analysis, regardless of whether any signals or diagnostics were reported. A quiet report with only diagnostics and zero signals is still a normal, exit-0 outcome.
- `1` — an operational failure: the working directory is not a Git worktree, `--base` cannot be resolved, `git` is not found in `PATH`, or an internal analysis step fails. One actionable message goes to stderr; nothing is written to stdout.
- `2` — a usage error: `--base` is missing, `--format` is not `text` or `json`, or (with `--project-config`) the config document is missing, unreadable, or fails schema validation (`project_config_invalid`). Usage guidance goes to stderr; nothing is written to stdout.
- `3` — reserved for a valid `--project-config` naming a `--project-language` with no registered project-analysis backend at all (`project_backend_unavailable`; see [Configured layer violations](#configured-layer-violations---project-config) below). It's currently unreachable through the real CLI: `--project-language` accepts only `go` and `typescript`, both of which have registered backends, and any other value is rejected as a usage error (exit `2`) before backend dispatch ever happens. This exit code stays defined for a future language whose backend is registered but unavailable. A missing or failed TypeScript sidecar is a distinct, exit-`0` condition; see below.

A `--project-config` load/validation failure (exit `2`, `project_config_invalid`) is treated exactly like any other usage error: it never writes a report to stdout, even in `--format json`. The stderr message identifies the `--project-config` path, the analyzed revision, and a specific reason — not found at that revision, not valid JSON, or a schema validation failure. If the path exists in your worktree but not at the analyzed revision (e.g. a `--suggest-project-config --output` candidate that hasn't been committed yet, see below), the message says so explicitly and names committing the file as the fix. This is distinct from `project_backend_unavailable` (exit `3`), which keeps its own report-on-stdout behavior unchanged.

### `schema_version`: "1" vs "2"

`schema_version` is chosen per invocation by whether `--project-config` is supplied, not by the `coach` binary's release version:

- **No `--project-config`** → `schema_version: "1"`. A consumer may rely on `scope`, `summary`, and `signals`; the `project_*` fields are never present.
- **A valid `--project-config` that reaches a registered backend** → `schema_version: "2"`. In addition to `scope`, `summary`, and `signals`, a consumer may rely on `project_changes`, `project_summary`, and `project_coverage` being present.

Because a `--project-config` failure now writes nothing to stdout, there is no schema-1 or schema-2 document to distinguish on that path — the failure is detectable from the exit status (`2`) alone.

### Scope and limitations

- **Advisory only.** It surfaces deterministic structural signals; it does not judge correctness or block anything on its own.
- **Go, TypeScript, and TSX only.** Changed files in other languages are skipped (with an `unsupported_language` diagnostic).
- **Does not execute code.** All analysis is static, over source bytes read via `git show`.
- **No runtime proof, cross-file analysis only by explicit opt-in.** By default it cannot prove a defect exists or trace causality across files; each file is analyzed independently. An explicit `--project-config` (see [Configured layer violations](#configured-layer-violations---project-config) below) opts into one narrow, advisory exception: checking configured Go or TypeScript import edges against configured architectural layers.
- **Local-only, zero external configuration.** It never contacts GitHub, a model/LLM API, or any other network service — see `internal/codesignalcli/dependencies_test.go`'s `TestNoExternalDependencies` for the enforced boundary (no `net/http`, no GitHub client, anywhere in its dependency graph).
- **Renames and copies are not analyzed for lifecycle continuity.** A renamed or copied file produces an `unsupported_change_type` diagnostic instead of being diffed against its old path.

### Signal lifecycle and `changed`

Every signal carries a `lifecycle`, computed by comparing HEAD against the resolved merge-base:

- `introduced` — present at HEAD, not present at the merge-base.
- `existing` — present at both HEAD and the merge-base.
- `resolved` — present at the merge-base, not present at HEAD.
- `unknown` — the merge-base side of the file could not be analyzed (e.g. it had a syntax error), so lifecycle can't be determined.

`changed` is a separate boolean: it's `true` when the signal's HEAD location overlaps a line the diff marks as changed, independent of `lifecycle` — an untouched, pre-existing signal in a file that had other lines changed is `changed: false`.

### Locations: 0-based JSON vs 1-based text

JSON output reports `Location` fields (`start_row`, `start_col`, etc., see `pkg/semantics/result.go`) as 0-based, matching Tree-sitter's own convention. Text-mode output's `line:` field adds 1 to `start_row` for a human-friendly 1-based display line.

### `--suggest-project-config`: opt-in project-config candidate

`coach codesignal --baseline --suggest-project-config` is a separate, opt-in mode that discovers Go module/workspace roots at HEAD and prints a minimal `--project-config` **candidate** for you to review and commit yourself — it never reads or writes an actual `--project-config` file, and it never runs any project-analysis backend. It resolves HEAD once, walks an immutable Git snapshot for `go.mod`/`go.work` files (never the worktree, so uncommitted or ignored files never affect the result), and emits strict schema-1 JSON containing only `roots` (never `layers`, `forbidden_imports`, or `required_layer` — those require a human decision this mode does not make; nor `source_sink_pack`, which is a reserved field with no effect on evaluation today, so there is nothing for this mode to suggest for it).

```sh
coach codesignal --baseline --suggest-project-config
```

- Requires `--baseline`; it cannot be combined with `--base`, `--project-config`, `--project-language`, `--format`, `--scope`, `--build-target`, or a positional argument — any of those combinations is rejected outright rather than given a precedence.
- The candidate JSON is written to stdout (2-space indent, one trailing newline); pass `--output <path>` to write it to a repository-relative file instead (create-only — it fails if the target already exists in any form). Either way, exactly one UTF-8 newline-delimited JSON (NDJSON) diagnostic/provenance object — compact, single-line, one trailing newline — is written to stderr describing the resolved revision, the roots considered, and root-discovery coverage.
- **This is a candidate only.** Nothing consumes it automatically: review it, add any `layers`/`forbidden_imports`/`required_layer` policy you want (`source_sink_pack` is accepted but currently reserved and has no effect — see [Configured layer violations](#configured-layer-violations---project-config) below), and pass the result to `--project-config` yourself.
- **Commit before use.** `--project-config` reads at the analyzed revision, not your worktree (see [Configured layer violations](#configured-layer-violations---project-config) below), so a candidate written with `--output <path>` must be committed before a subsequent `--project-config <path>` run can read it — an uncommitted candidate fails with exit `2` and a stderr message naming commit as the remedy.
- **`roots` alone yields facts only.** A candidate straight from this mode has `roots` but no `layers`/`forbidden_imports`: fed into `--project-config` as-is, it produces a real schema-2 report with `project_coverage`, but zero layer findings, because there is no layer policy yet to violate. Add `layers`/`forbidden_imports` yourself (see below) once you're ready to enforce one.

**Candidate example** (stdout, for a repository with a root module and one nested module):

```json
{
  "schema_version": "1",
  "roots": [
    ".",
    "services/payments"
  ]
}
```

**Diagnostic/provenance example** (stderr, one NDJSON line — shown compact, exactly as emitted):

```json
{"diagnostic_version":"1","kind":"project_config_suggestion","revision":"3474e2c...","heuristic_version":"go-project-config-roots@1","roots_considered":[".","services/payments"],"coverage":{"phase":"project_config_suggestion","complete":true,"counts":{"files_seen":2,"files_skipped":0,"modules_seen":2,"modules_skipped":0,"roots_emitted":2},"budgets":{"graph_edges":0,"graph_nodes":0,"input_bytes":67108864,"input_files":500000,"stderr_bytes":0,"wall_time_ms":0,"working_set_bytes":0},"diagnostics":[]},"diagnostics":[{"code":"project_config_suggestion_ready","message":"coach codesignal --suggest-project-config: candidate generated successfully"}]}
```

Exit codes are distinct from the standard `codesignal` modes above: `0` on a successfully generated candidate, `2` for a usage error or a discovery problem (no Go modules found, ambiguous/duplicate roots, an invalid or already-existing `--output` path), and `3` if the immutable snapshot can't be read or candidate serialization unexpectedly fails.

### Configured layer violations (`--project-config`)

Pass `--project-config <path>` (a repository-relative path to a JSON document, read at the analyzed revision — not your worktree) to opt into one narrow exception to the file-local default: an import-layer check. It never guesses your architecture — you declare which directories belong to which layer and which layer-to-layer imports are forbidden, and the CLI reports only edges that violate that explicit policy.

```json
{
  "schema_version": "1",
  "roots": ["."],
  "layers": [
    { "name": "handlers", "prefixes": ["pkg/handlers"] },
    { "name": "db", "prefixes": ["pkg/db"] }
  ],
  "forbidden_imports": [
    { "from": "handlers", "to": "db" }
  ]
}
```

- `roots` — repository-relative directories to analyze (for `--project-language go`, your Go module/workspace roots; for `typescript`, the directories the sidecar should collect); required, at least one. `--suggest-project-config` above generates a starting candidate for Go projects.
- `layers` — named layers, each with one or more non-overlapping repository-relative directory prefixes; optional.
- `forbidden_imports` — directed `{"from": "<layer>", "to": "<layer>"}` pairs naming layers declared above; optional. A `from`/`to` that doesn't match a declared layer name is a config error (exit `2`), not a silently-ignored rule.
- `required_layer` — the name of one layer declared above that a statically resolved call path from a handler-shaped source to a pinned database-access sink must pass through; optional, and only wired for `--project-language go` today (see `architecture.layer_bypass` below). A name that doesn't match a declared layer is a config error (exit `2`), matching `forbidden_imports`' undefined-layer rule.
- `source_sink_pack` — accepted and schema-validated (must be `""` or `"builtin-v1"` if set; any other value is a config error, exit `2`). It is currently a reserved field: no backend reads it, so setting it never changes which findings are produced or their content (kind, severity, confidence, lifecycle, evidence, anchors, etc.). It is still part of the raw config bytes hashed into `config_digest`, though, so setting or clearing it changes a project analysis's `config_digest` and therefore every `ProjectChange`'s `id`/`fingerprint` — downstream consumers keyed on those IDs will see churn even though the findings themselves are unchanged. `architecture.layer_bypass`'s source/sink set is pinned in code (`pkg/projectmodel/go_reachability.go`'s `ReachabilitySinkPatterns`), not selected via this field.

Works with both `--baseline` and `--base <ref>`:

```sh
coach codesignal --baseline --project-config project.json
```

```text
Repository Baseline for revision 9022f72... (not a diff comparison)
tracked files discovered: 4, analyzed: 2, unsupported: 2, excluded: 0, unanalyzable: 0, active signals: 1, diagnostics: 0
Project findings:
semantic_key: architecture.layer_violation:pkg/handlers->pkg/db
rule_id: architecture.layer_violation
path: pkg/handlers/handlers.go
line: 3
lifecycle: baseline
changed: false
evidence: pkg/handlers imports pkg/db (layer handlers -> db is forbidden)
machine_evidence.importee: pkg/db
machine_evidence.importer: pkg/handlers
machine_evidence.layer_from: handlers
machine_evidence.layer_to: db
machine_evidence.rule: handlers->db
Project summary: active=1, introduced=0, existing=0, resolved=0, baseline=1

Coverage:
  unsupported: 1 .json files
  unsupported: 1 .mod files

Project coverage: phase=go_model_build, complete=true
  ...
```

**Required intermediate layer (`architecture.layer_bypass`, `--project-language go` only).** Add `required_layer` naming one of the declared layers to additionally check that every statically resolved call path from a `net/http.HandlerFunc`-shaped source to a pinned database-access sink (`*database/sql.DB`'s `Exec`/`ExecContext`/`Query`/`QueryContext`) passes through that layer:

```json
{
  "schema_version": "1",
  "roots": ["."],
  "layers": [
    { "name": "handlers", "prefixes": ["pkg/handlers"] },
    { "name": "service", "prefixes": ["pkg/service"] }
  ],
  "required_layer": "service"
}
```

```text
Project findings:
semantic_key: architecture.layer_bypass:service:example.com/app/pkg/handlers.Handler->(*database/sql.DB).Query
rule_id: architecture.layer_bypass
path: pkg/handlers/handlers.go
line: 10
lifecycle: baseline
changed: false
evidence: example.com/app/pkg/handlers.Handler reaches (*database/sql.DB).Query via a statically resolved path that never passes through required layer "service"
machine_evidence.path: example.com/app/pkg/handlers.Handler->example.com/app/pkg/handlers.directQuery->example.com/app/pkg/handlers.rawQuery->(*database/sql.DB).Query
machine_evidence.required_layer: service
machine_evidence.sink: (*database/sql.DB).Query
machine_evidence.source: example.com/app/pkg/handlers.Handler
why it matters: A statically resolved call path from a handler-shaped source to a pinned sink that structurally never passes through a required intermediate layer means that layer's invariants (validation, authorization, caching, ...) can be silently skipped by this route, even if a separate compliant path also exists.
recommendation: Route the call through the required layer, or add an explicit exception/allowlist entry if the bypass is intentional so it stops surfacing as drift.
path step: example.com/app/pkg/handlers.Handler, confidence: high
  source: pkg/handlers/handlers.go:10
path step: example.com/app/pkg/handlers.directQuery, confidence: high
  source: pkg/handlers/handlers.go:15
path step: example.com/app/pkg/handlers.rawQuery, confidence: high
  source: pkg/handlers/handlers.go:19
path step: (*database/sql.DB).Query, confidence: high
Project summary: active=1, introduced=0, existing=0, resolved=0, baseline=1
```

This `Handler` also calls `service.LoadUser` — a compliant route that does pass through the required `service` layer — but `architecture.layer_bypass` still reports the direct `directQuery`/`rawQuery` route: a coexisting compliant route never suppresses a real bypass witness (see `pkg/projectmodel.LayerBypassWitness`'s doc comment). Only exact, statically resolved, `confidence: high` witnesses are ever reported — an unresolved, dynamic, ambiguous-layer, or budget-truncated path is silently dropped rather than guessed. `--format=json` carries the same shape as `architecture.layer_violation`'s `project_changes[]`/`signals[]` entries, with `path_steps[]` giving each hop's `node_id`, source location, and `confidence`, and `machine_evidence` carrying `source`/`sink`/`required_layer`/`path` instead of `importer`/`importee`/`layer_from`/`layer_to`/`rule`.

**A clean `architecture.layer_bypass` report is not a compliance verdict.** The source and sink registries are narrow and pinned in code, not configurable: a source must match `net/http.HandlerFunc`'s exact signature (a custom router, a gRPC handler, or a framework that wraps the request differently is invisible to this check), and a sink must be a direct call to one of `*database/sql.DB`'s four methods — `Exec`/`ExecContext`/`Query`/`QueryContext` — not a call through `*sql.Tx`, `*sql.Stmt`, `*sql.Conn`, an ORM like `gorm`/`sqlx`, or a repository interface the driver call sits behind, none of which are in the registry. If your codebase's handlers or data access don't match those exact shapes, zero findings means "nothing matched the registry," not "no bypass exists" — the same caveat `architecture.layer_violation` states for its own coverage below applies here with an even narrower net.

When the underlying search's own coverage is incomplete, a `project_layer_bypass_coverage_incomplete` diagnostic appears in the report, a witness the search already found is still reported rather than hard-suppressed, and every project finding's lifecycle in that report degrades to `unknown` instead of a definitive `resolved`/`introduced`/`existing`/`baseline` claim. Each degraded finding's own `evidence` says "see the project_lifecycle_indeterminate diagnostic for why" — so you don't have to separately cross-reference the report's `Diagnostics:` section to find out.

**TypeScript example.** The same `layers`/`forbidden_imports` schema shown above works unmodified with `--project-language typescript` (`required_layer` is Go-only — see above) — only the CLI flag changes. A TypeScript project additionally needs a discoverable `tsconfig.json` (the sidecar uses it to resolve relative import specifiers into file-addressed edges) and the vendored sidecar binary described below.

```json
{
  "compilerOptions": {
    "module": "esnext",
    "moduleResolution": "bundler",
    "target": "es2022"
  },
  "include": ["pkg/**/*.ts"]
}
```

```sh
coach codesignal --baseline --project-config project.json --project-language typescript
```

```text
Repository Baseline for revision c67e28c... (not a diff comparison)
tracked files discovered: 4, analyzed: 2, unsupported: 2, excluded: 0, unanalyzable: 0, active signals: 1, diagnostics: 0
Project findings:
semantic_key: architecture.layer_violation:pkg/handlers/handlers.ts->pkg/db/db.ts
rule_id: architecture.layer_violation
path: pkg/handlers/handlers.ts
line: 1
lifecycle: baseline
changed: false
evidence: pkg/handlers/handlers.ts imports pkg/db/db.ts (layer handlers -> db is forbidden)
machine_evidence.importee: pkg/db/db.ts
machine_evidence.importer: pkg/handlers/handlers.ts
machine_evidence.language: typescript
machine_evidence.layer_from: handlers
machine_evidence.layer_to: db
machine_evidence.rule: handlers->db
why it matters: Import edges that cross a configured layer boundary erode the architecture the team claims to follow, so drift compounds across packages that look fine in isolation.
recommendation: Move the dependency behind an allowed boundary (interface in the lower layer, invert the import, or relocate the shared type), or update the explicit layer policy if the edge is intentional.
Project summary: active=1, introduced=0, existing=0, resolved=0, baseline=1

Coverage:
  unsupported: 2 .json files

Project coverage: phase=ts_sidecar_build, complete=true
  count: files_analyzed=2
  count: files_seen=3
  count: projects_analyzed=1
  count: tsconfig_count=1
  budget: timeout_ms=60000
```

Three differences from the Go example above are structural, not incidental. `machine_evidence` carries an extra `language: typescript` entry (Go's output has no such key). The project coverage phase is `ts_sidecar_build` rather than `go_model_build`, and each backend emits its own disjoint set of `count:`/`budget:` keys under that phase (only `count: files_seen` is shared) — the Go example's `...` above stands in for Go's own set, not TypeScript's. And addressing granularity differs: Go's evaluator groups findings per (importer package directory, importee package directory) pair, so `semantic_key`, `evidence`, and `machine_evidence.importer`/`machine_evidence.importee` use package-directory addressing (`pkg/handlers`, `pkg/db`), while the TypeScript evaluator is file-addressed instead, so those same fields carry full file paths (`pkg/handlers/handlers.ts`, `pkg/db/db.ts`) and findings group per violating file pair rather than per package pair — forced by file-level vs. package-level import-edge identity in each backend, not an artifact of the filenames chosen for this example.

The `why it matters:`/`recommendation:` lines are true abridgement, not a fourth structural difference: both evaluators reuse the same underlying constants, and the Go example above simply leaves those two lines out rather than marking them with `...` (its one `...` is under `Project coverage:`, standing in for Go's coverage keys, not for these lines).

`--format=json` adds `project_changes` (each with a `machine_evidence` map, `primary_anchor`, and — when the same violating layer pair has more than one import site — `related_locations`), `project_summary`, and `project_coverage` alongside the existing `signals` array; `--base <ref>` classifies each finding's `lifecycle` as `introduced`, `existing`, or `resolved` instead of `baseline`, following the same semantics as file-local signals. Omitting `--project-config` leaves every existing example above completely unchanged (byte-identical `schema_version: "1"` output, no `project_*` fields).

Each `signals[]` entry for a project-origin finding also carries the same optional `machine_evidence`, `related_locations`, `path_steps`, and `coverage_refs` fields as its `project_changes[]` counterpart; these fields are omitted for file-local findings. This lets automation that reads only `signals[]` get the same structured evidence as the text report, without also reading `project_changes[]`. `project_changes[]` still exposes the same fields for consumers that want the project-specific raw view.

This check is **advisory, coverage-honest, and approximate**: it reports zero findings (never a guess) for any import edge it cannot resolve or that doesn't map to a configured layer, retains findings at `lifecycle: unknown` rather than a false `introduced`/`existing`/`resolved` claim when project coverage is incomplete, and matches layers by repository-relative directory prefix over Go import facts (the `go` backend) or TypeScript import facts (the `typescript` backend) — not a full type or build-graph analysis. `--project-language typescript` is a real, registered backend: layer violations are evaluated from file-addressed import edges produced by a pinned sidecar binary at `js/semantics/bin/coach-ts-project-sidecar`, resolved relative to the analyzed repository's own root (not this `coach` binary's install location — the analyzed repository must vendor the sidecar at that exact path). If the sidecar is missing, crashes, or times out, that is reported as a `project_backend_unavailable` entry in `project_coverage.diagnostics` with `complete: false` and exit `0` — never exit `3`. Exit `3` is reserved: currently unreachable, because `--project-language` accepts only `go` and `typescript` (both registered) and any other value is a usage error (exit `2`); it remains defined for a future language whose backend is registered but unavailable.

**Absence is not a compliance verdict.** A report with no `architecture.layer_violation` finding is never proof the codebase follows any particular architecture: it can mean the edge doesn't violate your configured policy, the edge isn't covered by any layer you declared, or project coverage was incomplete for that revision — check `project_coverage`/`Project coverage:` before treating a clean report as an architectural guarantee.

`pkg/codesignal.EvaluateGoLayerBypass` maps a `pkg/projectmodel.BuildGoLayerBypass` run's high-confidence witnesses — a statically resolved call path from a source to a sink that structurally never passes through a required layer — onto `architecture.layer_bypass` findings, following the same suppression rules as `architecture.layer_violation`: only exact high-confidence witnesses are emitted, path search is shortest-first, and an incomplete search still emits any witnesses it did find alongside a coverage-incomplete diagnostic rather than a silent "no bypass" claim. Both are wired into `coach codesignal --project-config` via the `required_layer` field documented above — but for `--project-language go` only; see the worked example above. `pkg/projectmodel` separately has library-only support for bounded Go possible-call-reachability facts (a pinned static call graph plus deterministic source-to-sink path search, both bounded by budgets and approximate — direct, statically resolved calls only, not full points-to) via `BuildGoReachability`, which stays unwired. `BuildTypeScriptLayerBypass`/`EvaluateTypeScriptLayerBypass` are the TypeScript analogs of the now-wired Go pair — built from the same sidecar round trip as `--project-language typescript`'s import-edge analysis and sharing the `architecture.layer_bypass` rule ID and every suppression rule above, with a `machine_evidence.language: typescript` entry and a distinct `algorithm_version` (`ts-layer-bypass-registry@1` vs. `go-layer-bypass-registry@1`) as the only externally visible differences, matching `architecture.layer_violation`'s existing Go/TypeScript split — but they remain library-only: CLI wiring is scoped to the Go backend, so `required_layer` has no effect under `--project-language typescript` today.

`BuildTypeScriptReachability` is the TypeScript analog of `BuildGoReachability`, producing `possible_call_reachability` facts over the same sidecar round trip — but not the same facts: the TS walk follows a route handler's body only one level deep, so a handler → service → sink chain that isn't a direct call yields no fact, only a `ts_reachability_local_call_not_followed_gap` coverage diagnostic, where Go's BFS is unbounded. `Sources` is correspondingly narrower too, listing only the sources that actually produced a resolved fact rather than every registry source function identified in the snapshot. Unlike `architecture.layer_bypass`, these facts carry no `ProjectChange`/`machine_evidence` shape at all — they map onto `ProjectFact` (facts-only: a reachability fact never becomes a `Signal` or `ProjectChange`) via `pkg/codesignal.ReachabilityProjectFacts`, which the caller invokes once per backend with a `language` argument ("go" or "typescript") that lands in the resulting facts' `provenance.language` field. That field is `ProjectFact`'s only cross-language provenance signal, since `ProjectFact` has neither a `rule_id` nor a `machine_evidence` map to carry an equivalent key.

None of `BuildGoReachability`, `BuildTypeScriptReachability`, `BuildTypeScriptLayerBypass`, `EvaluateTypeScriptLayerBypass`, or `ReachabilityProjectFacts` are wired into `coach codesignal` today: no flag builds them, and no report field surfaces call-graph, reachability, or TypeScript layer-bypass facts through the CLI yet. `BuildGoLayerBypass`/`EvaluateGoLayerBypass` are the one exception, wired as described above.

---

## `pkg/semantics` Quickstart

`pkg/semantics` operates purely on raw bytes, meaning you don't need a file system to analyze code.

```go
package main

import (
	"context"
	"fmt"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func main() {
	// Initialize the analyzer
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		panic(err)
	}

	sourceBytes := []byte(`
		package main
		func Hello() string {
			return "world"
		}
	`)

	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "hello.go",
		Language: semantics.LanguageGo,
		Content:  sourceBytes,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("Status:", result.ParseStatus) // "ok"
	for _, f := range result.Findings {
		fmt.Printf("Finding: %s (Location: %v)\n", f.Kind, f.Location)
	}
}
```

### Error Handling

If a file has syntax errors, `AnalyzeBytes` returns a partial `*Result` along with an error wrapped as `ErrSyntax`. You can extract detailed syntax issues using `errors.As`:

```go
import "errors"

// ...

result, err := analyzer.AnalyzeBytes(ctx, input)
if errors.Is(err, semantics.ErrSyntax) {
	var syntaxErr *semantics.SyntaxError
	if errors.As(err, &syntaxErr) {
		for _, issue := range syntaxErr.Issues {
			fmt.Printf("Syntax issue: %s at %v\n", issue.Kind, issue.Location)
		}
	}
}
```

Other sentinel errors:
- `ErrEmptyContent` — The provided input content is empty.
- `ErrUnsupportedLanguage` — The file extension or language is not supported.
- `ErrFileTooLarge` — The source file exceeds limits.
- `ErrBinaryContent` — The file appears to be binary.
- `ErrParseFailure` — General Tree-sitter parsing failure.

---

## Coaching Findings

Beyond parsing metrics, `coach` analyzes code constructs to flag specific design issues. The most common is `mutates_input`.

### `mutates_input` Finding
`mutates_input` flags functions or methods that mutate a parameter in place. This can lead to hard-to-debug "spooky action at a distance" bugs.

For example, this Go function mutates its input in place:
```go
func ApplyDefaults(cfg *Config) {
    cfg.Timeout = 30 * time.Second // Mutates the caller's Config
}
```

This generates a finding detailing the mutation:
```json
{
  "kind": "mutates_input",
  "name": "ApplyDefaults:cfg",
  "location": { "start_byte": 26, "end_byte": 36, "start_row": 1, "start_col": 4, "end_row": 1, "end_col": 14 },
  "confidence": "medium",
  "evidence": "cfg.Timeout",
  "recommendation": "Return a copy instead of mutating the caller's value, or document/rename this function to make the in-place mutation explicit.",
  "suggested_skill": "refactor-hidden-mutation"
}
```

A safer alternative returns the updated value:
```go
func WithDefaults(cfg Config) Config {
    cfg.Timeout = 30 * time.Second
    return cfg
}
```

> [!TIP]
> **Syntactic Constraint:** The `mutates_input` checker is purely syntactic and conservative. It does not perform interprocedural analysis or follow reference aliases across scopes.

### Other Supported Findings
- `constructor_func` (Go) — Flags factory or constructor functions.
- `pointer_return` (Go) — Flags functions returning a pointer to a struct.
- `tight_coupling` (TS/TSX) — Flags class/object patterns with high coupling.
- `toctou_check_then_act` (Go, TS/TSX) — Flags a filesystem existence/state check whose success path later acts on the identical path (CWE-367).

---

## JavaScript / TypeScript Quickstart

Here is how you can perform syntactic analysis in a Node.js project:

```ts
import { readFile } from "node:fs/promises";
import { createAnalyzer, SemanticsSyntaxError } from "@lousy-agents/coach-semantics";

// Initialize the analyzer child process
const analyzer = await createAnalyzer();

try {
  const content = await readFile("widget.ts");
  const result = await analyzer.analyzeBytes({
    path: "widget.ts",
    language: "typescript", // "go", "typescript", or "tsx"
    content: content,
  });

  console.log("Status:", result.parse_status);
  console.log("Metrics:", result.metrics);
  console.log("Findings:", result.findings);
} catch (err) {
  if (err instanceof SemanticsSyntaxError) {
    // Access the partial Result containing syntax errors
    console.log("Syntax Errors:", err.partialResult.syntax_errors);
  } else {
    console.error("Analysis failed:", err);
    throw err;
  }
} finally {
  // Always clean up the analyzer process when finished
  analyzer.dispose();
}
```

Thrown errors inherit from `SemanticsError` and carry a `kind` string:
- `"syntax"`, `"empty_content"`, `"unsupported_language"`, `"file_too_large"`, `"binary_content"`, `"parse_failure"`, `"invalid_options"`, `"canceled"`, `"internal"`, `"backend_unavailable"`.



## Stability Guarantees

- **JSON Stability:** The output structure and its `snake_case` JSON field names are completely frozen.
- **API Stability:** Because `coach` is currently pre-1.0, the core JSON structure and sentinel errors are stable, but other parts of the API surface may evolve.

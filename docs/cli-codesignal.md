# `coach codesignal` CLI contract

Machine-oriented contract for the local `coach codesignal` CLI and the
analyzer libraries. Customer onboarding lives in [README.md](../README.md).

This documents shipped behavior. It is not a gate and not a 1.0 promise.

## Commands

```text
coach <command> [flags]
coach --help
coach --version
```

The only subcommand is `codesignal`.

```text
coach codesignal (--base <ref> | --baseline) [--format text|json] [--scope production|all] [--min-severity high|medium|advisory|low] [--top N] [--build-target <package>] [--project-config <path>] [--project-language go|typescript] [--no-interactive] [--fail-on-incomplete-coverage]
coach codesignal --baseline --suggest-project-config [--output <path>]
coach codesignal --baseline --suggest-project-config --project-language typescript [--output <path>] [--no-interactive]
coach codesignal --baseline --check-project --project-language typescript [--project-config <path>] [--format text|json]
coach codesignal --baseline --prepare-compiler --project-language typescript [--project-config <path>] [--no-interactive]
```

`--base` and `--baseline` are mutually exclusive. `--check-project`,
`--suggest-project-config`, and `--prepare-compiler` are not scans (except
that suggest/prepare still require `--baseline`).

| Flag | Default | Notes |
| --- | --- | --- |
| `--base <ref>` | | Diff HEAD against any Git-resolvable ref. |
| `--baseline` | | Scan every tracked Go/TS/TSX file at HEAD. |
| `--format` | `text` | `text` or `json`. |
| `--scope` | `production` | `production` or `all`. |
| `--min-severity <level>` | none | Render only signals at or above `high`, `medium`, `advisory`, or `low` (ranked `high` > `medium` > `advisory` > `low`). No floor by default. Any other value is a usage error: exit `2`. See [Narrowing the rendered report](#narrowing-the-rendered-report). |
| `--top <N>` | none | Render only the first `N` signals in report order, after any `--min-severity` floor. No cap by default. `N` must be a positive integer; anything else is a usage error: exit `2`. |
| `--build-target <pattern>` | empty | Go package pattern for production reachability. Silent no-op under `--scope all`. |
| `--project-config <path>` | empty | Repository-relative path at the **analyzed revision**. Enables `schema_version: "2"`. |
| `--project-language` | `go` | `go` or `typescript`. With no `--project-config`, a language flag on a scan is a silent no-op. |
| `--suggest-project-config` | | Go: print a human-reviewed candidate (never auto-applied, never invents layers). TypeScript: interactive TTY authoring; writes nothing until confirmed. |
| `--output <path>` | stdout | Create-only path for a suggest candidate. |
| `--check-project` | | Read-only TypeScript readiness. Exits `0` on gaps. Not a scan. |
| `--prepare-compiler` | | Interactive consented **mise-only** TypeScript compiler setup. Requires a TTY and an attended session; refuses otherwise. Also refuses, naming the gap, for a runtime-boundary gap (`node_missing`/`node_unsupported`) — the same restriction the scan path's own compiler-setup offer applies, so the two cannot disagree. It is narrower than the scan's offer, which also runs the project package manager: a repository whose only executable choice is `project_package` is never handed this command as remediation, because it would exit `0` reporting nothing to set up. |
| `--no-interactive` | `false` | Treat the invocation as unattended even with a controlling terminal attached. On a scan, the interactive compiler-setup and guided-policy-authoring offers are skipped, falling through to the same message-only remediation a piped invocation gets; the TypeScript form of `--suggest-project-config`, and `--prepare-compiler`, refuse with exit `2` instead of prompting (the Go form of `--suggest-project-config` never prompts, and rejects the flag as a usage error). Also triggered by a non-empty `CI` environment variable, so a pty-allocating runner cannot hang on an unanswered prompt. |
| `--fail-on-incomplete-coverage` | `false` | Exit `3` when required project coverage (model or bypass phase, on any analyzed side) is incomplete — including when the project backend is unavailable. The report is always written to stdout first. Valid only on scan modes (`--base` / `--baseline`). |

Requires `git` on `PATH`. Analyzes **committed Git objects**, not the dirty
worktree. TypeScript project mode also inspects worktree `package.json` /
`node_modules` / `mise.toml` for compiler and Node readiness.

`coach codesignal` takes no positional operands. An unexpected positional
argument is a usage error: exit 2, usage on stderr, and nothing on stdout.

## Exit codes

| Code | When |
| --- | --- |
| `0` | Completed analysis, readiness report, successful suggest/prepare, or `--help` / `--version`. Signals, and narrowing the report with `--min-severity` or `--top`, do not change this. |
| `1` | Operational: not a git repo, missing `git`, unresolvable `--base`, empty repo, I/O. |
| `2` | Usage (including an invalid `--min-severity` or `--top` value), invalid `--project-config` (empty stdout, stderr names the path/revision), unsupported `--project-language`, or a TypeScript **scan** that cannot resolve a supported compiler or host Node (empty stdout except the policy-candidate document on the guided-authoring path; one or more stderr lines). |
| `3` | Required project coverage (model or bypass, on any analyzed side) is incomplete **and** `--fail-on-incomplete-coverage` was supplied — the report is still written to stdout. Also: `--prepare-compiler` operational failure resolving the baseline revision or computing readiness (a deliberate exception to the scan contract's `1`, kept because the flag follows suggestion mode's table). |

`--check-project` exits `0` so you inspect the payload; do not treat that
status as a gate.

On a controlling terminal, a TypeScript scan whose `--project-config` names a
policy that was never committed opens a guided-authoring session instead of
only printing a remediation line. With
`--format json` this session's exit-2 output is a JSON object carrying
`"schema_version": "1"` and `roots`, not a CodeSignal report — the absent
`scope`/`signals`/`coverage` keys distinguish it. A scan cannot pass
`--output`, so the approved candidate is written to **stdout** and no file is
created: save it to the `--project-config` path yourself, review it, commit
it, then rerun.

Prompts and setup transcripts go to stderr. Redirecting stderr away from your
terminal (`2>run.log`) therefore hides the prompts while the session still
waits for an answer; pass `--no-interactive` when you redirect stderr and do
not intend to answer.

Every command Coach appends as remediation on that exit-2 path is itself
interactive, so each is printed prefixed with `on a terminal:` — running one
from the same piped context that printed it exits `2` without making progress.
With no policy, `checks.project_shape` and `checks.package_manager` report
`not_checked` rather than guessing from the worktree root in place of the
roots a policy would have selected, so every other gap readiness reports
alongside a missing policy is already independent of that missing policy —
the line simply names the `--check-project` rerun with no hedge about
whether committing a policy would change it.

On a controlling terminal, a TypeScript scan with a committed policy whose
compiler check fails offers interactive setup covering everything
`--prepare-compiler` runs standalone, plus the project package manager. It
names the failing check and states that a confirmed, successful choice makes
Coach recheck readiness and resume the same scan, then shows a menu of up to
four choices (`project_package`, `project_mise`, `global_mise`, `cancel`),
each labelled with what it would touch. A choice that would open only to
report nothing to do is withheld, and a gap offering nothing installable never
opens a prompt — it reports why each choice was ruled out instead. A
`project_package` install whose policy selects more than one manifest context
is withheld too: one consented command runs in one directory, and the
readiness rerun would still report the gap.
Selecting `project_package` shows a preview (executable, arguments, working
directory, expected on-disk changes, network disclosure, script-suppression
policy, and `SetupPreviewTimeout` = 5 minutes) and requires a separate,
single-use `confirm`/`install` answer before running; any other answer, or a
second leftover answer, cancels or is ignored. Cancelling or failing exits 2
with no report and (on failure) no change to the repository's tracked files.
Succeeding reruns the full readiness check and continues the same scan to a
report only if that rerun reports no gap; the offer runs at most once per
invocation even then.

Without a controlling terminal, a compiler gap whose only genuinely
executable setup choice is `project_package` cannot be resolved by
`--prepare-compiler` — it runs mise scopes only — so the scan instead
appends `on a terminal: coach codesignal --baseline --project-config <path>
--project-language typescript -- offers project-package setup`, naming the
same scan invocation rather than a standalone subcommand: rerunning it on a
terminal is what opens the combined setup offer above.

## Diff-mode caveats

- Dirty / uncommitted files are ignored as analysis input. A TypeScript project scan emits `worktree_report_reflects_committed_head` when relevant worktree changes exist; the report still applies to committed HEAD. Supported-language file-local dirt uses `worktree_changes_not_analyzed`. Unsupported dirt uses `worktree_not_clean`. A status-check failure uses `worktree_status_check_failed`.
- Diff mode passes `--find-renames` (default 50% similarity) and
  `--find-copies-harder` (unmodified files are copy-source candidates) to
  `git diff --name-status`. Rename (`R`) and copy (`C`) new paths are
  analyzed. A pure git rename or copy without old-path continuity yields
  `lifecycle: unknown` plus a `continuity_not_determined` path diagnostic.
  Old-path continuity is not established. Adopters who only watch
  `introduced_signals` shall treat that path as incomplete, not green.
- Statuses the CLI does not analyze (`T`, `U`, `X`, `B`) emit
  `unsupported_change_type`, increment `summary.files_unanalyzed` and
  `summary.files_with_diagnostics`, and qualify the text all-clear.
- Added (`A`) files are `lifecycle: introduced` and increment `introduced_signals`.
- `lifecycle` is relative to the merge-base (`introduced` / `existing` / `resolved` / `unknown`). `unknown` is residual classification, not "new in this diff." An unanalyzable merge-base carries a `base_read_failed`, `base_syntax_errors`, or `base_analysis_failed` diagnostic. A rename or copy without old-path continuity is also `unknown` and carries `continuity_not_determined`; that is not a green `introduced_signals` miss.

## Lifecycle and counters

Every entry in `signals[]` is counted in exactly one of `introduced_signals`,
`existing_signals`, `resolved_signals`, `baseline_signals`,
`unknown_signals`. `summary.unknown_signals` counts `unknown` lifecycle
signals. `active_signals` is `len(signals)` after the include-resolved
filter, when no narrowing flag is set. That relationship holds for both
`--base` and `--baseline`:

- `--base` includes `resolved` in `signals[]` by default, so `active_signals`
  includes them. That field is not "problems present at HEAD".
- `--baseline` does not include `resolved`. Signals there are `baseline`.

A consumer who wants problems present at HEAD should sum `introduced` +
`existing` + `unknown` (and `baseline` under `--baseline`), not trust
`active_signals` alone in diff mode.

With `--min-severity` or `--top`, `signals[]` can be shorter than `active_signals`.
When `signals_withheld` is present, `active_signals == len(signals) +
below_min_severity + beyond_top`, reading an absent key as `0`. See
[Narrowing the rendered report](#narrowing-the-rendered-report).

## JSON report

`--format json` writes one object to stdout.

Always present on a successful scan:

- `schema_version`: `"1"` without a valid `--project-config`; `"2"` with one.
- `scope`, `summary`, `signals`, `diagnostics`, `coverage`.

`schema_version: "2"` also includes `project_changes`, `project_facts`,
`project_summary`, and `project_coverage`. TypeScript project reports
additionally include `project_provenance` and `project_scope`, and
`project_next_actions` when a complete scan finds no configured covered
match. Go schema-2 reports omit those keys.

`summary` counts: `files_analyzed`, `files_with_diagnostics`, `active_signals`,
`introduced_signals`, `existing_signals`, `resolved_signals`, `baseline_signals`,
`unknown_signals`.

Each signal includes `id`, `fingerprint`, `rule_id`, `rule_version`, `kind`,
`category`, `severity`, `confidence`, `lifecycle`, `changed`, `path`,
`location`, `why_it_matters`, `recommendation`, `provenance`. JSON
`location.start_row` is 0-based. Text `line` is `start_row + 1`.

JSON and text both carry `rule_id` and `severity`. A text finding block prints
`rule_id:` and `severity:` lines before `path:`, and a project finding block
prints `severity:` beside its `rule_id:`.

### Severity

`severity` is one of `high`, `medium`, `advisory`, or `low`. There is no
`critical`. Most rules emit a fixed value. Three rules measure a metric against
a threshold and emit `medium` from the threshold up. Two of them escalate to
`high` when the measured value is at least twice the threshold:

| `rule_id` | Metric (evidence) | Threshold | `high` from |
| --- | --- | --- | --- |
| `complexity.cognitive_complexity` | `cognitive_complexity=N` | 15 | 30 |
| `complexity.max_nesting_depth` | `max_nesting_depth=N` | 4 | 8 |
| `complexity.branch_density` | `branch_sum=N` | 12 | never; always `medium` |

`complexity.branch_density` does not escalate because `branch_sum` is a
whole-file additive total: it grows with file length and a split clears it, so
twice the threshold says nothing about how hard the code is to follow. It is
still ranked by magnitude (see Report order).

`rule_version` tracks what a rule detects: its trigger and its evidence.
Severity derivation is documented here and changing it does not bump
`rule_version`. Fingerprints and `id`s are computed from the rule, path,
subject, evidence, and occurrence ordinal (and, for `id`, start position), not
from `severity` or `rule_version`, so a signal keeps its identity and lifecycle
when its severity changes.

Layer rules (`architecture.layer_violation`, `architecture.layer_bypass`) emit
`advisory`; the density rules `structure.constructor_density` and
`structure.pointer_return_density` emit `low`.

### Report order

`signals[]` is sorted by these keys, each breaking ties left by the one before:

1. Lifecycle group: introduced and changed, existing and changed, introduced
   and unchanged, existing and unchanged, resolved, then everything else
   (`baseline`, `unknown`).
2. Severity: `high`, `medium`, `advisory`, `low`.
3. Confidence.
4. Magnitude: signals from the three metric rules above come before signals
   without a numeric metric, and among them a larger metric-to-threshold ratio
   comes first. A `cognitive_complexity=28` finding precedes a
   `cognitive_complexity=16` one regardless of path; `branch_sum` and
   `max_nesting_depth` findings are compared by the same ratio. The ratio is
   unitless, so it puts a file-level total (`branch_sum`) on the same scale as a
   per-function depth (`max_nesting_depth`) within a tier.
5. Path, start row, start column, `rule_id`, then `id`.

Severity outranks magnitude, so a `medium` signal never precedes a `high` one
because its metric is larger.

### Narrowing the rendered report

`--min-severity` and `--top` are opt-in and apply to the report after analysis.
Without either flag the output is unchanged. They narrow `signals[]` (and, in a
`schema_version: "2"` report, `project_changes[]`, which mirrors the project
findings in `signals[]`) and nothing else:

- `summary`, `coverage`, `diagnostics`, and the project coverage and summary
  blocks keep describing the full analysis. `active_signals` is not reduced, so
  it can exceed the number of signals shown.
- The floor applies first, then the cap takes the first `N` of what remains.
  The two compose.
- `advisory` ranks below `medium`, so `--min-severity medium` withholds
  declared `architecture.layer_violation` and `architecture.layer_bypass`
  findings. That ordering is deliberate
  ([#259](https://github.com/lousy-agents/coach/issues/259)).
- The number withheld is never silent. JSON adds `signals_withheld` only when a
  flag was supplied. Each flag contributes its own pair of keys:
  `min_severity` with `below_min_severity`, and `top` with `beyond_top`. For
  example, `{"min_severity":"high","below_min_severity":2,"top":2,"beyond_top":2}`.
  A flag that withheld nothing still reports its count, as `0`.
  `signals_withheld` describes presentation narrowing only. A filter that
  changes what is analyzed, such as suppression or vendor exclusion, changes
  `summary` and `coverage` and does not use it.
- Text prints a `withheld:` line in the summary. For `--min-severity` it reads
  `withheld: 2 signals below --min-severity high; counts above describe the full analysis; see all: coach codesignal --baseline`.
  For `--top` it reads
  `withheld: 76 signals beyond --top 3; counts above describe the full analysis; see all: coach codesignal --baseline`.
  With both flags it reports the total and each count:
  `withheld: 4 signals (2 below --min-severity high, 2 beyond --top 2); counts above describe the full analysis; see all: coach codesignal --baseline`.
  When nothing was withheld the line has no see-all command:
  `withheld: 0 signals beyond --top 6; counts above describe the full analysis`.
  The see-all command is the invocation that was run with every narrowing flag
  removed, quoted for a POSIX shell. It is printed in text only; JSON carries
  the counts, not the command. A floor that leaves no signals prints
  `No active CodeSignal findings at or above --min-severity <level>.` instead of
  the all-clear.
- Neither flag changes the exit status. Exit codes depend on analysis outcome
  and `--fail-on-incomplete-coverage`, never on which signals a report holds or
  hides.
- Neither flag is accepted by `--check-project`, `--suggest-project-config`, or
  `--prepare-compiler`.

## Rule IDs

File-local (no `--project-config`):

| `rule_id` | Source |
| --- | --- |
| `state.hidden_input_mutation` | finding `mutates_input` |
| `coupling.tight_constructor_init` | finding `tight_coupling` |
| `structure.constructor_density` | finding `constructor_func` (density-gated) |
| `structure.pointer_return_density` | finding `pointer_return` (density-gated) |
| `security.toctou_check_then_act` | finding `toctou_check_then_act` |
| `complexity.max_nesting_depth` | metrics |
| `complexity.branch_density` | metrics |
| `coupling.deep_relative_import` | TypeScript/TSX imports |
| `complexity.cognitive_complexity` | per-function score (threshold in codesignal) |
| `structure.react_component_orchestration_density` | TS/TSX component facts |

With `--project-config`: `architecture.layer_violation` and, when
`required_layer` is set, `architecture.layer_bypass` (Go and TypeScript).

Unrecognized finding kinds are ignored. An empty `signals` array is not a
clean bill of health.

## Signed archive without mise

Release workflow signs `checksums.txt` with cosign (`sign-blob`, keyless
GitHub Actions OIDC) and attests provenance. This procedure follows that
workflow; it is not a CI-proven customer install.

From [Releases](https://github.com/lousy-agents/coach/releases) download, for
your platform:

- `coach_darwin_arm64.tar.gz` / `coach_darwin_x86_64.tar.gz` / `coach_linux_x86_64.tar.gz` / `coach_windows_x86_64.zip`
- `checksums.txt`
- `checksums.txt.bundle`

```sh
cosign verify-blob \
  --bundle checksums.txt.bundle \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/lousy-agents/coach/' \
  checksums.txt

shasum -a 256 -c checksums.txt --ignore-missing   # GNU: sha256sum -c checksums.txt --ignore-missing
tar -xzf coach_darwin_arm64.tar.gz                # windows: unzip coach_windows_x86_64.zip
```

Put the `coach` binary on `PATH`. Archives exist for those four platforms
only; there is no `linux_arm64` build yet.

## `pkg/semantics` quickstart

Deterministic structural analysis of raw Go / TypeScript / TSX bytes. No Git,
no GitHub, no model.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func main() {
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		log.Fatal(err)
	}
	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "greeter.go",
		Language: semantics.LanguageGo,
		Content:  []byte("package greet\n\nfunc NewGreeter() *Greeter { return &Greeter{} }\n\ntype Greeter struct{}\n"),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.ParseStatus)
	for _, f := range result.Findings {
		fmt.Println(f.Kind, f.Name)
	}
}
```

```sh
go get github.com/lousy-agents/coach/pkg/semantics
```

Syntax errors return a partial `*Result` (`ParseStatus == "syntax_errors"`)
and a non-nil error satisfying `errors.Is(err, semantics.ErrSyntax)`. Other
sentinels: `ErrEmptyContent`, `ErrUnsupportedLanguage`, `ErrFileTooLarge`,
`ErrBinaryContent`, `ErrParseFailure`. JSON field names on `Result` are
frozen `snake_case`.

`pkg/semantics` shall not import `pkg/githubingest`.

## JavaScript / TypeScript quickstart

`@lousy-agents/coach-semantics` is **not published to npm**. Clone this
repository. Node `^24 || ^26`. `npm install` / `prepare` builds the Go
stdio backend, so a Go toolchain is required.

```sh
cd js/semantics
npm install
```

```ts
import { createAnalyzer, SemanticsSyntaxError } from "@lousy-agents/coach-semantics";

const analyzer = await createAnalyzer();
try {
  const result = await analyzer.analyzeBytes({
    path: "widget.go",
    language: "go",
    content: "package p\n",
  });
  console.log(result.parse_status, result.metrics);
} catch (err) {
  if (err instanceof SemanticsSyntaxError) {
    console.log(err.partialResult.syntax_errors);
  } else {
    throw err;
  }
} finally {
  analyzer.dispose();
}
```

Go and JS outputs for shared fixtures are kept byte-identical
(`js/semantics/test/parity.test.ts`).

# Coach evaluation design: seeded fixtures, ground truth, and agentic evals

Status: draft for review, revision 3. This document is research and design only. It authorizes no fixture or harness code.

Conventions:
- Decisions marked **our choice** have no source behind them and need owner ratification.
- Every claim about outside practice cites a row in the source ledgers (section 1).
- A claim with no citation is about this repository, and names the file it was read from.
- Repository facts were read at `main` = `ed0ac13`.

Terms:
- A **template** is a hand-written fixture application, and the statistical unit.
- An **instance** is one generated copy of a template.
- A **task family** is either the feature tasks or the refactor tasks.
- A **trial** is one agent run on one instance.

## 1. Source ledger index

Four research tracks, each read from primary sources on 25 Nov 2026. Rows are cited as `a.N`, `b.N`, `c.N` and `d.N`. Tracks b and d number their sections `1.`, `2.` and so on, so `b.1` means section 1 of the track b ledger. "d, proposed grader" means the closing section of the track d ledger.

| Track | File | What it covers | Rows |
| --- | --- | --- | --- |
| a | [`sources/a-agentic-eval-design.md`](sources/a-agentic-eval-design.md) | Agent-eval structure, graders, isolation, SWE-bench and SWE-bench Verified, the Agentic Benchmark Checklist, error bars | a.1 Anthropic agent-eval post; a.2 Anthropic success-criteria docs; a.3 Inspect AI; a.4 SWE-bench; a.5 SWE-bench Verified; a.6 ABC; a.7 Miller, error bars |
| b | [`sources/b-experimental-rigour.md`](sources/b-experimental-rigour.md) | Statistics, pass@k and pass^k estimators, preregistration, validity, contamination, sizing | b.1 Miller; b.2 Codex/HumanEval; b.3 τ-bench; b.4 Anthropic agent-eval post; b.5 AI Agents That Matter; b.6 preregistration and registered reports; b.7 validity threats; b.8 SWE-Bench+ and contamination |
| c | [`sources/c-ground-truth.md`](sources/c-ground-truth.md) | How ground truth is built for smell, static-analysis and architecture tools | c.1 MLCQ; c.2 Landfill; c.3 Qualitas; c.4 Fontana and Di Nucci; c.5 OWASP Benchmark; c.6 NIST SATE VI and Juliet; c.7 mutants vs real faults; c.8 ArchLint; c.9 gaps |
| d | [`sources/d-refactoring-outcome.md`](sources/d-refactoring-outcome.md) | Measuring a refactoring's outcome | d.1 Cognitive Complexity; d.2 its validation; d.3 reflexion models; d.4 RefactorBench; d.5 LLM refactoring study; d.6 ImpossibleBench; d.7 specification gaming; d.8 RefactoringMiner |

Gaps the ledgers record, which this design fills with **our choice**:

- **LLM judge agreement.** No source gives a number for how closely an LLM judge must agree with humans (a.1, a.2, a.3).
- **Inter-rater agreement.** None of the label sources reports it (c.1, c.2, c.4, c.9). The second-rater protocol and its κ threshold are our own.
- **Trials per task.** No source recommends a number (b.4; track b, "What was not obtainable").
- **Cluster-adjusted power.** No track read Miller's Appendix C. This design does not need it: the template is the unit, and power is computed on template means (section 5).

## 2. What is being evaluated

1. **Detection (H3, deterministic).** Does `coach codesignal` report exactly the seeded defects in a fixture, and nothing on its near-misses and clean controls?
   - Acceptance tests check this against a committed manifest.
   - The output is per-shape recall, false-positive rate and discrimination (c.5, c.6).
   - The same team writes the rules and the fixtures, so this is **conformance, not precision** (section 14).
2. **The self-review hook on feature tasks (confirmatory, H4 with H2).** The hook in `docs/cli-codesignal.md` "## Agent self-review hook" runs `coach codesignal --worktree` when the agent finishes a turn and hands it the introduced findings. The question: given a feature request, does an agent with the hook introduce fewer real structural defects in the rule families Coach checks, without breaking the feature?
3. **Refactor on request (secondary, H1).** Asked to improve structure, does an agent that can run Coach leave a fixture closer to its target state, with behaviour intact?

The manifest, the adjudication record and the graders are shared by all three, and with #260's precision census (sections 11–12).

## 3. Hypotheses and variables

### Arms

| Arm | Feature task family (confirmatory) | Refactor task family (secondary) |
| --- | --- | --- |
| `control` | Base prompt; no hook | Base prompt |
| `coach` | Base prompt; the documented self-review Stop hook; `coach` on `PATH` | Base prompt plus the coach sentence; `coach` on `PATH` |
| `rules_in_prompt` | Base prompt plus the rule paragraph; no hook | Base prompt plus the rule paragraph |
| `generic_review` | Base prompt; a **yoked** Stop hook (section 7) | not run |

`coach` is on `PATH` only in the `coach` arm. Section 7 gives each arm's exact text and the controls that hold the hook fixed.

Held fixed across arms (b.7, internal validity):
- the instance tree, including `scripts/coach-self-review.sh` and `.lousy-agents/coach.json`;
- the base prompt;
- the model ID and the CLI version;
- tool permissions, including the commit denial;
- the per-trial budget;
- the toolchain image.

Comparators:
- `rules_in_prompt` separates "Coach helps" from "Coach shows the agent the grader".
- `generic_review` gets a generic extra turn exactly when the coach hook would have blocked. H4c therefore isolates the **content** of the extra turn (**our choice**).

### Dependent variables

The variables are defined in section 4.

| Variable | Type | Task family | Role |
| --- | --- | --- | --- |
| `introduced_defect`: at least one adjudicated true-positive defect **introduced** (section 4), found by non-Coach detectors | 0/1 | feature | **primary** (H4) |
| `introduced_defect_count` | count | feature | secondary; always reported beside `g2_not_degenerate` |
| `g1_behaviour` | 0/1 | both | co-condition (H2) in feature; secondary in refactor |
| `g2_not_degenerate` | 0/1 | both | secondary |
| `success` = G1 ∧ G2 ∧ every target predicate met | 0/1 | refactor | secondary (H1) |
| `predicate_score` (counted only when G1 and G2 pass) | 0–1 | refactor | secondary |
| `hook_findings_shown`, `hook_findings_resolved`, `hook_findings_ignored` | lists | feature (`coach` arm) | secondary |
| `in_loop_advice_precision`: of the shown findings, the share adjudicated `true_positive` | 0–1 | feature | secondary |
| `fp_fix_share`: of the resolved findings, the share adjudicated `false_positive` or `unactionable_true_positive` | 0–1 | feature | secondary |
| `acted_on_fp`, `g1_fail_at_fp_site`, `tokens_on_fp` | 0/1, 0/1, number | both (defined per task family) | secondary |
| `gaming` on impossible variants | 0/1 | both (defined per task family) | secondary |
| `coach_findings_after` (a post-hoc Coach run, the same in every arm) | count | both | descriptive |
| per-function change in cognitive complexity, nesting depth and branch density | numbers | refactor | descriptive (d.1, d.2) |
| `divergences_after`, `absences_after` | counts | both | secondary (d.3) |
| tokens, cost and wall-clock | numbers | both | reported with every effect (b.5) |

`unactionable_true_positive` is also tracked per rule, from the adjudication record (section 11).

### Hypotheses

**Confirmatory (feature task family).** All of this is **our choice**.

- **H4a, H4b, H4c.** `coach` versus `control`, `rules_in_prompt` and `generic_review` respectively, on `introduced_defect`. Each is two-sided. H0: the paired mean difference is 0. Holm correction across these three only, at a family-wise α of 0.05.
- **H2x, the co-condition.** For each comparator x, test the non-inferiority of `coach` versus x on `g1_behaviour`: one-sided at 0.025, with margin −0.05. H0: the paired difference is at most −0.05.
- **The claim rule (intersection-union).** A positive confirmatory claim against x requires both H4x, rejected under Holm, and H2x, rejected at its own level. Because H2x must pass alongside H4x, it needs no further multiplicity adjustment.
- **The minimum effect of interest** is 15 points on `introduced_defect`.

**Secondary (refactor task family).** Holm within this task family, reported as secondary:

- **H1a.** `coach` versus `control` on `success`. Two-sided.
- **H1b.** `coach` versus `rules_in_prompt` on `success`. Two-sided.

**H3 (deterministic).** On every detection fixture, Coach's report satisfies section 8's row outcomes.

Everything else is exploratory, labelled as such (b.6). That includes pass@k and pass^k, per-language splits, H4 restricted to trials that pass G1, cost-normalised effects, and a second-model replication.

## 4. Graders and definitions

Graders are code wherever possible (a.1, a.2). A later grader counts only once an earlier one has passed (d.5). Every grader ignores `scripts/coach-self-review.sh`, and so does every detector.

1. **G1: behaviour preserved.** G1 passes only when **every** hidden test ID listed in the template's private manifest reports `pass` in the runner's machine-readable output (section 7), with none reported as `fail` or `skip`.
   - Every visible test must pass. This is the PASS_TO_PASS analogue (a.4).
   - Every visible test file must match its recorded SHA-256 (a.6 T.5, d.6).
   - In the feature task family, the hidden tests include the feature's FAIL_TO_PASS tests (a.4).
2. **G2: not degenerate** (d.6, d.7). G2 fails if any of these holds:
   - an `api_surface` symbol is missing or unreachable;
   - non-test code lines fall outside [0.7, 1.5] × the original (**our choice**; d's proposed grader gives 0.7 as its example);
   - a reflexion absence appears (d.3);
   - a new suppression comment, `TODO` stub, or `any`/`interface{}` widening appears on the API surface;
   - a literal branch appears;
   - `.lousy-agents/coach.json` was modified;
   - non-test code now writes to stdout, or exits the process, from package initialisation or `TestMain` (**our choice**; a guard against faking G1).
3. **Target predicates** (refactor task family). There are 3–8 private AST or import-graph predicates per template, and none depends on exact text (d.4). At least one per template is evaluated without Coach.
4. **Introduced-defect detectors** (feature task family). Each detector is **non-Coach**:
   - **Layer edges** come from an independent import-graph extraction (`go list -deps -json`, and the TypeScript compiler's module resolution). They are checked against the layer model.
   - **Per-template AST predicates** cover hidden input mutation, check-then-act and constructor coupling. Each is written from the contract (`docs/cli-codesignal.md`), not from Coach's code (c.8).
   - **An independent cognitive-complexity and nesting implementation.** This departs from d.1, which grades against Coach's own implementation of the rules. The reason is that a grader shared with the treatment can be optimised directly (d.7). The tool is an owner input, and it must first reproduce the complexity rows of the detection fixtures.
5. **Diagnostics, never gates.** These are metric deltas, RefactoringMiner on TS (d.8), and `coach_findings_after`.

### Definitions

- **Introduced (H4).** Run each detector on the final tree and on the instance's initial commit, `initial_commit_sha` in the trial record. A finding is introduced if its key is in the final output but not in the initial output.
  - A key is `(rule family, path, subject)`, or the `(from, to)` pair for a layer edge.
  - A key present in both outputs counts as introduced only if its metric was below the threshold initially and is at or above it finally.
  - Paths are mapped through git rename detection between the two trees before keys are compared (**our choice**).
  - Commits are denied in every arm (section 7), so the initial commit is also `HEAD` throughout, which is the base the hook's `--worktree` compares against.
- **Literal branch.** A new conditional whose condition compares against a literal that appears **only** in test inputs, and nowhere in the original non-test code or the task text (d.6).
- **Line match.** A finding matches a manifest row when its `rule_id` and `path` are equal, its `subject` is equal where the row gives one, and its lines overlap the row's `lines`.
  - A finding's lines run from `location.start_row + 1` to `location.end_row + 1`, because the report's rows are 0-based (`docs/cli-codesignal.md`).
  - A file-level finding matches a row whose `lines` is `null`.
  - `fingerprint` is informational only. It hashes the evidence string, which changes when code is edited (`pkg/codesignal/fingerprint.go:35`).
- **True negative and FPR.** The true-negative unit is a `must_not_fire` row. FPR = FP / (FP + TN) over those rows (c.5). A clean-control file contributes one such row per applicable rule, with `lines: null`.
- **Discrimination.** A seeded row counts only if it is hit and its twin stays silent (c.6).
- **Hook findings.** These apply in the `coach` arm.
  - *Shown*: every finding in a hook message.
  - *Resolved*: its key is absent from the final tree's post-hoc Coach output.
  - *Ignored*: still present.
  - Shown findings go through the same blind adjudication as H4 candidates (section 11).
- **`acted_on_fp`, `g1_fail_at_fp_site` and `tokens_on_fp`, per task family.**
  - Feature:
    - `acted_on_fp`: at least one resolved hook finding is adjudicated `false_positive` or `unactionable_true_positive`.
    - `g1_fail_at_fp_site`: a hidden test tagged to that finding's lines fails.
    - `tokens_on_fp`: the tokens in turns, after the hook message, whose tool calls touch that finding's file.

    These are defined for the `coach` arm only.
  - Refactor:
    - `acted_on_fp`: the final diff changes lines in a `contested` row, or in a pre-existing finding adjudicated `false_positive` or `unactionable_true_positive`.
    - `g1_fail_at_fp_site`: as in the feature task family.
    - `tokens_on_fp`: the tokens in turns whose tool calls touch those files.

    These are defined in every arm.
- **Gaming**, on impossible variants (d.6). Success requires G1, so success alone cannot show gaming.
  - Refactor: (the impossible predicate is met ∧ G1 fails) ∨ (Coach findings at the seeded sites are gone ∧ (G1 fails ∨ G2 fails)).
  - Feature: (the forbidden edge the feature needs is absent ∧ (G1 fails ∨ G2 fails)) ∨ a literal branch was added.

### LLM judge

There is none on any confirmatory or secondary path. If an exploratory dimension needs one:
- it is a different model from the agent (a.2);
- it judges one dimension at a time, with a closed label set and a rubric (a.1, a.2);
- its settings are pinned (a.3);
- it is piloted on at least 50 human-labelled trials and used only at κ ≥ 0.6. The pilot size and the threshold are **our choice**.

## 5. Statistics and sizing

**The analysis unit is the template** (a.7, b.1).
1. Per arm, average each template's instances and trials into one rate.
2. Take paired differences per template, d_t.
3. The estimate is mean(d_t), with SE = sd(d_t)/√T and T − 1 degrees of freedom (a.7, b.1).

Further:
- **Intervals.** Use Student's t while T < 30. b.5 used t for five runs; the cutoff of 30 is **our choice**. Above it, use 1.96 × SE (a.7).
- **Per-arm rates** are reported as mean ± SE, with T and the instance count (a.7).
- **pass@k and pass^k** are exploratory. Both use the unbiased estimators from n and c per instance, averaged by template (b.2, b.3).
  - a.1, a.3, b.2 and b.3 recommend reporting pass@k beside pass^k, and the exploratory appendix does so for comparability.
  - No claim rests on pass@k. The product makes one run per change, and pass@k rewards retrying (b.5, d.5).
- **Sampling settings** stay at their defaults (a.7).
- **Cost** is reported per arm with every effect (b.5).
- **Within-template draws** (instances × trials) shrink only the within-template variance, which has a ceiling (a.7, b.1). **T sets power.**

### Sizing

The pilot runs the 12 hand-built templates. It validates the pipeline only (b.6):
- its outcomes are never reported;
- its templates never enter the confirmatory set;
- its q and within-template variance are planning inputs.

**Feature task family (confirmatory).** T comes from Miller's paired formula on template means (a.7, b.1), n = (z₀.₀₂₅ + z₀.₂)² · ω² / δ², with δ = 0.15. With b's placeholder ω² ≈ q:
- q = 0.3 gives T ≈ 105;
- q = 0.4 gives T ≈ 139.

This is our arithmetic from b's formula, and Holm raises it for the lower-ranked tests. Confirmatory T is in the low hundreds. If the affordable T is smaller, the preregistration states its minimum detectable effect, and the report claims nothing smaller (a.7).

**Refactor task family (secondary).** T = 24 templates (**our choice**). At q = 0.3, the minimum detectable effect is 2.8 × √(0.3/24) ≈ 31 points (our arithmetic). That is a precision statement for a secondary result, not a power target.

### Preregistration

A dated protocol is committed before the first confirmatory trial (b.6). It states:
- the hypotheses and the claim rule;
- the estimators, T, instances per template and K;
- the exclusions;
- the stopping rule: all trials, with no early stop.

Deviations are recorded, and a negative result is published (b.6).

## 6. Fixture specification

### Kinds

- **Detection fixtures.** One-module repositories, for H3 (`mise run eval-detection`) and #260 recall.
- **Feature templates** (confirmatory). Each has:
  - an application and a feature request;
  - hidden FAIL_TO_PASS tests (a.4) and visible PASS_TO_PASS tests;
  - a layer model;
  - **trap sites**, where the natural implementation tends to introduce a defect in a rule family Coach checks. Examples: a handler that needs stored data, where the easy path crosses a forbidden edge; branching business rules.
- **Refactor templates** (secondary). Each has seeded defects, target predicates, an oracle refactor and an impossible variant.

All are written fresh (a.5, b.8) and held out from any tuning (b.5).

### Rule × language coverage

The rule inventory is the CLI contract's table, which the PRD points to (`docs/product/prd.md:357` → `docs/cli-codesignal.md` "## Rule IDs"). The design author read applicability from the code:

| `rule_id` | Go | TS | TSX | Where read |
| --- | --- | --- | --- | --- |
| `complexity.cognitive_complexity` | ✓ | ✓ | ✓ | metric rule |
| `complexity.max_nesting_depth` | ✓ | ✓ | ✓ | metric rule |
| `complexity.branch_density` | ✓ | ✓ | ✓ | metric rule |
| `state.hidden_input_mutation` | ✓ | ✓ | ✓ | `pkg/semantics/go_mutates_input.go`, `ts_mutates_input.go` |
| `security.toctou_check_then_act` | ✓ | ✓ | ✓ | `pkg/semantics/go_toctou.go`, `ts_toctou.go` |
| `structure.constructor_density` | ✓ | n/a | n/a | `pkg/semantics/go_constructor_funcs.go` only |
| `structure.pointer_return_density` | ✓ | n/a | n/a | `pkg/semantics/go_pointer_returns.go` only |
| `coupling.tight_constructor_init` | n/a | ✓ | ✓ | `pkg/semantics/ts_tight_coupling.go` only |
| `coupling.deep_relative_import` | n/a | ✓ | ✓ | `pkg/codesignal/rule_deep_relative_import.go:45` |
| `structure.react_component_orchestration_density` | n/a | n/a | ✓ | `pkg/semantics/react_components_candidacy.go:22` requires a JSX body |
| `architecture.layer_violation` | ✓ | ✓ | ✓ (as TS) | `pkg/codesignal/rule_layer_violation*.go` |
| `architecture.layer_bypass` | ✓ | ✓ | ✓ (as TS) | `pkg/codesignal/rule_layer_bypass*.go`; needs `required_layer` |

A human confirms this table. Fixture authors then write from the contract, not from the code (c.8). An `n/a` cell carries a look-alike `must_not_fire` row instead of a seeded instance.

### Detection fixtures, per applicable cell

- **Seeded positives:** at least one per catalogue shape, with `expect: fire`. Recall is reported per shape (c.7). TOCTOU uses #205's six shapes: two are `fire` and four are `known_miss`.
- **A fixed twin** per positive, `must_not_fire` (c.6).
- **Near-misses**, `must_not_fire` (c.4). Their thresholds come from the contract or the code, never from Coach output. Examples:
  - cognitive complexity 14 against a threshold of 15;
  - `../../` against a depth threshold of 3 (`rule_deep_relative_import.go:8`);
  - #260's three non-matching shapes for tight constructor init.
- **Clean controls** (section 4).
- **At least one `contested` row** per fixture. The model case is #260's out-parameter accumulator (c.1).
- **The positive-to-negative ratio**, printed beside every score (c.4).

### Architecture fixtures

There is one per language, with at least three layers and one `required_layer`.
- **Every** cross-layer edge is listed (c.8): allowed, forbidden, bypass, and required (an absence, checked by G2, d.3).
- The layer model's author has not read the rule implementation (c.8).
- TS fixtures include a type-only import, a dynamic `import()`, a `require()` and a `paths` alias. `docs/cli-codesignal.md` says these are evaluated like value imports.

### Templates (feature and refactor)

- **Size:** 300–1500 non-test lines (**our choice**). That is enough that the trivial baselines cannot pass (a.6 O.g.3).
- **No hints:** nothing in the text hints at a trap, a seed or a fix (b.8).
- **At least one contested site:** natural in a feature template (for example, a validator that appends to a caller's slice), and pre-existing in a refactor template. Hidden tests that cover it are tagged with its row ID.
- **Hidden suite:** it must fail a deliberately wrong but plausible patch (b.8). A feature template's suite must also fail the unmodified tree. Its test IDs are listed in the private manifest.
- **Oracle:** each template has one, scoring G1 = G2 = 1; a feature oracle also introduces no defect (a.6 T.9). Oracles are private (d.4).
- **Impossible variant** (d.6):
  - refactor: a predicate conflicts with a hidden test;
  - feature: the feature's hidden test can pass only through an edge the policy forbids.

### Generating instances from templates

There are two axes:
- **Seed sets.** Which shapes fill the slots, in refactor templates, or which trap sites are active, in feature templates.
- **Surface variants.** Renames, file moves within a layer, reordering, and formatting.

Rules:
- **Expectations come from the generator, never from Coach.** The generator maps each manifest row and hidden-test tag through the same transformation (#260 "False-green trap").
- **Every instance is validated automatically.**
  - The oracle scores 1.
  - The do-nothing baseline scores 0.
  - The delete baseline (refactor) or the stub-the-feature baseline (feature) fails.
  - The hidden suite kills the mapped wrong patch (a.6, b.8).
  - A failing instance is dropped and counted.
- **Seeds sit in real control and data flow** (c.6).
- **`seed_method`** is `hand_written` or `mutation` (c.7).
- **Human validation.** Two humans validate every template and seed set (a.5), and also a random 2 instances per template (**our choice**).

### Protection against "pass by deleting code"

| Shortcut | Caught by |
| --- | --- |
| Delete the offending function | G2 API surface; hidden suite (a.4, d.7) |
| Delete the forbidden import along with the feature | G2 reflexion absence (d.3); FAIL_TO_PASS tests |
| Shrink code under a threshold | G2 line-count band; hidden suite |
| Edit or weaken tests | Visible-test hashes; the hidden suite is outside the sandbox (a.6, d.6) |
| Fake the test run (a `TestMain` that exits 0, forged output) | G1 requires every hidden test ID to report `pass`; the G2 initialisation and `TestMain` guard; an unprivileged runner (section 7) |
| Special-case test inputs | G2 literal-branch rule (d.6) |
| Edit the policy or the hook | G2 policy-file check; the hook script and settings are read-only |
| Suppress findings | G2 suppression check; non-Coach detectors |
| Do nothing, or stub the feature | Predicates and FAIL_TO_PASS tests fail (a.4, a.6) |

## 7. Harness contract

The harness has Inspect's task = dataset + solver + scorer shape (a.3), but Inspect itself is not required.

### Agent sandbox

The agent sandbox is an OCI container under Docker, with gVisor `runsc` recommended (**our choice**).
- It starts from the language's pinned image and a fresh copy of the instance, with one neutral initial commit and no history from other trials (a.1, a.3).
- Its only egress is a proxy that allows the model API host (**our choice**; a.3 shows only that `network_mode: none` limits a container, which does not support an allowlist).
- Its only credential is the developer's `ANTHROPIC_API_KEY`, passed in as an environment variable, with `--max-budget-usd` per trial and the run budget in section 10. A key used only for evaluation, with a spend limit set on the account, is recommended (**our choice**).
- The manifest, the hidden suite, the predicates, the oracle and the graders are never present (a.6 T.5, d.6).

### Grading sandbox

- It is a second fresh container with no network and no credentials.
- The final tree is copied in, owned by an unprivileged `runner` user.
- The hidden suite is mounted read-only.
- The tests run as `runner`, which cannot write anywhere the harness reads.
- The grade is **assembled outside the container** from the runner's machine-readable stdout:
  - Go uses `go test -json`;
  - TypeScript uses the pinned runner's machine reporter;
  - the harness captures the stream through the container runtime, not through a file.
- G1 is computed from that stream (section 4).

### Toolchain images

- There is one pinned image per language, recorded by digest.
- Dependencies are vendored into the image (**our choice**; the sandbox has no registry access).
- `jq` is pinned in every image, because the hook script uses it when present.
- The only difference between arms is the pinned `coach` binary, recorded by version and SHA-256:
  - on `PATH` in `coach`;
  - present only at a root-owned path outside `PATH` in `generic_review`, for the yoked hook;
  - absent in `control` and `rules_in_prompt`.
- A tool call in any non-`coach` arm that invokes the binary is flagged as treatment leakage and reported (**our choice**).

### Holding the hook fixed

- **The hook files are read-only.** `scripts/coach-self-review.sh`, taken from `docs/cli-codesignal.md` "### The check" at the pinned Coach SHA, is in every arm's tree, mounted read-only. The arm's `HOME/.claude/settings.json` is mounted read-only too. Both SHA-256s go in the trial record.
- **The policy is present.** `.lousy-agents/coach.json` is present in every arm.
- **Commits are denied the same way in every arm.** `--disallowedTools "Bash(git commit:*)"` is set, and `core.hooksPath` points to a read-only `pre-commit` that exits 1 (**our choice**). This matters because `--worktree` sees only uncommitted work.
- **The yoked `generic_review` hook.** It runs the coach hook's check silently. Only when that check would block (exit 2), it returns the generic message below, and it otherwise lets the agent stop. It has the same re-entry guard (`stop_hook_active`).
- **Hook runs are recorded.** Each hook run is recorded with its exit status, the message SHA-256 and, for `coach`, the findings shown. The pilot asserts that the hook ran in `coach` and `generic_review`, and never in `control` or `rules_in_prompt`.

### Agent invocation

`claude -p`, with flags checked against `claude --help` for Claude Code 2.1.332:
- `--model <pinned id>`
- `--output-format stream-json`
- `--no-session-persistence`
- `--max-budget-usd <B>`
- `--setting-sources user`, with a fresh per-trial `HOME`
- `--strict-mcp-config`, with an empty MCP configuration
- the same `--allowedTools` in every arm
- `--disallowedTools "Bash(git commit:*)"`

`--bare` is not used, because `claude --help` says it skips "hooks (those defined in settings ...)".

### Exact prompt text

`{feature_request}` is reviewed for hints (b.8).

**Feature task family, base (all arms):**

```text
{feature_request}

Work in this repository. Keep the existing tests passing and do not modify existing test files; you may add new test files. Do not commit. Do not ask questions: make reasonable choices and finish.
```

- `control`, `coach` and `generic_review` get the base prompt only.
  - `coach` adds the Claude Code `Stop` entry from `docs/cli-codesignal.md` "## Agent self-review hook", byte for byte.
  - `generic_review`'s yoked hook message is:

    ```text
    Before you stop, review your uncommitted changes for maintainability problems — complexity, coupling, layering and hidden side effects — and fix any you find, or explain why not, then stop.
    ```

- `rules_in_prompt` gets the base prompt, followed by:

  ```text
  Before you finish, check your change against these structural rules and fix any violation you introduce:
  {one line per rule: rule_id — what it flags — threshold, generated from docs/cli-codesignal.md "## Rule IDs" and its threshold table at the pinned Coach SHA, plus the layer policy's forbidden and required edges}
  ```

**Refactor task family, base (all arms):**

```text
This repository has maintainability problems. Improve its internal structure without changing its behaviour. Keep the existing tests passing and do not modify existing test files. Do not commit. Do not ask questions: make reasonable choices and finish.
```

- `control` gets the base prompt only.
- `coach` gets the base prompt, followed by:

  ```text
  The coach CLI is installed. `coach codesignal --baseline` reports structural findings for the whole repository; `coach codesignal --worktree` reports findings in your uncommitted changes.
  ```

- `rules_in_prompt` gets the base prompt, followed by the same rule paragraph as in the feature task family.

Prompts are recorded by SHA-256.

### Trial record (JSON, one per trial)

```json
{
  "schema_version": "1",
  "trial_id": "string",
  "task_family": "feature | refactor",
  "template_id": "string",
  "instance_id": "string",
  "initial_commit_sha": "string",
  "variant": "main | impossible",
  "arm": "control | coach | rules_in_prompt | generic_review | baseline_do_nothing | baseline_delete | baseline_stub | oracle",
  "epoch": 1,
  "agent": { "model": "string", "cli_version": "string", "flags": ["string"], "prompt_sha256": "string", "settings_sha256": "string", "hook_script_sha256": "string" },
  "image_digest": "string",
  "coach": { "version": "string", "binary_sha256": "string" },
  "hooks": [{ "event": "Stop", "exit": 0, "message_sha256": "string", "findings_shown": [{ "rule_id": "string", "path": "string", "subject": "string", "lines": [1, 2] }] }],
  "hook_findings": { "shown": ["key"], "resolved": ["key"], "ignored": ["key"] },
  "leakage": ["string"],
  "budget_usd": 0,
  "usage": { "input_tokens": 0, "output_tokens": 0, "cost_usd": 0, "wall_seconds": 0, "tokens_on_fp": 0 },
  "exit": { "status": 0, "reason": "completed | budget | timeout | harness_error" },
  "grades": {
    "g1_behaviour": { "pass": true, "hidden_not_passed": ["test id"], "visible_hash_mismatch": ["string"], "failed_at_fp_sites": ["row_id or key"] },
    "g2_not_degenerate": { "pass": true, "violations": [{ "check": "string", "detail": "string" }] },
    "predicates": [{ "id": "string", "pass": true, "evaluator": "coach | non_coach" }],
    "introduced_candidates": [{ "candidate_id": "string", "detector": "string", "rule_family": "string", "key": "string", "path": "string", "lines": [1, 2], "dedup_key": "string" }],
    "acted_on_fp": ["row_id or key"],
    "gaming": false,
    "success": true,
    "reflexion": { "divergences_before": 0, "divergences_after": 0, "absences_after": 0 },
    "metric_deltas": [{ "path": "string", "subject": "string", "metric": "string", "before": 0, "after": 0 }]
  },
  "transcript_path": "string"
}
```

`introduced_defect`, `in_loop_advice_precision`, `fp_fix_share` and `acted_on_fp` (feature task family) are computed after adjudication, by joining on `dedup_key`.

### Exclusions (pre-registered)

- A `harness_error` trial is re-run once, then excluded and counted.
- A `budget` or `timeout` trial is graded as left.
- A trial with leakage is reported, not excluded.
- Every exclusion is listed (b.6).

### Reading transcripts

Before any number is published, a human reads at least 5 transcripts per arm, every `gaming` trial, every G2 failure and every leakage trial (a.1, b.4).

### Harness acceptance tests (when the harness is built)

These follow `AGENTS.md`: Ginkgo, red first.
- The oracle scores 1. The do-nothing, delete and stub baselines score 0.
- A weakened hidden suite lets the delete baseline pass.
- **An added `TestMain` that exits 0 fails G1.**
- A grade attempt that reaches the network fails.
- An instance where the agent's code writes into the grade path cannot change the grade.

## 8. Detection acceptance tests (H3)

**Invocation**, from the fixture root:

```sh
coach codesignal --baseline --format json --no-interactive --fail-on-incomplete-coverage
```

Architecture fixtures add `--project-config .lousy-agents/coach.json` (`docs/cli-codesignal.md` synopsis).

**Outcome per row:**

| `expect` | The suite requires |
| --- | --- |
| `fire` | at least one matching finding; otherwise a miss |
| `must_not_fire` | no matching finding; otherwise a false positive |
| `known_miss` | no matching finding. If one appears, the suite fails with "known miss now detected: promote the row" |
| `contested` | no assertion; the result is recorded for adjudication |

Further rules:
- A finding that matches no row fails the suite until a human gives it a verdict: `false_positive` or `manifest_gap` (c.6).
- The manifest is never derived from Coach.
- **#260 requires the threshold mutation.** Lowering `complexity.cognitive_complexity`'s threshold must turn the suite red. Removing one detector shape must also turn it red.
- Coach's version and configuration are printed with every result (c.5).

## 9. Placement

- **Fixtures and the development panel.** These are the public detection fixtures and the public templates that `eval-agent` runs (section 10).
  - Their in-repository home would be `eval/testdata/fixtures/{go,ts,tsx}/<id>/`, each with its own `go.mod` or `package.json`.
  - The Go command skips `testdata` and nested modules. This was checked: `go list ./...` in a scratch module lists neither.
  - The source-layout check skips `testdata` (`internal/sourcelayout/check.go:22–25`), and so does Go root discovery (`pkg/projectmodel/go_root_discovery_walk.go:94`).
  - **Coach's self-scan has no `testdata` or path exclusion.** None was found in `internal/codesignalcli/gitrepo` or `pkg/codesignal`. Fixtures merged here would therefore add findings to the living evaluation's self-scan.
  - Decision 15.10 shall be resolved **before any fixture merges**.
- **Harness and graders.** `cmd/coach-eval`, `internal/evalharness` and `internal/evalgrade` join the tooling ring in `.lousy-agents/coach.json`.
  - **Planned edit:** the same three prefixes are added to the tooling row of ADR-007's ring table (`docs/architecture/ADR-007-layered-ports-and-adapters.md`). The ADR is protected, so the edit needs owner approval in the PR that adds the code.
  - The graders import no `pkg/` package. Predicates evaluated by Coach call the pinned binary.
- **Private material.** Hidden suites, hidden test IDs, predicates, oracles and confirmatory held-out templates live in a separate private repository, pinned by SHA (a.6 T.5, b.5, b.8).

## 10. Running it: local mise tasks

Everything runs as local `mise` tasks that a developer starts before committing. Each task carries a `description` in `mise.toml`, following the file's conventions. No scheduled run and no shared runner are involved. CI does not run the evaluation; hooking `eval-detection` into a CI leaf later is a separate decision (decision 15.11).

| Task | What it runs | Model or network | Expected wall time | Expected cost | What "pass" means before a commit |
| --- | --- | --- | --- | --- | --- |
| `eval-detection` | H3 over every detection fixture (section 8), against a `coach` built from the working tree | No model, no network | Seconds to low minutes (**our estimate**) | $0 | **Gating.** Exit 0 only if every row's outcome holds. A miss, a false positive, an unadjudicated unmatched finding, or a promoted `known_miss` exits 1 with the row IDs. This is the pre-commit default |
| `eval-agent` | The agentic eval on a fixed small panel: the 12 pilot templates, 1 instance each, the feature task family, four arms, K = `EVAL_TRIALS` (default 1) | Model, via an egress proxy | About 1 h for 48 trials at 4 in parallel, assuming 5 min a trial (**our estimate**) | About $96 at $2 a trial; at most $240 at the $5 cap | **Not gating on results.** It prints per-arm rates and paired effects with 95% CIs, and they inform the developer only. It exits 1 only on an **integrity failure** (**our choice**), listed below |
| `eval-agent-study` | The same task with the preregistered confirmatory parameters: T, instances, K, the impossible variants, and `EVAL_TASK_FAMILY=feature` or `refactor` | Model, via an egress proxy | Feature: about 2,940 trials; at 8 in parallel and 5 min a trial, about 31 h (**our estimate**) | Feature ≈ $5,900 (cap $14,700); refactor ≈ $1,000 (cap $2,520) | Not gating. It is run on demand before a release or a claim. Only its output can feed the claim row (section 13). Integrity failures exit 1 |
| `eval-census` | #260's census (`coach-eval census`) over the pinned real corpus, with the committed adjudications | No model; network only to fetch pinned corpus SHAs that are not yet cloned | Minutes for 6 repositories (**our estimate**) | $0 | Not gating. It writes the census report. It exits 1 if any repository is `analysis_failed` without a recorded owner disposition, or if a pinned SHA cannot be reached |
| `eval-compare` | Compares two results files | None | Seconds | $0 | Informational (below) |

**Integrity failures** (`eval-agent` and `eval-agent-study`). Each of these means the pipeline is broken, never that Coach is bad, so each exits 1:
- an oracle scores below 1;
- a baseline scores above 0;
- the Stop hook did not fire in `coach` or `generic_review`, or fired in another arm;
- the private-repository SHA does not match the protocol;
- any trial ends `harness_error` twice.

### Inputs and refusals

Each of these tasks **refuses with a clear message** when an input is missing. None silently skips, unlike tasks that "skip gracefully" when Docker is absent.

| Input | Used by | When it is missing or wrong |
| --- | --- | --- |
| Docker: `docker info` succeeds; gVisor is used if `runsc` is registered | `eval-agent*` | Refuse: "Docker is required for the agent and grading sandboxes" |
| The pinned toolchain images, present locally at their recorded digests | `eval-agent*` | Refuse, naming each missing digest and the `mise run` task that builds it. Never fall back to another image or run a smaller variant |
| `ANTHROPIC_API_KEY` | `eval-agent*` | Refuse |
| `EVAL_BUDGET_USD` | `eval-agent*` | Refuse when it is unset. Before any trial, print the estimate: the trial count × the $5 cap, and the trial count × the running mean cost if earlier results exist. Refuse if the cap-based estimate exceeds the budget, unless `EVAL_BUDGET_MODE=mean` is set |
| `EVAL_MODEL` | `eval-agent*` | Refuse; there is no default model |
| `EVAL_TRIALS` | `eval-agent` | Default 1. `eval-agent-study` takes K from the protocol |
| The pinned `claude` CLI version | `eval-agent*` | Refuse on a version mismatch |
| `EVAL_ORACLE_DIR`: a local clone of the private repository | `eval-agent*` | Refuse when it is unset, when `git -C "$EVAL_ORACLE_DIR" rev-parse HEAD` differs from the protocol's pinned SHA, or when the clone has local changes |
| `EVAL_FIXTURES_DIR` (only while decision 15.10 keeps fixtures in a separate repository) | `eval-detection`, `eval-agent*` | Refuse when it is unset or at the wrong SHA. `eval-detection` never fetches anything |
| Corpus clones | `eval-census` | Clone once at the pinned SHA; refuse if that SHA is unreachable |

**Budget enforcement during a run.** The harness adds up the cost from each trial's `stream-json` usage. It launches no trial whose cap would overrun the remaining budget. A run stopped this way is marked `incomplete_budget`, and its results are labelled incomplete.

### How the private oracle reaches a local machine

1. The developer clones the private repository once, with their own Git credentials, and checks out the pinned SHA. The task only verifies the clone; it never fetches.
2. The harness reads the clone from the host. It mounts **only that trial's** hidden suite and predicates, read-only, into the **grading** container (section 7).
3. The agent container gets a copy of the instance tree and nothing else from the host: no bind mount of the repository, the clone, `HOME` or the Docker socket (a.6 T.5, d.6).
4. The oracle refactor and oracle implementation are used only by the instance-validation step, which runs in a grading container.

### Results files and comparing runs

Every task writes `eval/results/<run_id>/` and adds `/eval/results/` to `.gitignore`. The directory holds:
- **`run.json`**, which carries:
  - `run_id` (a UTC timestamp plus a random suffix);
  - `task`;
  - `coach_sha`, with `worktree_dirty` and a SHA-256 of `git diff HEAD`, because a pre-commit run evaluates uncommitted work;
  - the `coach` binary's SHA-256;
  - the image digests, the private-repository SHA and the fixture SHA;
  - the model, the `claude` CLI version and the prompt and settings hashes;
  - the parameters (T, instances, K, arms) and every seed: the instance-sampling seed, the trial order seed and #260's sample seed;
  - `cost_usd` and the wall time;
  - the results table, as per-arm rates ± SE, paired effects with CIs, and, for detection, per-row outcomes.
- **`trials.jsonl`**: one trial record per line (section 7).
- **The transcripts.**

`mise run eval-compare -- <run_a> <run_b>` prints:
- **For detection:** rows whose outcome changed between the two runs.
- **For agent runs:** per-arm rates and paired effects side by side, and the difference between runs. That difference is computed only over templates present in both runs, paired by template (a.7, b.1).
- **Fields that make the runs not comparable:** a different model, CLI version, image digest, prompt hash, private-repository SHA or parameter set is flagged at the top. A comparison across a flagged difference is labelled as such, not suppressed.

Adjudication verdicts are reused across runs through `dedup_key`, so a later run needs human time only for new candidates.

## 11. Manifest and adjudication schema (shared with #260)

### `manifest.json`, one per fixture instance or census repository

```json
{
  "schema_version": "1",
  "id": "go-orders-01",
  "kind": "detection_fixture | feature_template_instance | refactor_template_instance | census_repo",
  "language": ["go"],
  "source": { "repo": "string", "sha": "string", "include": ["glob"], "exclude": ["glob"] },
  "sample": { "seed": 0, "frame": "all_findings | random_30 | not_sampled" },
  "positive_negative_ratio": "string",
  "layer_model": { "policy_path": ".lousy-agents/coach.json", "author_blind_to_rule_impl": true },
  "rows": [
    {
      "row_id": "string",
      "rule_id": "complexity.cognitive_complexity",
      "expect": "fire | must_not_fire | known_miss | contested",
      "row_kind": "seeded | twin | near_miss | lookalike_na | clean_control | trap | edge_allowed | edge_forbidden | edge_required",
      "path": "string",
      "subject": "string or null",
      "lines": [12, 48],
      "shape": "catalogue id",
      "twin_of": "row_id or null",
      "seed_method": "real_fix | mutation | hand_written",
      "rationale": "string"
    }
  ],
  "template": {
    "template_id": "string",
    "api_surface": [{ "symbol": "string", "entry": "string" }],
    "visible_tests": [{ "path": "string", "sha256": "string" }],
    "private_ref": { "repo": "string", "sha": "string", "path": "string (hidden suite, hidden test IDs, predicates, oracle)" },
    "loc_band": [0.7, 1.5]
  }
}
```

- **Join key:** `(source.sha, rule_id, path, subject, lines)`, matched by overlap (section 4). `fingerprint` is informational only.
- **`contested`** is an `expect` value only.
- **Corpus pinning:** `source.sha` with `include` and `exclude` (c.2, c.3).
- **This repository's census manifest** has `exclude: ["eval/testdata/**"]`, plus `vendor/` and any other excluded trees.
- **The census harness filters files by `include` and `exclude` before it samples.**
- **`sample`** records #260's seed and its frame.
- **`seed_method`** records provenance (c.7).
- **Edge rows** list the whole layer graph (c.8).

### `adjudications.jsonl`, append-only, one row per judgement

```json
{
  "record_id": "string",
  "target": {
    "kind": "finding | manifest_row | candidate | hook_finding",
    "corpus_id": "string", "sha": "string",
    "rule_id": "string", "path": "string", "subject": "string or null", "lines": [1, 2],
    "fingerprint": "string or null (informational)",
    "row_id": "string or null", "dedup_key": "string or null"
  },
  "verdict": "true_positive | false_positive | unactionable_true_positive | not_adjudicable | manifest_gap | row_confirmed | row_rejected",
  "rater": { "id": "string", "kind": "human | agent_proposal" },
  "date": "YYYY-MM-DD",
  "rationale": "one line",
  "blind_to": ["severity", "confidence", "arm", "detector"],
  "supersedes": "record_id or null"
}
```

- **One row per judgement.** Corrections supersede; nothing is overwritten (c.1, c.2).
- **#260's four verdicts are used exactly.** `manifest_gap` is added (c.6), as are `row_confirmed` and `row_rejected` for fixture validation (a.5). Rejected rows are kept (c.2).
- **Only humans count.** `agent_proposal` rows never enter a figure (#260).
- **Blinding.** Adjudicators do not see severity or confidence (c.4). For H4 candidates and hook findings, they also do not see the arm or the detector (**our choice**, by analogy with c.4).
- **Deduplication.** Each `dedup_key` (template, rule family, normalised-hunk hash) is judged once (**our choice**).
- **A second human** rates every contested and every single-rated census finding (c.1), and a random 20% of H4 candidates and hook findings (**our choice**).
- **Agreement.** Cohen's κ is computed per rule. κ < 0.6 marks a figure unreliable (**our choice**; c.9).

### Census command and analysis status

- **Command:** `coach-eval census --corpus <dir> --adjudications <file> --out <report>`.
- **Each `census_repo`** is scanned with `coach codesignal --baseline --format json --no-interactive --fail-on-incomplete-coverage`, plus `--project-config` where the repository has a policy.
- **`analysis_status`** is recorded per rule and per repository, as one of:
  - `measured`;
  - `not_exercised` (with the named gap);
  - `not_adjudicable_only`;
  - `analysis_failed` (with the exit status and the report's `coverage` evidence);
  - `out_of_scope` (with the owner's decision).

  A repository that fails analysis never makes a rule `not_exercised`.
- **Fixture rows never enter census precision**, and never satisfy or defeat `not_exercised`.

**Two-repository red test** (#260 "Acceptance-test guidance"). The test runs a two-repository fixture corpus in which:
- one rule has 12 recorded adjudications and 0 false positives;
- one rule has zero findings;
- one repository is made to fail analysis.

It asserts three things:
- the report distinguishes "0 false positives out of 12 adjudicated" from `not_exercised`;
- the failing repository is `analysis_failed`, not `not_exercised`;
- a mutated rule threshold turns the suite red.

## 12. Mapping to #260

| #260 criterion (abridged) | What this design supplies |
| --- | --- |
| Per rule: repos, SHAs, finding count, adjudicated count, verdict counts | `census_repo` manifests and human adjudication rows, through `coach-eval census` |
| Zero findings → `not exercised`, with the gap | `analysis_status: not_exercised`. `analysis_failed` is kept separate, and fixture rows are excluded |
| Each record has a human, a date, the fingerprint and a rationale | `rater`, `date`, `target.fingerprint` (informational), `rationale`; the join key is section 11's |
| Empty evidence → `not_adjudicable` | That verdict. The density rules now copy `finding.Evidence` (`rule_*_density.go:29`); this needs a fresh check |
| Below threshold → a named disposition | Not supplied; the census report carries it |
| PRD updated | Not supplied. #260 names a "Success Signals" section that `docs/product/prd.md` no longer has (its gate is now §12), so #260 needs retargeting |
| Security recall from a seeded shape suite | Detection fixtures: per-shape recall, discrimination and known misses for TOCTOU (c.6, c.7). This is recall only |
| Pilot engineer action | Not supplied |
| A re-run reproduces the result | `source.sha`, `include`/`exclude`, and the Coach version and hash (c.3, c.5) |
| An unpinnable repository is excluded | `sha` is required |
| Harness false-green test | Section 11's two-repository red test and section 8's threshold mutation |

**Including `architecture.*` in #260 is an owner decision.** #253 is closed, but #260's scope boundary still excludes them.

## 13. What each result can feed

**A confirmatory H4/H2 result is necessary, but not sufficient,** for the living evaluation's agent self-review gate. PRD §12 says: "Until the evaluation carries a claim for agent self-review, that job is not judged". That gate stays unjudged until a restamp also runs the documented hook on a fresh real repository.

The new claim row on `docs/product/evaluations/codesignal-pilot-readiness.html` is "Agent self-review: controlled effect". Its fields:
- the permitted wording (decision 15.5);
- T;
- the effect with its 95% CI for each comparator;
- the cost ratio against `control` and against `generic_review`;
- the date;
- the HEAD the result was produced at.

The row may be written only if the cost ratio against `control` is at most 1.3, a hard gate (decision 15.4). Only `eval-agent-study` output can fill it. `eval-agent` panel results never appear in it.

| Result | Can feed | Cannot feed |
| --- | --- | --- |
| H4 with H2, confirmatory tier | The claim row above, scoped to the rule families Coach checks, fixture evidence and one model | The repeat-use test: PRD §4 says "a hook's scans do not" count |
| H3 | The pre-commit gate (`eval-detection`); per-shape recall and known misses for #260 and #205 | Precision, and the security precision gate, which PRD §12 requires "on a named corpus". Only #260's real corpus provides that |
| H1 (secondary) | Exploratory product evidence | Any release claim |
| `eval-agent` (development panel) | A developer's local feedback on a change, and drift tracking by comparing saved results across commits (`eval-compare`) | Any claim |

**Claims come only from `eval-agent-study`.**

## 14. Threats to validity

Grouped under the four headings in b.7, plus contamination.

**Construct**
- **H3 is conformance, not precision.** The same team writes the rules and the fixtures. Blinding the fixture authors reduces this but does not remove it (c.8).
- **H4 covers the rule families Coach checks:** layering, complexity, hidden input mutation, check-then-act and constructor coupling. **It says nothing about defects outside them**, such as correctness bugs, security beyond check-then-act, performance or naming. A coach-arm agent could trade a defect inside these families for one outside them. `introduced_defect_count` is always reported beside `g2_not_degenerate` and G1 so that a trade of that kind is visible.
- **Seeded defects are not natural defects** (c.6, c.7).
- **Cognitive complexity is only partly validated** (d.1, d.2).
- **Grader circularity** (d.7). It is mitigated by the non-Coach detectors and the `rules_in_prompt` comparator.
- **Manifest errors** (c.6). They are mitigated by two-human validation, `manifest_gap`, and the oracle and baseline checks.

**Internal**
- The extra turn and the rule text are isolated by `generic_review` (yoked) and `rules_in_prompt`.
- Coach's output tokens are covered by reporting cost beside every effect (b.5).
- Drift is controlled by pinning the model, CLI, image digest, Coach hash, hook script and settings. Drift over time is tracked by comparing saved results across commits (`eval-compare`); there is no scheduled run.
- Treatment leakage is controlled by the binary placement and by recording leakage.
- Faked grading is countered by an unprivileged runner, grades assembled outside the container, an all-IDs-pass rule, and a G2 guard. One gap remains: code run by the tests could forge runner output. Transcript reading covers it.

**External**
- Fixtures are simpler than real repositories (c.5). Claims stop at the templates' languages, sizes and trap kinds (b.7).
- One model and one harness (b.7).

**Conclusion**
- The template is the unit, with T − 1 degrees of freedom (a.7, b.1).
- Holm across H4a–c, plus the intersection-union claim rule with H2 (**our choice**).
- The protocol is preregistered (b.6).

**Contamination and leakage**
- Fixtures are written fresh (a.5, b.8).
- Their text carries no hints (b.8).
- Private material stays outside the sandbox (a.6 T.5, b.5).
- Hidden tests are checked against a wrong patch (b.8).

## 15. Owner decisions, with recommendations

1. **Confirmatory task family.** *Recommend:* the self-review hook on feature tasks (H4 with H2). Refactor on request is secondary.
2. **Model.** *Recommend:* pin the current flagship model at preregistration, and run one smaller model as an exploratory replication. Model IDs belong in the protocol, not here.
3. **Adjudicator capacity**, likely the binding constraint. *Our estimate:*
   - template validation: 105 × 2 seed sets × 2 raters × about 1.5 h ≈ 630 h;
   - H4 candidates plus hook findings: at an assumed 2 per trial and 50% deduplication, about 2,500 judgements, plus 20% doubly rated, at about 2 min each ≈ 100 h.

   *Recommend:* name the humans and their hours before preregistration, and cap T at that.
4. **Cost ceiling, a hard gate.** *Recommend:* the claim row is written only if `coach`'s mean cost per trial is at most 1.3× `control`'s (**our choice**). Above that, the result is reported in the run report and not as a claim.
5. **Permitted wording for a positive result.** *Recommend:* "In a controlled evaluation on T seeded Go and TypeScript feature templates with one model, agents running Coach's self-review hook introduced X (95% CI a–b) percentage points fewer adjudicated defects in the structural rule families Coach checks (layering, complexity, hidden input mutation, check-then-act, constructor coupling) than [comparator], with feature tests passing at a non-inferior rate." Do not use "prevents", "finds all" or any unscoped wording.
6. **T and the budget.** *Recommend:* pilot first, then fix T (section 5). Approve a standing `eval-agent` budget per developer, and approve each `eval-agent-study` run's budget separately (section 10).
7. **Thresholds marked our choice.** *Recommend:* ratify before preregistration:
   - the H2 margin of −0.05, with H2 at one-sided 0.025;
   - the 15-point minimum effect;
   - the line band [0.7, 1.5];
   - K = 3 with 2 instances per template, and K = 1 on impossible variants;
   - the t cutoff of 30;
   - κ ≥ 0.6, and 20% double rating;
   - a template size of 300–1500 lines;
   - B = $5;
   - the 1.3× gate;
   - refactor T = 24.
8. **`n/a` coverage cells.** *Recommend:* accept look-alike `must_not_fire` rows.
9. **`architecture.*` in #260.** *Recommend:* include them now that #253 is closed, scored by the exact edge sets.
10. **Where public fixtures live. This must be resolved before any fixture merges.** Coach's self-scan has no path exclusion (section 9). *Recommend:* keep public fixtures in a separate public repository, pinned by SHA and read from a local clone (`EVAL_FIXTURES_DIR`) by `eval-detection`, until a Coach path exclusion exists. That exclusion is a Coach behaviour change, so open its own issue. Only then move the fixtures to `eval/testdata/`. Either way, this repository's census manifest excludes `eval/testdata/**`.
11. **CI.** *Recommend:* leave it out of scope. `eval-detection` is fast, offline and deterministic, so it could later become a CI leaf in `status.needs` if the owner wants a shared gate. The agentic tasks stay local.

# Coach evaluation design: seeded fixtures, ground truth, and agentic evals

Status: draft for review, revision 2. This document is research and design only. It authorizes no fixture or harness code.

Conventions:
- Decisions marked **our choice** have no source behind them and need owner ratification.
- Every claim about outside practice cites a row in the source ledgers (section 1).
- A claim with no citation is about this repository, and names the file it was read from.
- Repository facts were read at `main` = `ed0ac13`.

## 1. Source ledger index

Four research tracks, each read from primary sources on 25 Nov 2026. Rows are cited as `a.N`, `b.N`, `c.N` and `d.N`. Tracks b and d number their sections `1.`, `2.` and so on, so `b.1` means section 1 of the track b ledger.

| Track | File | What it covers | Rows |
| --- | --- | --- | --- |
| a | [`sources/a-agentic-eval-design.md`](sources/a-agentic-eval-design.md) | Agent-eval structure, graders, isolation, SWE-bench and SWE-bench Verified, the Agentic Benchmark Checklist, error bars | a.1 Anthropic agent-eval post; a.2 Anthropic success-criteria docs; a.3 Inspect AI; a.4 SWE-bench; a.5 SWE-bench Verified; a.6 ABC; a.7 Miller, error bars |
| b | [`sources/b-experimental-rigour.md`](sources/b-experimental-rigour.md) | Statistics, pass@k and pass^k estimators, preregistration, validity, contamination, sizing | b.1 Miller; b.2 Codex/HumanEval; b.3 τ-bench; b.4 Anthropic agent-eval post; b.5 AI Agents That Matter; b.6 preregistration and registered reports; b.7 validity threats; b.8 SWE-Bench+ and contamination |
| c | [`sources/c-ground-truth.md`](sources/c-ground-truth.md) | How ground truth is built for smell, static-analysis and architecture tools | c.1 MLCQ; c.2 Landfill; c.3 Qualitas; c.4 Fontana and Di Nucci; c.5 OWASP Benchmark; c.6 NIST SATE VI and Juliet; c.7 mutants vs real faults; c.8 ArchLint; c.9 gaps |
| d | [`sources/d-refactoring-outcome.md`](sources/d-refactoring-outcome.md) | Measuring a refactoring's outcome | d.1 Cognitive Complexity; d.2 its validation; d.3 reflexion models; d.4 RefactorBench; d.5 LLM refactoring study; d.6 ImpossibleBench; d.7 specification gaming; d.8 RefactoringMiner |

Gaps the ledgers record, which this design fills with **our choice**:

- No source gives a number for how closely an LLM judge must agree with humans (a.1, a.2, a.3).
- None of the label sources reports inter-rater agreement (c.1, c.2, c.4, c.9). The second-rater protocol and its κ threshold are therefore our own choice, with no precedent behind them.
- No source recommends a number of trials per task (b.4; track b, "What was not obtainable").
- No track read Miller's cluster-adjusted power formula (Appendix C). This design does not rely on it: the analysis unit is the family, and power is computed on family means (section 5).

## 2. What is being evaluated

Three questions, each needing its own instrument.

1. **Detection (H3, deterministic).** Does `coach codesignal` report exactly the seeded defects in a fixture, and nothing on its near-misses and clean controls?
   - It is checked by acceptance tests against a committed manifest.
   - It gives per-shape recall, false-positive rate and discrimination (c.5, c.6).
   - The same team writes the rules and the fixtures, so this is **conformance to the team's own specification, not precision** (section 14).
2. **The self-review hook on feature tasks (confirmatory, H4 and H2).** This is the job the product ships. The hook in `docs/cli-codesignal.md` "## Agent self-review hook" runs `coach codesignal --worktree` when the agent finishes a turn and hands it the introduced findings. The question: does an agent given a feature request introduce fewer real structural defects with the hook than without it, without breaking the feature?
3. **Refactor on request (secondary, H1).** Asked to improve structure, does an agent that can run Coach leave a fixture closer to its target state, with behaviour intact?

The manifest, the adjudication record and the graders are shared by all three, and with #260's precision census (sections 11–12).

## 3. Hypotheses and variables

### Arms (independent variable)

| Arm | Feature family (confirmatory) | Refactor family (secondary) |
| --- | --- | --- |
| `control` | Base prompt; no `coach`; no hook | Base prompt; no `coach` |
| `coach` | Base prompt; pinned `coach` on `PATH`; the documented self-review Stop hook | Base prompt plus the coach sentence; pinned `coach` on `PATH` |
| `rules_in_prompt` | Base prompt plus the rule paragraph; no `coach`; no hook | Base prompt plus the rule paragraph; no `coach` |
| `generic_review` | Base prompt; no `coach`; a Stop hook that asks once for a generic maintainability review | not run |

Section 7 gives the exact text of every arm.

- `rules_in_prompt` is a confirmatory comparator. Comparing `coach` with it is what separates "Coach helps" from "Coach shows the agent the grader".
- `generic_review` gives the agent the same extra review turn the coach hook gives, without Coach's content. Using it as a comparator is **our choice**.

Everything else is held fixed across arms (b.7, internal validity): the fixture instance, the base prompt, the model ID, the CLI version, tool permissions, the per-trial budget and the toolchain image (section 7).

### Dependent variables

Per trial, after the agent exits (section 4 defines each term):

| Variable | Type | Family | Role |
| --- | --- | --- | --- |
| `introduced_defect`: at least one adjudicated true-positive structural defect introduced, found by **non-Coach** detectors | 0/1 | feature | **primary** (H4) |
| `introduced_defect_count` | count | feature | secondary |
| `g1_behaviour`: hidden and visible tests pass, visible tests unmodified | 0/1 | both | confirmatory non-inferiority (H2), feature family |
| `g2_not_degenerate` | 0/1 | both | secondary |
| `success` = G1 ∧ G2 ∧ every target predicate met | 0/1 | refactor | secondary (H1) |
| `predicate_score` (counted only when G1 and G2 pass) | 0–1 | refactor | secondary |
| `acted_on_fp`: the final diff changes a contested or adjudicated false-positive site | 0/1 | both | secondary (advice cost) |
| `g1_fail_at_fp_site`: a hidden test tagged to such a site fails | 0/1 | both | secondary |
| `tokens_on_fp`: tokens in turns whose tool calls touch those sites' files after the first hook message | number | feature | secondary |
| `gaming` on impossible variants (section 4) | 0/1 | both | secondary |
| `coach_findings_after`: introduced findings from a post-hoc Coach run, graded the same way in every arm | count | both | descriptive |
| per-function change in cognitive complexity, nesting depth and branch density | numbers | refactor | descriptive only (d.1, d.2) |
| `divergences_after`, `absences_after` against the layer model | counts | both | secondary (d.3) |
| input and output tokens, cost, wall-clock | numbers | both | reported with every effect (b.5) |

`unactionable_true_positive` is tracked per rule from the adjudication record (section 11).

### Hypotheses

Confirmatory family. Holm correction at a family-wise α of 0.05 across the four tests (the α and the Holm family are **our choice**):

- **H4a.** `coach` vs `control` on `introduced_defect`. Two-sided. H0: the paired mean difference is 0.
- **H4b.** `coach` vs `rules_in_prompt`, the same. Two-sided.
- **H4c.** `coach` vs `generic_review`, the same. Two-sided.
- **H2.** Non-inferiority of `coach` vs `control` on `g1_behaviour`. One-sided at 0.025, with margin −0.05. H0: the paired difference is at most −0.05. Rejecting H0 is equivalent to the lower bound of the two-sided 95% interval lying above −0.05. The margin is **our choice**.

The minimum effect of interest for H4 is 15 points on the proportion of trials that introduce a defect (**our choice**). It sets the family count (section 5).

Secondary family (refactor on request). Holm within the family, reported as secondary:

- **H1a.** `coach` vs `control` on `success`. Two-sided.
- **H1b.** `coach` vs `rules_in_prompt` on `success`. Two-sided.

**H3 (deterministic).** On every detection fixture, Coach's report satisfies section 8's row outcomes. No statistics are involved.

Everything else is exploratory and labelled that way in the report (b.6). That includes pass^k, per-language splits, H4 restricted to G1-passing trials, cost-normalised effects and a second-model replication.

## 4. Graders and definitions

Graders are code wherever possible (a.1, a.2). A later grader counts only once an earlier one has passed (d.5).

1. **G1, behaviour preserved.**
   - The grader-held hidden suite passes. In the feature family it includes the feature's FAIL_TO_PASS tests.
   - The visible tests pass, the PASS_TO_PASS analogue (a.4).
   - Every visible test file matches its recorded SHA-256 (a.6 T.5, d.6).
2. **G2, not degenerate** (d.6, d.7). Fail if any of these holds:
   - an `api_surface` symbol is missing or unreachable from its entry point;
   - non-test code lines fall outside [0.7, 1.5] × the original (**our choice**; d.6 proposes 0.7);
   - a reflexion absence appears (d.3);
   - a new suppression comment, `panic("TODO")`/`throw new Error("TODO")` stub, or `any`/`interface{}` widening appears on the API surface;
   - a **literal branch** appears (defined below).
3. **Target predicates** (refactor family). 3–8 private predicates per fixture, each its own subtest. They are AST or import-graph checks that do not depend on exact text (d.4). At least one per fixture is evaluated without Coach.
4. **Introduced-defect detectors** (feature family, H4). These are **non-Coach** detectors, which yield candidate defects in the agent's diff:
   - layer edges from an independent import-graph extraction (`go list -deps -json`; the TypeScript compiler's module resolution) checked against the fixture's layer model;
   - per-template AST predicates for hidden input mutation, check-then-act and constructor coupling, written from the contract (`docs/cli-codesignal.md`), not from Coach's code (c.8);
   - an independent cognitive-complexity and nesting implementation. Which tool is an owner input; it must first reproduce the detection fixtures' complexity rows.

   Humans adjudicate every candidate (section 11). Only `true_positive` counts toward `introduced_defect`.
5. **Diagnostics, never gates.** Metric deltas, RefactoringMiner on TS (d.8), and the post-hoc Coach findings.

### Definitions

- **Literal branch.** A new conditional (`if`, `switch`/`case`, ternary, or a map lookup used as a branch) whose condition compares against a literal that appears **only** in test inputs, visible or hidden, and nowhere in the original non-test code or the task text (d.6).
- **Line match.** A finding matches a manifest row when `rule_id` and `path` are equal, `subject` is equal where the row gives one, and the finding's lines overlap the row's `lines`.
  - The finding's lines run from `location.start_row + 1` to `location.end_row + 1`, because the report's rows are 0-based (`docs/cli-codesignal.md`).
  - A file-level finding, with a zero location, matches a row whose `lines` is `null`.
  - The fingerprint is informational only. It hashes the evidence string, which changes when code is edited (`pkg/codesignal/fingerprint.go:35`).
- **True negative and FPR.** The true-negative unit is a `must_not_fire` row.
  - A `must_not_fire` row with a matching finding is a false positive; without one it is a true negative.
  - FPR = FP / (FP + TN) over `must_not_fire` rows (c.5).
  - A clean-control file contributes one `must_not_fire` row per applicable rule, with `lines: null`.
- **Discrimination.** A seeded row counts only if it is hit and its twin is silent (c.6).
- **Gaming** on an impossible variant. A trial games if either of these holds:
  - the impossible predicate is met ∧ G1 fails; or
  - Coach findings at the seeded sites are gone ∧ (G1 fails ∨ G2 fails).

  Success requires G1, so success itself cannot measure gaming (d.6).
- **Acted on a false positive.** The final diff changes lines within a row whose `expect` is `contested`, or within a finding adjudicated `false_positive` or `unactionable_true_positive`.

### LLM judge

There is none on any confirmatory or secondary path. If an exploratory dimension needs one:
- it uses a different model from the agent (a.2);
- it judges one dimension per call, with a closed label set and a rubric (a.1, a.2);
- its settings are pinned (a.3);
- it is piloted against at least 50 human-labelled trials, and used only if κ ≥ 0.6. The pilot size and the threshold are **our choice** (a.1, a.2 give none).

## 5. Statistics and sizing

**The analysis unit is the family** (a.7, b.1). A family is a template: every instance generated from one template, together with every trial on those instances.

1. For each arm, average across the template's instances and trials to get one rate per template.
2. Take the paired difference d_t = rate_coach(t) − rate_comparator(t) for each template t.
3. The mean of d_t is the estimate. Its SE is sd(d_t)/√T, with T − 1 degrees of freedom (a.7, b.1).

Detail:
- **Intervals.** Use Student's t while T < 30 (b.5 used t for five runs; the cutoff of 30 is **our choice**) and 1.96 × SE otherwise (a.7).
- **Per-arm rates** are reported as mean ± SE, with T and the instance count beside each (a.7).
- **Consistency** is exploratory: pass^k per instance from n trials and c successes, as C(c,k)/C(n,k), then averaged by template (b.2, b.3). pass@k is not reported (b.5, d.5).
- **Sampling settings** stay at the model's defaults (a.7).
- **Cost** is reported per arm with every effect, as tokens, dollars and wall-clock (b.5).

Within-template draws, meaning instances × trials, only shrink the within-template part of the variance, and that has a ceiling (a.7, b.1). **T, not the instance count, sets power.**

### Sizing

The pilot runs the 12 hand-built templates (section 6). It validates the pipeline only (b.6):
- its outcomes are never reported as results;
- its templates never appear in the confirmatory set;
- its measured disagreement rate q and within-template variance are planning inputs only.

T for the confirmatory set comes from Miller's paired formula on template means (a.7, b.1). The formula is n = (z₀.₀₂₅ + z₀.₂)² · ω² / δ², with δ = 0.15 and ω² taken from the pilot. With b's placeholder ω² ≈ q:
- q = 0.3 gives T ≈ 105;
- q = 0.4 gives T ≈ 139.

This arithmetic is ours, from b's formula. The Holm adjustment raises T further for the lower-ranked tests. **Confirmatory T is therefore in the low hundreds of templates.** That cost dominates the whole design, and section 15 asks the owner to choose. If the affordable T is smaller, the preregistration states the minimum detectable effect it gives, and the report claims nothing smaller (a.7).

### Preregistration

Before the first confirmatory trial, commit a dated protocol (b.6). It states:
- H2, H3 and H4, plus the secondary hypotheses;
- the estimators, T, instances per template and K;
- the exclusions (section 7) and the stopping rule: all trials run, with no early stop.

Deviations are recorded with their reason, and a negative result is published (b.6).

## 6. Fixture specification

### Three kinds of fixture

- **Detection fixtures.** One-module repositories that exercise rules. They feed H3, the CI leaf and the recall half of #260.
- **Feature templates** (confirmatory). Each is a small application with:
  - a feature request;
  - hidden FAIL_TO_PASS tests for the feature (a.4);
  - visible PASS_TO_PASS tests;
  - a layer model;
  - **trap sites**: places where the natural implementation tends to introduce a structural defect. Examples: a handler that needs stored data, where the easy path imports the store directly across a forbidden edge; or branching business rules.
- **Refactor templates** (secondary). Each is a small application with seeded defects, target predicates, an oracle refactor and an impossible variant.

All are written fresh for this eval, not lifted from public code (a.5, b.8). Templates are held out: rule thresholds and prompts are never tuned on them (b.5).

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

A human confirms this table. Fixture authors then write from the contract, not from the code (c.8). An `n/a` cell carries a look-alike `must_not_fire` row instead of a seeded instance, for example a Go `../../../` import path.

### Detection fixtures, per applicable cell

- **Seeded positives.** At least one per catalogue shape, with `expect: fire`.
  - Recall is reported per shape (c.7).
  - TOCTOU uses #205's six shapes: two are `fire` and four are `known_miss`.
- **A fixed twin per positive**, with `expect: must_not_fire` (c.6).
- **Near-misses**, with `expect: must_not_fire`: code that resembles a shape, or that sits just inside a threshold (c.4).
  - Thresholds come from the contract or the code, never from Coach output. Examples: cognitive complexity 14 against a threshold of 15 (`docs/cli-codesignal.md`); `../../` against a depth threshold of 3 (`rule_deep_relative_import.go:8`).
  - For tight constructor init, the near-misses are #260's three non-matching shapes.
- **Clean controls.** Each clean-control file contributes `must_not_fire` rows (section 4).
- **At least one `contested` row per fixture**, where the pattern is present but the advice may be wrong. The model case is the out-parameter accumulator from #260. These rows are adjudicated, and given no fixed expectation (c.1).
- **The positive-to-negative ratio** is stated in the manifest and printed next to every score (c.4).

### Architecture fixtures

One per language. Each has a `.lousy-agents/coach.json` with at least three layers and one `required_layer`.
- **Every** cross-layer edge is listed: allowed edges, forbidden edges (divergences), layer-skipping edges (bypasses), and required edges (absences, checked by G2, not by Coach) (c.8, d.3).
- The layer model's author has not read the rule implementation (c.8).
- TS fixtures include a type-only import, a dynamic `import()`, a `require()` and a `paths` alias. `docs/cli-codesignal.md` says these are evaluated like value imports.

### Templates (feature and refactor)

- **Size.** 300–1500 non-test lines (**our choice**). That is complex enough that the do-nothing and delete baselines cannot pass (a.6 O.g.3).
- **No hints.** No comment, identifier, file name, commit message or test name may hint at a trap, a seeded defect or its fix (b.8).
- **At least one contested site per template.**
  - In a feature template, the natural implementation creates it. An example is a validator that appends to a violations slice passed in by the caller.
  - In a refactor template, it is pre-existing.
  - These sites feed `acted_on_fp`, `g1_fail_at_fp_site` and `tokens_on_fp`. The hidden tests that cover a contested site are tagged with its row ID.
- **Hidden suite.**
  - It must fail at least one deliberately wrong but plausible patch, checked before use (b.8).
  - A feature template's hidden suite must fail the unmodified tree, because the feature is missing.
- **Oracle.** A refactor template has an oracle refactor; a feature template has an oracle implementation. Each must score G1 = G2 = 1, and in the feature family it must introduce no defect (a.6 T.9). Oracles are private (d.4).
- **Impossible variant.** The target cannot be met without breaking behaviour.
  - In a refactor template, a predicate conflicts with a hidden test.
  - In a feature template, the feature's hidden test can only pass through an edge the policy forbids.
  - Impossible variants are reported separately, as the `gaming` rate (d.6).

### Generating instances from templates

Each template yields instances along two axes:

- **Seed sets.** For refactor templates, a seed set is which catalogue shapes fill the slots. For feature templates, it is which trap sites are active.
- **Surface variants.** Renaming identifiers and the domain vocabulary, moving files within a layer, reordering declarations, and formatting.

Generator rules:
- **Expectations come from the generator.** It maps each manifest row through the same transformation it applies to the code. It never derives an expectation by running Coach (#260 "False-green trap").
- **Every instance is validated automatically.**
  - The oracle must score 1.
  - The do-nothing and delete baselines must score 0. In the feature family, the delete baseline is replaced by a "stub the feature" baseline, which must fail G1.
  - The hidden suite must kill the mapped wrong patch (a.6, b.8).
  - A failing instance is dropped and counted.
- **Seeds sit in real control and data flow** (c.6).
- **Provenance.** `seed_method` is `hand_written` for template slots and `mutation` for generated variants (c.7).
- **Human review.**
  - Two humans validate every template and every seed set (a.5).
  - They also review a random 2 instances per template; the sample size is **our choice**.

Instances raise within-template precision only. Power comes from T (section 5).

### Protection against "pass by deleting code"

| Shortcut | Caught by |
| --- | --- |
| Delete the offending function | G2 API surface; hidden suite (a.4, d.7) |
| Delete the forbidden import together with the feature | G2 reflexion absence (d.3); FAIL_TO_PASS tests in the feature family |
| Shrink code below a threshold | G2 line-count band; hidden suite |
| Edit or weaken tests | Visible-test hashes; hidden suite outside the sandbox (a.6, d.6) |
| Special-case test inputs | G2 literal-branch rule (d.6) |
| Suppress or hide findings | G2 suppression check; non-Coach predicates and detectors |
| Do nothing, or stub the feature | Every predicate fails on the seeded tree; FAIL_TO_PASS tests fail (a.4, a.6) |

The oracle, do-nothing and delete/stub checks run on every instance before any confirmatory trial (a.1, a.6).

## 7. Harness contract

The harness follows Inspect's task = dataset + solver + scorer shape (a.3), but this contract does not require Inspect itself.

### Two sandboxes per trial

Both sandboxes are OCI containers run with Docker. Using gVisor's `runsc` runtime is recommended where available. All of this is **our choice**.

1. **Agent sandbox.**
   - Starts from the language's pinned toolchain image and a fresh copy of the instance.
   - Gets one initial commit with a neutral message, and no history from any other trial (a.1, a.3).
   - The only egress is through a proxy that allows the model API host. a.3 shows only that `network_mode: none` limits processes inside a container; it does not support this allowlist, so the allowlist is **our choice**.
   - The only credential is a per-run API key, issued for this eval with a spend cap enforced outside the agent, plus `--max-budget-usd` inside it (**our choice**).
   - The manifest, the hidden suite, the predicates, the oracle and the graders are never present (a.6 T.5, d.6).
2. **Grading sandbox.** A second fresh container, started from the same image.
   - It has no network and no credentials.
   - It gets the agent's final tree, plus that trial's hidden suite and predicates, mounted read-only.
   - It writes one JSON grade file and nothing else. The trial record is assembled outside both sandboxes.

### Toolchain images

- There is one pinned image per language: Go, and Node with TypeScript.
- Each is recorded by digest in every trial record.
- The image is identical across arms except for the `coach` binary, which is pinned by version and SHA-256 and present only in the `coach` arm's layer.
- Dependencies are vendored into the image, because the agent sandbox has no package-registry access (**our choice**).

### Agent invocation

The agent runs `claude -p` with these flags, checked against `claude --help` for Claude Code 2.1.332:
- `--model <pinned id>`;
- `--output-format stream-json`;
- `--no-session-persistence`;
- `--max-budget-usd <B>`;
- `--setting-sources user`, with `HOME` set to a fresh per-trial directory that holds only the arm's `.claude/settings.json`;
- `--strict-mcp-config` with an empty MCP configuration;
- the same `--allowedTools` list in every arm.

`--bare` is **not** used. `claude --help` says it skips "hooks (those defined in settings ...)", and the hook arms need their Stop hook. The trial record captures every hook run (its exit status and message). The pilot asserts that the Stop hook ran in `coach` and `generic_review`, and never ran in `control` or `rules_in_prompt`.

### Exact prompt text

The base prompts are fixed. `{feature_request}` is the template's request text, which is reviewed for hints (b.8).

**Feature family, base (all arms):**

```text
{feature_request}

Work in this repository. Keep the existing tests passing and do not modify existing test files; you may add new test files. Do not ask questions: make reasonable choices and finish.
```

Arm-specific parts:
- `control`: the base prompt only. No settings hooks.
- `coach`: the base prompt only. The arm's `settings.json` holds the Claude Code `Stop` entry from `docs/cli-codesignal.md` "## Agent self-review hook", byte for byte, at the pinned Coach SHA. `scripts/coach-self-review.sh` from that section is placed in the instance. The hook's message is the only Coach text the agent sees.
- `rules_in_prompt`: the base prompt, followed by:

  ```text
  Before you finish, check your change against these structural rules and fix any violation you introduce:
  {one line per rule: rule_id — what it flags — threshold, generated from docs/cli-codesignal.md "## Rule IDs" and its threshold table at the pinned Coach SHA, plus the layer policy's forbidden and required edges}
  ```

- `generic_review`: the base prompt only. The arm's `settings.json` holds a `Stop` hook that uses the same re-entry guard as the coach hook (`stop_hook_active`). It exits 2 once with this message:

  ```text
  Before you stop, review your uncommitted changes for maintainability problems — complexity, coupling, layering and hidden side effects — and fix any you find, or explain why not, then stop.
  ```

**Refactor family, base (all arms):**

```text
This repository has maintainability problems. Improve its internal structure without changing its behaviour. Keep the existing tests passing and do not modify existing test files. Do not ask questions: make reasonable choices and finish.
```

Arm-specific parts:
- `control`: the base prompt only.
- `coach`: the base prompt, followed by:

  ```text
  The coach CLI is installed. `coach codesignal --baseline` reports structural findings for the whole repository; `coach codesignal --worktree` reports findings in your uncommitted changes.
  ```

- `rules_in_prompt`: the base prompt, followed by the same rule paragraph as in the feature family.

Prompts are stored by SHA-256 in every trial record.

### Trial record (JSON, one per trial)

```json
{
  "schema_version": "1",
  "trial_id": "string",
  "family": "feature | refactor",
  "template_id": "string",
  "instance_id": "string",
  "instance_tree_sha": "string",
  "variant": "main | impossible",
  "arm": "control | coach | rules_in_prompt | generic_review | baseline_do_nothing | baseline_delete | baseline_stub | oracle",
  "epoch": 1,
  "agent": { "model": "string", "cli_version": "string", "flags": ["string"], "prompt_sha256": "string", "settings_sha256": "string" },
  "image_digest": "string",
  "coach": { "version": "string", "binary_sha256": "string" },
  "hooks": [{ "event": "Stop", "exit": 0, "message_sha256": "string" }],
  "budget_usd": 0,
  "usage": { "input_tokens": 0, "output_tokens": 0, "cost_usd": 0, "wall_seconds": 0, "tokens_on_fp": 0 },
  "exit": { "status": 0, "reason": "completed | budget | timeout | harness_error" },
  "grades": {
    "g1_behaviour": { "pass": true, "hidden_failed": ["string"], "visible_hash_mismatch": ["string"], "failed_at_fp_sites": ["row_id"] },
    "g2_not_degenerate": { "pass": true, "violations": [{ "check": "string", "detail": "string" }] },
    "predicates": [{ "id": "string", "pass": true, "evaluator": "coach | non_coach" }],
    "introduced_candidates": [{ "candidate_id": "string", "detector": "string", "rule_family": "string", "path": "string", "lines": [1, 2], "dedup_key": "string" }],
    "acted_on_fp": ["row_id"],
    "gaming": false,
    "success": true,
    "reflexion": { "divergences_before": 0, "divergences_after": 0, "absences_after": 0 },
    "metric_deltas": [{ "path": "string", "subject": "string", "metric": "string", "before": 0, "after": 0 }]
  },
  "transcript_path": "string"
}
```

`introduced_defect` is computed after adjudication, by joining `introduced_candidates` to `adjudications.jsonl` on `dedup_key`.

### Exclusions (pre-registered)

- A `harness_error` trial is re-run once. If it fails again, it is excluded and counted.
- A `budget` or `timeout` exit is graded as whatever tree the agent left.
- No other exclusion is allowed. Every exclusion is listed by trial ID (b.6).

### Reading transcripts

Before any number is published, a human reads a stratified random sample of transcripts: at least 5 per arm, every `gaming` trial, and every G2 failure (a.1, b.4).

### Harness acceptance tests (when the harness is built)

These follow `AGENTS.md`: Ginkgo, red first.
- The oracle scores 1. The do-nothing, delete and stub baselines score 0.
- A deliberately weakened hidden suite must let the delete baseline pass. This proves the gate carries weight.
- The grading sandbox must fail a grade attempt that tries to reach the network.

## 8. Detection acceptance tests (H3)

**Invocation:** from the fixture root, run
```sh
coach codesignal --baseline --format json --no-interactive --fail-on-incomplete-coverage
```
Architecture fixtures add `--project-config .lousy-agents/coach.json` (flags as in `docs/cli-codesignal.md`'s synopsis). An unanalyzed file then fails the run, instead of reading as clean.

Outcome per row, using section 4's line match:

| `expect` | The suite requires |
| --- | --- |
| `fire` | at least one matching finding of that rule; otherwise a miss |
| `must_not_fire` | no matching finding of that rule; otherwise a false positive |
| `known_miss` | no matching finding. If one appears, the suite fails with "known miss now detected: promote the row", which keeps #260's known-miss list honest |
| `contested` | no assertion. Whether it fired is recorded for adjudication |

Further rules:
- A finding that matches no row fails the suite until a human gives it a verdict: `false_positive`, or `manifest_gap` (c.6).
- The manifest is the source of truth and is never derived from Coach.
- A mutation check must turn the suite red: lowering `complexity.cognitive_complexity`'s threshold, or removing one detector shape (#260 "False-green trap").
- Coach's version and configuration are printed with every result (c.5).

## 9. Placement in this repository

- **Fixtures.** Public detection fixtures and the public tracking-panel templates (section 10) live under `eval/testdata/fixtures/{go,ts,tsx}/<id>/`, each with its own `go.mod` or `package.json`.
  - The Go command skips `testdata` directories and nested modules. Checked: `go list ./...` in a scratch module with both lists neither.
  - The source-layout check skips `testdata` (`internal/sourcelayout/check.go:22–25`), and Go root discovery skips it too (`pkg/projectmodel/go_root_discovery_walk.go:94`).
  - **Not established:** whether Coach's self-scan (`coach codesignal --baseline --project-config .lousy-agents/coach.json`) skips them. No `testdata` or path-exclusion handling was found in `internal/codesignalcli/gitrepo` or `pkg/codesignal`.
  - The PR that adds fixtures must show that the self-scan report is unchanged. If it is not, a path exclusion is a Coach behaviour change and needs its own issue.
- **Harness and graders.** `cmd/coach-eval`, `internal/evalharness` and `internal/evalgrade` join the tooling ring in `.lousy-agents/coach.json`, beside `cmd/source-layout-guard` and the other tooling commands. No production ring may import them.
  - The graders import no `pkg/` package.
  - Predicates evaluated by Coach call the pinned `coach` binary, so the graders never share code with the thing under test.
- **Private material.** Hidden suites, predicates, oracles and all confirmatory held-out templates live in a separate private repository, pinned by SHA in the protocol. This keeps them out of public training data (b.5, b.8) and out of every agent sandbox (a.6 T.5).

## 10. Tiers and cost

| Tier | What runs | When | Status of results |
| --- | --- | --- | --- |
| Detection | H3 on all detection fixtures, as `mise run eval-detection` | Every PR, as a new CI leaf added to `status.needs` | Regression gate |
| Tracking panel | The 12 pilot templates, 1 instance each, K = 3, four feature arms | Weekly schedule | Labelled **non-confirmatory**; for drift only |
| Confirmatory | A rotated held-out template set, never run before | Per release, when the owner funds it | The only tier that may support a claim |

The cost per trial is **our estimate**, not a measurement; the pilot replaces it.
- The cap is B = $5 per trial (`--max-budget-usd`).
- Planning assumes $2 per trial.

Example confirmatory run: T = 105 templates × 2 instances × K = 3 × 4 arms = 2,520 trials ≈ $5,000, with the cap bounding it at $12,600. A refactor secondary of 24 templates × 2 × 3 × 3 arms = 432 trials ≈ $860. The tracking panel is 12 × 3 × 4 = 144 trials ≈ $290 a week. The detection tier uses no model.

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
    "private_ref": { "repo": "string", "sha": "string", "path": "string" },
    "loc_band": [0.7, 1.5]
  }
}
```

- **Join key.** The join key is `(source.sha, rule_id, path, subject, lines)`, matched by line overlap (section 4). The `fingerprint` is carried as information only.
- **`contested` is an `expect` value only.** There is no separate flag.
- **Corpus pinning.** `source.sha`, `include` and `exclude` pin the corpus (c.2, c.3). `exclude` is where `vendor/` goes.
- **Sampling.** `sample` records #260's seed and its sampling frame.
- **Provenance.** `seed_method` records provenance (c.7).
- **Edge rows.** Edge rows list the whole layer graph (c.8).
- **Private material.** `template.private_ref` points into the private repository.

### `adjudications.jsonl`, append-only, one row per judgement

```json
{
  "record_id": "string",
  "target": {
    "kind": "finding | manifest_row | candidate",
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

- **One row per judgement.** Rows are never overwritten; a correction is a new row with `supersedes` (c.1, c.2).
- **Verdicts from #260.** `true_positive`, `false_positive`, `unactionable_true_positive` and `not_adjudicable` are #260's exactly.
- **Verdicts added here.**
  - `manifest_gap`: a correct finding with no matching row (c.6).
  - `row_confirmed` and `row_rejected`: fixture validation (a.5). A rejected row stays in the file (c.2).
- **Only humans count.** `agent_proposal` rows never enter a figure (#260).
- **Blinding.** Adjudicators do not see severity or confidence (c.4). For H4 candidates, they also do not see the arm or the detector that produced the candidate (**our choice**, by analogy with c.4).
- **Deduplication.** An H4 candidate is judged once per `dedup_key`: template, rule family, and a hash of the normalised hunk (**our choice**).
- **Second rater.** A second human rates every contested and every single-rated census finding (c.1), and a random 20% of H4 candidates (**our choice**). The report gives Cohen's κ per rule. κ < 0.6 marks that rule's figure as unreliable. This threshold is **our choice**, because no label source reports agreement (c.9).

### Census analysis status, one per rule

`analysis_status` takes one of these values:
- `measured`;
- `not_exercised`, with the named corpus gap;
- `not_adjudicable_only`;
- `out_of_scope`, with the owner's decision.

**Fixture rows never enter census precision, and never satisfy or defeat `not_exercised`.** Only `kind: census_repo` manifests feed the census.

## 12. Mapping to #260

| #260 criterion (abridged) | What this design supplies |
| --- | --- |
| Per rule: repos, SHAs, finding count, adjudicated count, verdict counts | `census_repo` manifests and human adjudication rows; counts are a query |
| Zero findings → `not exercised`, with the gap named | `analysis_status: not_exercised`. Fixture rows are excluded from this test |
| Each record has a human, a date, the fingerprint and a rationale | `rater`, `date`, `target.fingerprint` (informational), `rationale`. The join key is section 11's |
| Empty evidence → `not_adjudicable`, excluded from the denominator | The `not_adjudicable` verdict. The density rules now copy `finding.Evidence` (`rule_*_density.go:29`); whether it is still empty needs a fresh check |
| Below threshold → a named disposition | Not supplied; the census report carries it. #260's thresholds are unchanged |
| PRD updated with numbers | Not supplied. #260 names a "Success Signals" section that `docs/product/prd.md` no longer has (its headings run §1–§12; the gate is now §12); #260 needs retargeting |
| Security recall from a seeded shape suite | Detection fixtures: per-shape recall, discrimination and known misses for TOCTOU, using #205's shapes (c.6, c.7). This is recall only, never precision |
| Pilot engineer action | Not supplied; it stays out-of-band with #260 |
| A re-run reproduces the same set | `source.sha`, `include`/`exclude`, and the Coach version and hash per result (c.3, c.5) |
| An unpinnable repo is excluded | `sha` is required; there is no floating ref |
| Harness false-green test | Section 8's mutation check and section 7's baselines |

**Including `architecture.*` in #260 is an owner decision.** #253, the stated prerequisite, is closed, but #260's scope boundary still excludes those rules until the owner changes it.

## 13. What each result can feed

| Result | Can feed | Cannot feed |
| --- | --- | --- |
| H4 and H2, confirmatory tier | The living evaluation's agent self-review claim. PRD §12: "Until the evaluation carries a claim for agent self-review, that job is not judged". Scope it as fixture evidence, one model, seeded templates | The repeat-use test. PRD §4: "a hook's scans do not" count |
| H3 | A regression gate in CI; recall and known misses per shape for #260 and #205 | Precision, and the security precision gate. PRD §12 requires "a recorded precision review on a named corpus", and only #260's real corpus provides that |
| H1 (secondary) | Exploratory product evidence about refactor-on-request | Any release claim |
| Tracking panel | Drift alerts | Any claim |

## 14. Threats to validity

Organised under the four headings in b.7, plus contamination.

**Construct**
- **H3 is conformance, not precision.** The same team writes the rules and the fixtures, so H3 tests Coach against the team's own reading of its contract. Blinding fixture authors to the implementation reduces this but does not remove it (c.8).
- **Seeded defects are not natural defects** (c.6, c.7). Fixture recall is never read as recall on real code.
- **Cognitive complexity is only partly validated** (d.1, d.2), so metric deltas stay descriptive.
- **The coach arm could be optimising the grader** (d.7). H4 is therefore graded by non-Coach detectors, and `rules_in_prompt` tests "shows the grader" directly.
- **Manifests carry errors** (c.6). They are mitigated by two-human validation, `manifest_gap`, and the oracle and baseline checks.

**Internal**
- **Arms differ in more than Coach.** The `generic_review` and `rules_in_prompt` arms isolate the extra turn and the rule text.
- **Coach's output adds tokens.** Cost is reported with every effect (b.5).
- **Drift.** The model, CLI, image digest and Coach hash are pinned per trial.
- **Settings leakage.** Each trial gets a fresh `HOME`, `--setting-sources user` and `--strict-mcp-config`, and hook runs are recorded.

**External**
- **Fixtures are simpler than real repositories** (c.5). Claims stop at the templates' languages, sizes and trap families (b.7).
- **One model and one harness** (b.7). A second model is an exploratory replication.

**Conclusion**
- **The family is the unit, with T − 1 degrees of freedom** (a.7, b.1). T sets power, and the minimum detectable effect is stated.
- **Holm correction within each family**, and a preregistered protocol (b.6).

**Contamination and leakage**
- Fixtures are written fresh (a.5, b.8).
- Their text carries no hints (b.8).
- Private material stays in a private repository and outside the sandbox (a.6 T.5, b.5).
- Hidden tests are checked against a wrong patch (b.8).

## 15. Owner decisions, with recommendations

1. **Confirmatory task family.** *Recommend:* the self-review hook on feature tasks (H4, H2), with refactor on request as a secondary family.
2. **Model.** *Recommend:* pin the current flagship model at preregistration, and run one smaller model as an exploratory replication. Model IDs are recorded in the protocol, not in this document.
3. **Adjudicator capacity.** This is likely the binding constraint. *Our estimate* for the example run:
   - Template validation: 105 templates × 2 seed sets × 2 raters × about 1.5 h ≈ 630 h.
   - H4 candidates: at an assumed 1.5 candidates per trial and 50% deduplication, about 1,900 judgements, plus 20% doubly rated, at about 2 min each ≈ 75 h.

   *Recommend:* name the humans and their hours before preregistration, and cap T at what they can validate.
4. **Cost overhead ceiling.** *Recommend:* a positive H4 may be claimed as a product benefit only if `coach`'s mean cost per trial is at most 1.3× `control`'s (**our choice**). Otherwise the claim is reported together with the overhead.
5. **Permitted wording for a positive H4.** *Recommend:* "In a controlled evaluation on T seeded Go and TypeScript feature templates with one model, agents running Coach's self-review hook introduced X (95% CI a–b) percentage points fewer adjudicated structural defects than [comparator], with feature tests passing at a non-inferior rate." Do not use "Coach prevents", "Coach finds all" or any unscoped wording.
6. **T and the budget.** *Recommend:* run the pilot first, then fix T from it (section 5). Approve the budget per tier (section 10).
7. **Thresholds marked our choice.** *Recommend:* ratify as written or change before preregistration:
   - the H2 margin of −0.05;
   - the minimum effect of 15 points;
   - the line band [0.7, 1.5];
   - K = 3, with 2 instances per template;
   - the t cutoff of 30;
   - κ ≥ 0.6;
   - the 20% double rating;
   - a template size of 300–1500 lines;
   - B = $5;
   - the 1.3× ceiling.
8. **`n/a` coverage cells.** *Recommend:* accept look-alike `must_not_fire` rows where a rule cannot apply to a language.
9. **`architecture.*` in #260.** *Recommend:* include them now that #253 is closed, scored by the exact edge sets of section 6. This is the owner's decision.
10. **Self-scan exclusion for fixtures.** *Recommend:* if the fixture PR shows the self-scan picks up `eval/testdata`, open a separate issue for a path exclusion, rather than moving the fixtures out of the repository.

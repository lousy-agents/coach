# Coach evaluation design: seeded fixtures, ground truth, and an agentic refactor eval

Status: draft for review. This document is research and design only. It authorizes no fixture or harness code. Decisions marked **our choice** have no source behind them and need owner ratification. Every claim about outside practice cites a row in the source ledgers below; a claim with no citation is about this repository, and names the file it was read from.

Repository facts were read at `main` = `ed0ac13`.

## 1. Source ledger index

Four research tracks, each read from primary sources on 25 Nov 2026. Rows are cited as `a.N`, `b.N`, `c.N` and `d.N`. Tracks b and d number their sections `1.`, `2.` and so on; `b.1` means section 1 of the track b ledger.

| Track | File | What it covers | Rows |
| --- | --- | --- | --- |
| a | [`sources/a-agentic-eval-design.md`](sources/a-agentic-eval-design.md) | Agent-eval structure, graders, isolation, SWE-bench and SWE-bench Verified, the Agentic Benchmark Checklist, error bars | a.1 Anthropic agent-eval post; a.2 Anthropic success-criteria docs; a.3 Inspect AI; a.4 SWE-bench; a.5 SWE-bench Verified; a.6 ABC; a.7 Miller, error bars |
| b | [`sources/b-experimental-rigour.md`](sources/b-experimental-rigour.md) | Statistics, pass@k and pass^k estimators, preregistration, validity, contamination, sizing | b.1 Miller; b.2 Codex/HumanEval; b.3 τ-bench; b.4 Anthropic agent-eval post; b.5 AI Agents That Matter; b.6 preregistration and registered reports; b.7 validity threats; b.8 SWE-Bench+ and contamination |
| c | [`sources/c-ground-truth.md`](sources/c-ground-truth.md) | How ground truth is built for smell, static-analysis and architecture tools | c.1 MLCQ; c.2 Landfill; c.3 Qualitas; c.4 Fontana and Di Nucci; c.5 OWASP Benchmark; c.6 NIST SATE VI and Juliet; c.7 mutants vs real faults; c.8 ArchLint; c.9 gaps |
| d | [`sources/d-refactoring-outcome.md`](sources/d-refactoring-outcome.md) | Measuring a refactoring's outcome | d.1 Cognitive Complexity; d.2 its validation; d.3 reflexion models; d.4 RefactorBench; d.5 LLM refactoring study; d.6 ImpossibleBench; d.7 specification gaming; d.8 RefactoringMiner |

Gaps the ledgers record, which this design fills with **our choice**:

- No source gives a number for how closely an LLM judge must agree with humans (a.1, a.2, a.3).
- No smell data set in track c reports an inter-rater agreement statistic, so #260's agreement protocol has no precedent to copy (c.9).
- No source recommends a number of trials per task (b.4; track b, "What was not obtainable").
- Miller's cluster-adjusted power formula (Appendix C) was not read by any track (a.7, b.1).

## 2. What is being evaluated

Two different questions, which need different instruments:

1. **Detection.** Does `coach codesignal` report exactly the seeded defects in a fixture, and nothing on the near-misses and clean controls? The answer is deterministic. It is checked by acceptance tests against a committed manifest, and gives per-shape recall, false-positive rate and discrimination (c.5, c.6).
2. **Effect on an agent.** Does an agent that can run Coach leave a fixture closer to its target state, with behaviour intact, than the same agent without Coach? The answer is stochastic. It is measured by repeated trials with a paired no-Coach control (a.1, a.7, b.1).

The manifest, the adjudication record and the graders are shared by both, and by #260's precision census (section 9).

## 3. Hypotheses and variables

### Independent variable

`arm`, a property of the trial:

- **coach**: the `coach` binary is on `PATH`, and the prompt gets one sentence saying it is installed and what `coach codesignal --worktree` does.
- **control**: no `coach` binary, no network route to fetch one, and no sentence about it.

Everything else is held fixed between arms (b.7, internal validity): fixture copy, task prompt, model ID, tool permissions, budget, and harness version.

An optional third arm, **rules-in-prompt** (no binary, but the prompt lists the rule names and thresholds from `docs/cli-codesignal.md`), separates the effect of the tool from the effect of being told what to look for. It is exploratory unless the pre-registration promotes it (b.6).

### Dependent variables

Per trial, after the agent exits (section 6 defines each grader):

| Variable | Type | Role |
| --- | --- | --- |
| `success` = gate G1 ∧ gate G2 ∧ every target predicate met | 0/1 | **primary** |
| `g1_behaviour` (hidden and visible tests pass, visible tests unmodified) | 0/1 | secondary (non-inferiority) |
| `g2_not_degenerate` | 0/1 | secondary |
| `predicate_score`, the fraction of target predicates met, counted only when G1 and G2 pass | 0–1 | secondary |
| `divergences_after`, `absences_after` (reflexion classes against the fixture's layer model) | counts | secondary |
| per-function change in cognitive complexity, nesting depth and branch density | numbers | descriptive only (d.1, d.2) |
| `introduced_findings` from a post-hoc Coach run (graded in both arms) | count | descriptive |
| input and output tokens, cost, wall-clock | numbers | reported with every effect (b.5) |

### Hypotheses (confirmatory)

- **H1 (primary).** On the held-out fixture set, the mean per-fixture success rate is higher in the coach arm than in the control. H0: the paired mean difference is 0. Two-sided test, α = 0.05.
- **H2 (non-inferiority).** Coach does not make behaviour preservation worse: the lower 95% bound of the paired difference in `g1_behaviour` (coach minus control) is above −0.10. The margin is **our choice**.
- **H3 (detection; deterministic, not statistical).** On every detection fixture, Coach reports every row marked `fire` and no row marked `must_not_fire`, and reports nothing on clean controls. This hypothesis is checked by the acceptance suite in section 7.

Everything else (pass^k, the rules-in-prompt arm, cost-normalised effect, per-language splits) is exploratory and labelled that way in the report (b.6).

## 4. Metrics, graders and statistics

### Grader order

Graders are code wherever possible (a.1, a.2). The order matters, because a later grader is only meaningful once an earlier one has passed (d.5):

1. **G1, behaviour preserved.** The grader-held hidden suite runs against the agent's tree. Visible test files must match their recorded SHA-256 (a.6 T.5, d.6). Both must pass, as in SWE-bench's FAIL_TO_PASS and PASS_TO_PASS (a.4).
2. **G2, not degenerate** (d.6, d.7). Fail if any of these holds:
   - an `api_surface` symbol from the manifest is missing or unreachable from its entry point;
   - non-test code lines fall outside [0.7, 1.5] × the original (the band is **our choice**; d proposes 0.7 as an example);
   - a reflexion absence appears (d.3);
   - new suppression comments, `panic("TODO")`/`throw new Error("TODO")` stubs, or `any`/`interface{}` widening appear on the API surface;
   - code branches on a literal that appears in the hidden or visible test inputs.
3. **Target predicates.** Each refactor fixture has 3–8 private predicates, each a separate subtest (d.4). Predicates are written as AST or import-graph checks that do not depend on exact text (d.4). At least one predicate per fixture is evaluated by a tool other than Coach, so success is not defined only by Coach's own output (section 10, construct validity).
4. **Diagnostics, never gates.** Metric deltas, RefactoringMiner's refactoring types on TS fixtures (d.8), and Coach's post-hoc findings.

### LLM judge

There is none on the primary path. If an exploratory dimension, such as "does the change address the finding in a way a reviewer would accept", needs a model judge, then:

- it uses a different model from the agent (a.2);
- it judges one dimension per call with a closed label set and a written rubric (a.1, a.2);
- its settings are pinned (a.3);
- it runs a pilot against at least 50 human-labelled trials before it is used, and is used only if Cohen's κ against the human labels is at least 0.6 (the sample size and threshold are **our choice**; no source gives one, a.1, a.2).

### Statistics

- **Unit of analysis: the fixture, not the trial** (a.7, b.1). Each fixture's K trials per arm are averaged into one per-fixture rate.
- **Primary estimate:** the mean over fixtures of the paired difference d_f = rate_coach(f) − rate_control(f), with SE = sd(d_f)/√F (a.7, b.1).
  - The interval uses Student's t with F−1 degrees of freedom while F < 30 (b.5), and 1.96 × SE otherwise (a.7).
  - Where fixtures share a seeded repository ("family"), the SE is clustered on the family (a.7, b.1).
- **Per-arm rates** are reported as mean ± SE, with the fixture count beside each (a.7).
- **Consistency:** pass^k for k = 1…K, computed with the unbiased estimator C(c,k)/C(n,k) from each fixture's n trials and c successes, then averaged across fixtures (b.2, b.3). pass@k is not reported: it rewards retrying, and the product is a single run (b.5, d.5).
- **Trials per fixture: K = 5** (**our choice**). No source gives a number (b.4); b.5 used five runs. Miller's example shows more resampling has a ceiling (b.1).
- **Sampling settings stay at the model's defaults.** Temperature is not lowered to steady the numbers (a.7).
- **Secondary hypotheses** use Holm correction across H2 and any promoted exploratory test (**our choice**).
- **Cost** is reported with every effect: tokens and wall-clock per trial for each arm, and the effect per unit of cost (b.5).

### Sizing

The sizing table in b ("Sizing") shows that a paired design needs about 79 fixtures to detect a 20-point gap when the arms disagree on 40% of fixtures. It needs about 236 fixtures for a 10-point gap at a 30% disagreement rate. Those are placeholders, not measurements.

A hand-built set cannot reach that size, and adding trials per fixture cannot make up for too few fixtures (b.1). So the confirmatory set is **generated from templates** (section 5, "Generating refactor fixtures"), and the hand-built fixtures are the templates themselves.

The plan:

1. **Pilot: pipeline validation only.** Run each template's base instance (12 templates, 4 per language) × K = 5 × 2 arms, plus the oracle and both baselines.
   - The pilot shows that every stage works end to end: sandbox, agent run, graders, trial records and the analysis script.
   - Its success rates are never reported as a result and never count toward the confirmatory test (b.6).
   - Its disagreement rate q and within-fixture variance are recorded as planning inputs for step 2 only.
2. **Power calculation.** Use Miller's paired formula (a.7, b.1) with the pilot's q to fix F for 80% power at the minimum effect the owner names.
   - The b sizing table's placeholders put F at about 80–100 for a 20-point gap.
   - Generated instances that share a template are clustered, so the effective sample lies between the template count T and F. Miller's cluster-adjusted formula (Appendix C) must be read before F is fixed; no track has read it yet (a.7, b.1).
   - If the F the budget allows is smaller, the protocol states the minimum detectable effect it gives, and the report claims nothing smaller (a.7).
3. **Preregistration.** Commit a dated protocol file before the first confirmatory trial (b.6). It states H1–H3, the estimators, F, K, the exclusion rules (section 6) and the stopping rule (all F × K trials, no early stop). Any deviation is recorded with its reason, and a negative result is published (b.6).

## 5. Fixture specification

### Two kinds of fixture

- **Detection fixtures** are small, one-module repositories that exercise rules. They feed H3, the acceptance suite, and the recall half of #260's census.
- **Refactor fixtures** are realistic small applications with a few seeded defects, behaviour tests, a target state, an oracle refactor and an impossible variant. They feed H1 and H2.

Both are written fresh for this eval, not lifted from public repositories, so neither the fixtures nor their answers are in any model's training data (a.5, b.8). Refactor fixtures are held out: rule thresholds and prompts are never tuned on them (b.5).

### Rule × language coverage

The rule inventory is the CLI contract's table, which the PRD points to (`docs/product/prd.md:357` → `docs/cli-codesignal.md` "## Rule IDs"). That table lists 10 file-local rules and 2 architecture rules.

Applicability was read from the code, not found by running Coach:

| `rule_id` | Go | TS | TSX | Where applicability was read |
| --- | --- | --- | --- | --- |
| `complexity.cognitive_complexity` | ✓ | ✓ | ✓ | metric rule, all languages |
| `complexity.max_nesting_depth` | ✓ | ✓ | ✓ | metric rule, all languages |
| `complexity.branch_density` | ✓ | ✓ | ✓ | metric rule, all languages |
| `state.hidden_input_mutation` | ✓ | ✓ | ✓ | `pkg/semantics/go_mutates_input.go`, `ts_mutates_input.go` |
| `security.toctou_check_then_act` | ✓ | ✓ | ✓ | `pkg/semantics/go_toctou.go`, `ts_toctou.go` |
| `structure.constructor_density` | ✓ | n/a | n/a | `pkg/semantics/go_constructor_funcs.go` only |
| `structure.pointer_return_density` | ✓ | n/a | n/a | `pkg/semantics/go_pointer_returns.go` only |
| `coupling.tight_constructor_init` | n/a | ✓ | ✓ | `pkg/semantics/ts_tight_coupling.go` only |
| `coupling.deep_relative_import` | n/a | ✓ | ✓ | `pkg/codesignal/rule_deep_relative_import.go:45` returns early for Go |
| `structure.react_component_orchestration_density` | n/a | n/a | ✓ | `pkg/semantics/react_components_candidacy.go:22` requires a JSX body, which `.ts` cannot hold |
| `architecture.layer_violation` | ✓ | ✓ | ✓ (as TS) | `pkg/codesignal/rule_layer_violation*.go` |
| `architecture.layer_bypass` | ✓ | ✓ | ✓ (as TS) | `pkg/codesignal/rule_layer_bypass*.go`; requires `required_layer` in the policy |

A human confirms this table before any fixture is built. For an `n/a` cell, a seeded instance is impossible, so the fixture instead carries a look-alike row marked `must_not_fire`. Examples:
- a Go `../../../` import path;
- a TS function returning many object references, the look-alike for `pointer_return_density`;
- a `.ts` file with component-shaped functions.

### Per-language content of the detection fixtures

For each applicable rule × language cell:

- **At least one seeded positive per shape** in the seeding catalogue, marked `fire`.
  - Recall is reported per shape, never as one number per rule (c.7).
  - TOCTOU uses #205's six shapes: the two that match are `fire`, and the four that do not are `known_miss`. A `known_miss` row records a shape Coach is known not to cover, so it counts toward the known-miss list #260 asks for, not as a false negative to fix.
- **A fixed twin for each positive**, marked `must_not_fire`: the same code with the defect removed. It is used to score discrimination, which counts a hit only if the twin is silent (c.6).
- **Near-misses**, marked `must_not_fire`: code that resembles a shape but is correct, or falls just inside a threshold (c.4).
  - Thresholds are read from the code or the contract, never from Coach output. Examples: cognitive complexity 14 against a threshold of 15 (`docs/cli-codesignal.md` threshold table); a `../../` import against a depth threshold of 3 (`pkg/codesignal/rule_deep_relative_import.go:8`).
  - For tight constructor init, the near-misses are the three shapes #260 names as non-matching: a field initializer, local-then-assign, and a factory call.
- **Clean controls**: files with no seeded site. Any finding in them is a false positive.
- **Contested rows**, marked `contested: true`, where the pattern is present but the advice may be wrong. The `lousy-iam` out-parameter accumulator in #260 is the model case. These rows are adjudicated, not given a fixed expectation (c.1).
- **The ratio of positives to negatives**, stated in the manifest and printed next to every score (c.4).

### Architecture rules

Each language has a fixture with a `.lousy-agents/coach.json` that declares at least three layers and sets one `required_layer`. Its manifest lists **every** cross-layer import edge in the fixture, allowed and forbidden, so recall is exact rather than sampled (c.8).

- Forbidden edges are divergence rows (`fire` for `architecture.layer_violation`).
- A layer-skipping edge is a bypass row (`fire` for `architecture.layer_bypass`).
- Required edges are absence rows. Coach does not check absences; the G2 grader does (c.8, d.3).
- The layer model is written by someone who has not read the rule implementation (c.8).
- TS fixtures include a type-only import, a dynamic `import()`, a `require()` and a `paths` alias. `docs/cli-codesignal.md` says each of these is evaluated like a value import.

### Refactor fixtures

Each refactor fixture is a small application of 300–1500 non-test lines (**our choice**). It needs to be complex enough that a do-nothing or delete-everything agent cannot pass (a.6 O.g.3).

Its contents:

- **Seeded defects:** 2–5 rows from the detection catalogue, in realistic code. No comment, identifier, file name, commit message or test name may hint at a seeded defect or its fix (b.8).
- **Visible tests**, the PASS_TO_PASS analogue (a.4), with SHA-256 hashes recorded in the manifest.
- **A hidden suite** for the public API, kept outside the agent's sandbox (d.6). It must fail at least one deliberately wrong but plausible refactor, which is checked before use (b.8).
- **An API surface:** the exported symbols and entry points G2 protects.
- **Target predicates:** 3–8 private predicates (d.4), all failing on the seeded tree and all passing on the oracle.
- **An oracle refactor:** a reference tree that must score `success = 1` (a.6 T.9). It is kept private (d.4).
- **An impossible variant:** the same fixture with one predicate that cannot be met without breaking a hidden test. Any success on it means gaming (d.6). Impossible variants are reported apart from the main fixtures and never count toward H1.

### Generating refactor fixtures from templates

The confirmatory set is a **template × seed set × surface variant** matrix:

- **Template (T).** A hand-written refactor fixture as specified above: an application, its API surface, visible and hidden tests, a layer model, and slots where defects can be seeded. The 12 pilot fixtures are the 12 templates.
- **Seed set (S).** Which catalogue shapes fill the template's slots, chosen from the rules that apply to its language. Each seed set has its own target predicates and oracle, derived from the template's per-slot predicates and per-slot oracle fragments.
- **Surface variant (V).** Transformations that leave the seeded structure unchanged: renaming identifiers and the domain vocabulary, moving files within a layer, reordering declarations, and formatting.

**Example size.** 12 templates × 2 seed sets × 4 variants = 96 instances, within the 80–100 range. The counts are **our choice** and are owner inputs (section 11).

**Rules for the generator:**
- **Expectations come from the generator, not from Coach.** The generator maps each manifest row's path, subject and line range through the same transformation it applies to the code. No expectation is derived by running Coach (#260 "False-green trap").
- **Every instance is validated automatically before use.** The oracle must score 1, and the do-nothing and delete baselines must score 0. The hidden suite must fail the template's wrong-but-plausible patch, mapped through the variant (a.6, b.8). An instance that fails any check is dropped, counted, and its template is fixed.
- **Humans validate every template and a sample of instances.** Two humans validate each template and each seed set (a.5). They also check a random sample of 2 instances per template; the sample size is **our choice**. Their judgements are recorded as adjudication rows (section 8).
- **Seeds must sit in real control and data flow.** Injected defects are found at different rates from natural ones, and SATE lists being "embedded in existing control and data flows" as a quality criterion for injected bugs (c.6). Each seed slot is written into code the application actually runs.
- **Every row records its seed method** (`hand_written` for a template slot, `mutation` for a generated variant), so recall can be split by provenance (c.7).
- **Instances are clustered by template.** Pairing is still per instance: both arms run on the same instance. The SE is clustered on the template (a.7, b.1).

### Protection against "pass by deleting code"

Each layer catches a different shortcut:

| Shortcut | Caught by |
| --- | --- |
| Delete the offending function | G2 API surface; hidden suite (a.4, d.7) |
| Delete the forbidden import together with the feature | G2 reflexion absence (d.3) |
| Shrink the file under a metric threshold by removing code | G2 line-count band; hidden suite |
| Edit or weaken tests | Visible-test hashes; hidden suite outside the sandbox (a.6, d.6) |
| Special-case test inputs | G2 literal-branch check; hidden inputs differ from visible ones (d.6) |
| Suppress or hide findings | G2 suppression check; at least one non-Coach predicate per fixture |
| Do nothing | Every predicate fails on the seeded tree, so the do-nothing baseline must score 0 (a.6) |

Two baseline agents run on every fixture before the confirmatory run: **do-nothing** (exits immediately) and **delete** (removes every function body named in a seeded row). Both must score `success = 0`. The oracle must score `success = 1`. A fixture that fails any of these three checks is broken and is fixed before use (a.1, a.6).

### Human validation of fixtures

Every seeded row, near-miss, predicate and hidden test is reviewed by two humans independently. Where their labels differ, the stricter one is kept (a.5). Each judgement is recorded as an adjudication row (section 8). A fixture whose target predicates reject a valid refactor that a reviewer can show is fixed or dropped (a.5).

## 6. Harness contract

This is the contract a future harness implements. Its shape follows Inspect's task = dataset + solver + scorer (a.3): a sample is a fixture plus a prompt, the solver is a headless agent run, and the scorer is section 4's graders. Running on Inspect itself is allowed but not required.

### Per trial

1. **Isolation.** Create a fresh sandbox from the fixture's pinned tree. It gets one initial commit with a neutral message and no history from any other trial (a.1, a.3). There is no network except the model API endpoint (a.3). The manifest, hidden suite, oracle and graders are never inside the sandbox (a.6 T.5, d.6).
2. **Arm setup.** In the coach arm, a pinned `coach` binary (a version and a SHA-256) is on `PATH`. In the control arm, there is no `coach` binary and no installer.
3. **Agent run.** Run `claude -p` with these flags, which were checked against `claude --help` for Claude Code 2.1.332:
   - `--bare`, so no user or project hooks, CLAUDE.md or plugins are loaded and the arms differ only in what this contract sets;
   - `--model <pinned id>`;
   - `--output-format stream-json`;
   - `--no-session-persistence`;
   - `--max-budget-usd <B>`;
   - the same `--allowedTools` list in both arms.

   The prompt is identical across arms apart from the coach sentence. It is stored by hash. It does not name any rule, defect or file (b.8).
4. **Capture.** Keep the full transcript, token counts, cost, wall-clock time, exit status and the final tree diff (a.1, a.3, b.5).
5. **Grade.** Outside the sandbox, run G1 → G2 → predicates → diagnostics on the final tree, and write one trial record.

### Trial record (JSON, one per trial)

```json
{
  "schema_version": "1",
  "trial_id": "string",
  "fixture_id": "string",
  "fixture_tree_sha": "string",
  "variant": "main | impossible",
  "arm": "coach | control | rules_in_prompt | baseline_do_nothing | baseline_delete | oracle",
  "epoch": 1,
  "agent": { "model": "string", "cli_version": "string", "flags": ["string"], "prompt_sha256": "string" },
  "coach": { "version": "string", "binary_sha256": "string" },
  "budget_usd": 0,
  "usage": { "input_tokens": 0, "output_tokens": 0, "cost_usd": 0, "wall_seconds": 0 },
  "exit": { "status": 0, "reason": "completed | budget | timeout | harness_error" },
  "grades": {
    "g1_behaviour": { "pass": true, "hidden_failed": ["string"], "visible_hash_mismatch": ["string"] },
    "g2_not_degenerate": { "pass": true, "violations": [{ "check": "string", "detail": "string" }] },
    "predicates": [{ "id": "string", "pass": true, "evaluator": "coach | non_coach" }],
    "success": true,
    "reflexion": { "divergences_before": 0, "divergences_after": 0, "absences_after": 0 },
    "metric_deltas": [{ "path": "string", "subject": "string", "metric": "string", "before": 0, "after": 0 }]
  },
  "transcript_path": "string"
}
```

### Exclusions (pre-registered)

- A trial with `exit.reason = harness_error` is re-run once. If it fails again, it is excluded and counted.
- A `budget` or `timeout` exit is **not** excluded. It is graded as whatever tree the agent left.
- No other exclusion is allowed. Every exclusion is listed by trial ID in the report (b.6).

### Reading transcripts

Before any number is published, a human reads a stratified random sample of transcripts: at least 5 per arm, including every impossible-variant success and every G2 failure (a.1, b.4). Findings from that reading go into the threats section of the report.

### Harness acceptance tests (when the harness is built)

These follow `AGENTS.md`: Ginkgo, red first.

- The oracle scores 1, the do-nothing baseline scores 0, and the delete baseline scores 0, on a two-fixture corpus.
- A deliberately weakened hidden suite must let the delete baseline pass. This proves the gate is load-bearing.

## 7. Acceptance tests on fixtures (detection, H3)

The suite runs the built `coach codesignal` on each detection fixture and compares its JSON report with the committed manifest.

- **The manifest is the source of truth, and Coach is the thing under test.** The suite never derives its expectations by running Coach. That is #260's false-green trap.
- **Matching uses `(rule_id, path, subject)` plus the line range,** not the fingerprint. The fingerprint hashes the evidence string (`pkg/codesignal/fingerprint.go:35`), and the evidence carries the metric value, so it changes when the code is edited.
- **Every unmatched finding fails the suite** until a human gives it a verdict: either a false positive or a `manifest_gap`, which means a missing row (c.6). The manifest is itself an artefact under test (c.6).
- **A mutation check proves the suite can fail:** lowering `complexity.cognitive_complexity`'s threshold, or removing one detector shape, must turn it red (#260 "False-green trap").
- **Coach's configuration and version are printed with every result** (c.5).

## 8. Ground-truth manifest and adjudication schema (shared with #260)

Two files, used by the fixtures and by the census alike.

### `manifest.json`, one per fixture or census corpus entry

```json
{
  "schema_version": "1",
  "id": "go-orders-01",
  "kind": "detection_fixture | refactor_fixture | census_repo",
  "language": ["go"],
  "source": { "repo": "string", "sha": "string", "include": ["glob"], "exclude": ["glob"] },
  "positive_negative_ratio": "string",
  "layer_model": { "policy_path": ".lousy-agents/coach.json", "author_blind_to_rule_impl": true },
  "rows": [
    {
      "row_id": "string",
      "rule_id": "complexity.cognitive_complexity",
      "expect": "fire | must_not_fire | known_miss | contested",
      "row_kind": "seeded | twin | near_miss | lookalike_na | clean_control | edge_allowed | edge_forbidden | edge_required",
      "path": "string",
      "subject": "string",
      "lines": [12, 48],
      "shape": "catalogue id",
      "twin_of": "row_id or null",
      "seed_method": "real_fix | mutation | hand_written",
      "rationale": "string"
    }
  ],
  "refactor": {
    "api_surface": [{ "symbol": "string", "entry": "string" }],
    "visible_tests": [{ "path": "string", "sha256": "string" }],
    "hidden_suite_ref": "string (outside the repo the agent sees)",
    "predicates_ref": "string (private)",
    "oracle_ref": "string (private)",
    "loc_band": [0.7, 1.5]
  }
}
```

Field notes:

- `source.sha`, `include` and `exclude` pin the corpus, so a re-run scans the same files (c.2, c.3). `exclude` is where `vendor/` goes; 60 of 79 findings in one #260 repository were vendored code.
- `seed_method` records provenance, and the seeding catalogue lists the shapes it cannot express (c.7). Census rows use `real_fix` or leave the field out.
- Edge rows enumerate the full layer graph (c.8).
- `refactor` exists only for `kind = refactor_fixture`. Its `*_ref` paths point outside anything the agent can read (a.6 T.5).

### `adjudications.jsonl`, append-only, one row per judgement

```json
{
  "record_id": "string",
  "target": {
    "kind": "finding | manifest_row | trial",
    "corpus_id": "string", "sha": "string",
    "fingerprint": "string or null", "rule_id": "string", "path": "string", "subject": "string",
    "row_id": "string or null", "trial_id": "string or null"
  },
  "verdict": "true_positive | false_positive | unactionable_true_positive | not_adjudicable | manifest_gap | row_confirmed | row_rejected",
  "rater": { "id": "string", "kind": "human | agent_proposal" },
  "date": "YYYY-MM-DD",
  "rationale": "one line",
  "blind_to": ["severity", "confidence"],
  "supersedes": "record_id or null"
}
```

- **One row per judgement, never one merged verdict per finding,** so agreement can be computed afterwards. Rows are never overwritten; a correction is a new row with `supersedes` (c.1, c.2).
- **The first three verdicts and `not_adjudicable` are #260's exactly.** A finding with empty `evidence` gets `not_adjudicable` and leaves the denominator (#260).
- **`manifest_gap` is added for a correct finding that matches no row** (c.6).
- **`row_confirmed` and `row_rejected` record the two-human validation of fixture rows** (a.5). Rejected rows stay in the file, marked as rejected (c.2).
- **Only `rater.kind = human` counts.** An agent may write `agent_proposal` rows, as #260 allows, but they never enter a precision figure.
- **Adjudicators do not see the finding's severity or confidence** (c.4).
- **A second human rates every contested finding and every finding that has only one rating** (c.1). The report gives Cohen's κ per rule over the doubly rated set. A κ below 0.6 marks that rule's precision as unreliable (**our choice**; no precedent in the ledgers, c.9).

## 9. Mapping to #260

| #260 criterion (EARS, abridged) | What this design supplies |
| --- | --- |
| Per rule: repos, pinned SHAs, finding count, adjudicated count, verdict counts | `manifest.json` (`kind: census_repo`, `source.sha`) plus `adjudications.jsonl`; counts are a query over human rows |
| A rule with zero findings is `not exercised`, with the gap named; no precision | The census report derives it from the manifest. Detection fixtures guarantee every rule has at least one seeded positive, so a zero there is a detector failure, never a corpus gap |
| Each record has a human identity, date, fingerprint and rationale | `rater`, `date`, `target.fingerprint`, `rationale` |
| Empty evidence → `not_adjudicable`, excluded from the denominator, counted per rule | The `not_adjudicable` verdict. Whether the density rules still emit empty evidence needs a fresh check before the census; `rule_*_density.go:29` now copies `finding.Evidence` |
| Below threshold → named disposition | Out of scope here; the census report carries it. Thresholds (0.90 security, 0.70 structural) are #260's, unchanged |
| PRD capability table updated with numbers and date | Out of scope here (owner and protected-file rule) |
| Security rules: recall from a seeded shape suite before any claim | Detection fixtures give per-shape recall, discrimination and known misses for `security.toctou_check_then_act`, using #205's six shapes (c.6, c.7) |
| Pilot engineer action recorded, or `no usage observed` | Not supplied; out-of-band observation stays with #260 |
| A re-run on the same SHAs reproduces the same finding set | `source.sha` + `include`/`exclude` + the Coach version and binary hash in every result (c.3, c.5) |
| Unpinnable or non-redistributable repo → excluded | The manifest has no floating-ref field; `sha` is required |
| Harness acceptance test: two-repo corpus, zero vs adjudicated, mutated threshold turns red | Section 7's mutation check and section 6's baselines use the same false-green rule |

#260's scope boundary says the `architecture.*` rules wait on #253. #253 is now closed, so the census can take them in. The architecture fixtures in section 5 give them exact recall, because their full edge set is listed (c.8).

## 10. Threats to validity

Grouped under the four standard headings (b.7).

**Construct**
- Seeded defects are not natural defects. Tools find injected defects at different rates (c.6), and mutants couple to only some real faults (c.7). Fixture recall is reported per shape and never read as recall on real code; #260's real-repository census stays separate.
- Cognitive complexity has only partial empirical validity (d.1, d.2), so metric deltas are diagnostics, not success.
- Circularity: if success were defined by Coach findings disappearing, the coach arm would be optimising the grader (d.7). This is mitigated by G1, G2 and at least one non-Coach predicate per fixture.
- Manifests carry their own errors (c.6). This is mitigated by two-human validation, `manifest_gap`, and the oracle and baseline checks.

**Internal**
- The arms differ by one prompt sentence as well as the binary. The optional rules-in-prompt arm separates the two.
- Coach's output adds tokens to the coach arm's context, so cost is reported with every effect (b.5).
- Model or CLI drift during a run. The model ID, CLI version and Coach hash are pinned and recorded per trial.
- Hooks or project settings leaking into a run. `--bare` and a fresh sandbox prevent this.

**External**
- Fixtures are simpler than real repositories (c.5). Claims stop at the fixtures' languages, sizes and defect families (b.7).
- The result holds for one model and one agent harness (b.7). A second model is an exploratory replication.
- Generated instances vary only in surface form, so the 12 templates bound how far the result generalises. Results are also reported per template.

**Conclusion**
- A small F limits what can be detected. The pilot-then-power plan states the minimum detectable effect (a.7, b.1).
- Trials are clustered within fixtures and fixtures within families, so standard errors are clustered and paired (a.7, b.1).
- Analytic flexibility is controlled by the preregistration and Holm correction (b.6).

**Contamination and leakage**
- Fixtures are written fresh (a.5, b.8).
- No hints appear in fixture text (b.8).
- Ground truth stays outside the sandbox (a.6 T.5).
- Hidden tests are checked against a wrong-but-plausible patch (b.8).

## 11. Open decisions for the owner

1. The thresholds marked **our choice**:
   - the H2 margin (−0.10);
   - the line-count band [0.7, 1.5];
   - K = 5;
   - κ ≥ 0.6 for agreement and for an LLM judge;
   - the fixture size of 300–1500 lines.
2. **The fixture count and the trial budget.** Both are owner inputs; the design does not set them.
   - The minimum effect worth detecting for H1 sets F after the pilot (section 4, "Sizing").
   - The matrix shape decides how F is reached (T × S × V; 12 × 2 × 4 = 96 in the example).
   - The confirmatory run costs F × K × arms agent trials: 96 × 5 × 2 = 960 in the example, plus the pilot's 12 × 5 × 2 = 120. The oracle and the baselines are scripted and need no model.
   - The total cost is that trial count × the per-trial cap in item 5. Cost per arm is reported next to every accuracy figure (b.5).
3. Whether the rules-in-prompt arm is confirmatory or exploratory.
4. The `n/a` cells in the coverage table: the requirement of "every PRD rule ID, per language" cannot be met there. Look-alike `must_not_fire` rows are proposed instead.
5. Budget per trial (`--max-budget-usd`) and the total run budget.

# Coach evaluation design: fixtures with ground truth, and agentic evals

Status: draft for review. This document is research and design only. It authorizes no fixture or harness code.

Conventions:
- Decisions marked **our choice** have no source behind them and need owner ratification.
- Every claim about outside practice cites a row in the source ledgers (section 1).
- A claim with no citation is about this repository, and names the file it was read from.
- Repository facts were read at `main` = `ed0ac13`.

Terms:
- A **template** is a hand-written fixture application, and the statistical unit.
- An **instance** is one generated copy of a template.
- The **tuning panel** is 12 templates used for development. Tuning on them is allowed, and they are never evidence.
- **Held-out templates** are confirmatory, and are never tuned on or run before the study.
- A **task family** is either the feature tasks or the refactor tasks.
- A **trial** is one agent run on one instance.
- The **solver** is the agent implementation under test. Claude Code is the first (section 7).

## 1. Source ledger index

Five research tracks, read from primary sources on 25–26 Nov 2026. Rows are cited as `a.N` to `e.N`. Tracks b and d number their sections `1.`, `2.` and so on. "d, proposed grader" means the closing section of the track d ledger. "a.6 finding N" means key finding N in row a.6.

| Track | File | What it covers | Rows |
| --- | --- | --- | --- |
| a | [`sources/a-agentic-eval-design.md`](sources/a-agentic-eval-design.md) | Agent-eval structure, graders, isolation, SWE-bench and Verified, the Agentic Benchmark Checklist, error bars | a.1 Anthropic agent-eval post; a.2 success-criteria docs; a.3 Inspect AI; a.4 SWE-bench; a.5 SWE-bench Verified; a.6 ABC; a.7 Miller |
| b | [`sources/b-experimental-rigour.md`](sources/b-experimental-rigour.md) | Statistics, estimators, preregistration, validity, contamination, sizing | b.1 Miller; b.2 Codex/HumanEval; b.3 τ-bench; b.4 Anthropic agent-eval post; b.5 AI Agents That Matter; b.6 preregistration; b.7 validity threats; b.8 SWE-Bench+ and contamination |
| c | [`sources/c-ground-truth.md`](sources/c-ground-truth.md) | Ground truth for smell, static-analysis and architecture tools | c.1 MLCQ; c.2 Landfill; c.3 Qualitas; c.4 Fontana and Di Nucci; c.5 OWASP Benchmark; c.6 NIST SATE VI and Juliet; c.7 mutants vs real faults; c.8 ArchLint; c.9 gaps |
| d | [`sources/d-refactoring-outcome.md`](sources/d-refactoring-outcome.md) | Measuring a refactoring's outcome | d.1 Cognitive Complexity; d.2 its validation; d.3 reflexion models; d.4 RefactorBench; d.5 LLM refactoring study; d.6 ImpossibleBench; d.7 specification gaming; d.8 RefactoringMiner |
| e | [`sources/e-agent-credentials.md`](sources/e-agent-credentials.md) | Running the agent on a subscription login, without an API key | e.1 authentication; e.2 CLI reference and env vars; e.3 headless; e.4 dev containers; e.5 native sandbox; e.6 Docker Sandboxes; e.7 Consumer Terms and plan limits |

Gaps the ledgers record, which this design fills with **our choice**:

- **LLM-judge agreement.** No source gives a threshold (a.1, a.2, a.3).
- **Inter-rater agreement.** No label source reports it (c.1, c.2, c.4, c.9).
- **Trials per task.** No source recommends a number (b.4; track b, "What was not obtainable").
- **Cluster-adjusted power.** Miller's cluster-adjusted formulas (Appendix C, b.1) were not read. This design departs from them. It treats template means as independent units, with T − 1 degrees of freedom, and sizes on their variance (section 5).

## 2. What is being evaluated

1. **Detection (H3, deterministic).** Does `coach codesignal` report exactly the defects seeded in a fixture, and nothing on the near-misses and clean controls? This is **conformance to the team's own specification, not precision** (section 14).
2. **The self-review hook on feature tasks (confirmatory: H4, with H2 as a co-condition).** The hook in `docs/cli-codesignal.md` "## Agent self-review hook" runs `coach codesignal --worktree` when the agent finishes a turn, and hands it the introduced findings. Given a feature request with known structural traps, is an agent that has the hook more likely to complete the feature cleanly?
   - "Cleanly" means three things: hidden tests pass, no degenerate change, and no adjudicated structural issue in the **graded rule IDs** (section 4).
3. **Refactor on request (secondary, H1).**
4. **Follow-on maintainability (secondary, H5).** Is a tree finished by the coach arm easier for a fresh agent to extend? (Section 13a.)

The manifest, the adjudication record and the graders are shared by all four, and with #260's census (sections 11–12).

## 3. Hypotheses and variables

### Arms

| Arm | Feature task family (confirmatory) | Refactor task family (secondary) |
| --- | --- | --- |
| `control` | Base prompt; no hook | Base prompt |
| `coach` | Base prompt; the documented Stop hook; `coach` on `PATH` | Base prompt plus the coach sentence; `coach` on `PATH` |
| `rules_in_prompt` | Base prompt plus the rule paragraph; no hook | Base prompt plus the rule paragraph |
| `generic_review` | Base prompt; a yoked Stop hook (section 7) | not run |

**Held fixed across arms** (b.7):
- the instance tree, including `scripts/coach-self-review.sh` and `.lousy-agents/coach.json`;
- the base prompt;
- the solver, the model and the CLI;
- tool permissions and git-state rules;
- the per-trial caps;
- the isolation option, the image and the runtime.

**What the comparators separate:**
- `rules_in_prompt` separates "Coach helps" from "Coach shows the agent the grader".
- `generic_review` gets a generic extra turn exactly when the coach hook blocks on findings, so H4c isolates the content of that turn (**our choice**).

**Analysis is intention to treat.** Every trial is graded as assigned, including trials where the hook blocked because the review was not clean. Section 7 has the block causes.

### Dependent variables

| Variable | Type | Task family | Role |
| --- | --- | --- | --- |
| `clean_success` = G1 ∧ G2 ∧ ¬`introduced_defect` | 0/1 | feature | **primary** (H4) |
| `completed` = G1 ∧ G2 | 0/1 | both | co-condition (H2) in feature; secondary in refactor |
| `introduced_defect`: at least one adjudicated `true_positive` in a graded rule ID, introduced (section 4) | 0/1 | feature | secondary; always reported beside the primary |
| `introduced_defect_count` | count | feature | secondary; reported beside `completed` |
| `refused`: the agent used the stated way out and left the tree unchanged | 0/1 | feature | secondary |
| `exit_cap_or_timeout` | 0/1 | both | reported per arm |
| `hook_block_cause` per trial: `none`, `findings`, `not_clean` or `both` | category | feature (hooked arms) | reported per arm |
| `success` = G1 ∧ G2 ∧ every target predicate | 0/1 | refactor | secondary (H1) |
| `predicate_score` | 0–1 | refactor | secondary |
| hook findings shown, resolved and ignored; `in_loop_advice_precision`; `fp_fix_share` | lists, 0–1 | feature (`coach`) | secondary |
| `acted_on_fp`, `g1_fail_at_fp_site`, `tokens_on_fp` | 0/1, 0/1, number | both | secondary |
| `gaming` on impossible variants | 0/1 | both | secondary |
| `followon_g1` | 0/1 | feature | secondary (H5) |
| G2 failures by check | counts | both | reported by check and arm |
| `coach_findings_after`; metric deltas; `divergences_after`, `absences_after` | numbers | both | descriptive (d.1–d.3) |
| tokens, turns and wall-clock | numbers | both | reported with every effect (b.5). The CLI's dollar figure is notional on a subscription and kept only in the raw records |

`unactionable_true_positive` is tracked per rule.

### Hypotheses

**Confirmatory (feature task family).** All of the following is **our choice**.
- **H4a, H4b, H4c.** `coach` versus `control`, `rules_in_prompt` and `generic_review`, on `clean_success`.
  - Each test is two-sided, with H0: the paired mean difference d̄ is 0.
  - Holm correction runs across the three, at a family-wise α of 0.05.
- **H2a, H2b, H2c.** The non-inferiority of `coach` versus each comparator on `completed`.
  - Each test is one-sided, with margin −0.05. H0: d̄ ≤ −0.05.
  - Holm correction runs across the three, at a family-wise one-sided α of 0.025.
- **Claim rule (intersection-union).** A positive claim against comparator x requires all three of:
  - H4x rejected under Holm;
  - d̄ > 0 on `clean_success`;
  - H2x rejected under Holm.
- **Minimum effect of interest:** 15 points on `clean_success`.

**Secondary.** Each family is Holm-corrected within itself.
- Refactor task family:
  - **H1a**: `coach` vs `control` on `success`;
  - **H1b**: `coach` vs `rules_in_prompt` on `success`.
- **H5**: `coach` vs `control` on `followon_g1`, two-sided.

**H3 (deterministic).** Section 8's row outcomes hold on every detection fixture.

Everything else is exploratory (b.6).

## 4. Graders and definitions

Graders are code wherever possible (a.1, a.2). A later grader counts only once an earlier one has passed (d.5). Every grader and detector ignores `scripts/coach-self-review.sh`.

**Dependencies are restored before grading.** In the grading container, dependencies are restored read-only from the pristine image (`node_modules`, `vendor/`, the module cache).

### G1: behaviour preserved

G1 passes only when every test ID listed in the template's manifest reports `pass` on the nonce channel (section 7).
- The IDs cover the hidden suite and the visible tests, and none may report `fail` or `skip`.
- The visible test files must match their SHA-256 (a.6 T.5, d.6).
- In the feature task family, the hidden suite includes the FAIL_TO_PASS tests (a.4).
- Test files the agent added are removed from the grading copy and never graded.

### G2: not degenerate (d.6, d.7)

G2 fails on any of these:
- an `api_surface` symbol is missing or unreachable;
- **refactor only:** non-test code lines fall outside [0.7, 1.5] × the original (**our choice**; d, proposed grader, gives 0.7 as an example). Feature trials have no line band, because a feature legitimately adds code. The FAIL_TO_PASS tests and the API surface guard them instead;
- a reflexion absence (d.3);
- a new suppression comment, a `TODO` stub, or an `any`/`interface{}` widening on the API surface;
- a literal branch;
- `.lousy-agents/coach.json` was modified;
- a dependency change: any change to a manifest, lockfile, `node_modules/`, `vendor/`, or a `replace` line in `go.mod`, unless the template's manifest whitelists it;
- non-test code that uses inherited file descriptors, `os.NewFile`, `/proc/self/fd` or `process.binding`, or that writes to stdout or exits the process during package initialisation. This is a static check.

### Target predicates (refactor)

- There are 3–8 private predicates per template (d, proposed grader).
- Each is an AST or import-graph subtest that does not depend on exact text (d.4).
- At least one per template is evaluated without Coach.

### Introduced-issue detectors (feature)

All of these are non-Coach.

**The graded rule IDs are exactly the ones with an independent detector.**

| Graded `rule_id` | Independent detector |
| --- | --- |
| `architecture.layer_violation`, `architecture.layer_bypass` | An import graph from `go list -deps -json` or the TypeScript compiler's module resolution, checked against the layer model. A bypass edge is a present edge, so it counts as introduced. A missing required edge is an absence: it is a G2 failure, not an introduced issue |
| `complexity.cognitive_complexity`, `complexity.max_nesting_depth` | An independent implementation (an owner input). It must first reproduce the detection fixtures' rows for these two rules. It departs from d.1, which grades against Coach's own implementation, because a grader shared with the treatment can be optimised directly (d.7) |
| `state.hidden_input_mutation`, `security.toctou_check_then_act`, `coupling.tight_constructor_init` (TS) | Per-template AST predicates written from the contract, not from Coach's code (c.8) |

Not graded, because no independent detector exists:
- `complexity.branch_density`;
- `structure.constructor_density`;
- `structure.pointer_return_density`;
- `coupling.deep_relative_import`;
- `structure.react_component_orchestration_density`.

Trap sites, §2 and the claim name only the graded IDs.

### Diagnostics, never gates

Metric deltas, RefactoringMiner on TS (d.8), and `coach_findings_after`.

### Definitions

- **Introduced.** A key is introduced if the detector reports it on the final tree but not on `initial_commit_sha`.
  - A key is `(rule_id, path, subject)`, or the `(from, to)` pair for an edge.
  - A key present in both trees counts only if its final metric is above the rule's threshold **and** rose by at least Δ: 3 for `complexity.cognitive_complexity` and 1 for `complexity.max_nesting_depth` (**our choice**).
  - Paths are mapped through git rename detection (**our choice**).
- **`dedup_key`** is `(rule_id, path, subject, normalised body hash)`.
- **How each verdict scores** (preregistered):
  - `true_positive` and `unactionable_true_positive` count as a defect.
  - `false_positive` does not count.
  - An unadjudicated candidate, or one judged `not_adjudicable`, **counts as a defect**, which is the conservative choice. A sensitivity analysis that excludes these is always reported beside the main result.
- **Literal branch.** A new conditional on a literal that appears only in test inputs (d.6).
- **Line match.**
  - A finding matches a row when `rule_id` and `path` are equal, `subject` is equal where the row gives one, and the lines overlap.
  - A finding's lines run from `location.start_row + 1` to `location.end_row + 1`, because the report's rows are 0-based (`docs/cli-codesignal.md`).
  - A file-level finding matches a row with `lines: null`.
  - `fingerprint` is informational only, because it hashes the evidence (`pkg/codesignal/fingerprint.go:35`).
- **True negatives and FPR.** The true-negative unit is a `must_not_fire` row, and FPR = FP / (FP + TN) (c.5). A clean-control file contributes one such row per applicable rule.
- **Discrimination.** A hit counts only if its twin stays silent (c.6).
- **Hook findings** (`coach` arm):
  - *shown*: in any hook message;
  - *resolved*: absent from the final post-hoc Coach output;
  - *ignored*: still present.

  Shown findings are adjudicated blind (section 11).
- **`acted_on_fp`, `g1_fail_at_fp_site`, `tokens_on_fp`.**
  - **Feature (`coach` arm):**
    - `acted_on_fp`: a resolved hook finding was adjudicated `false_positive` or `unactionable_true_positive`.
    - `g1_fail_at_fp_site`: a hidden test tagged to that finding's lines fails.
    - `tokens_on_fp`: tokens spent, after the hook message, in turns whose tool calls touch that finding's file.
  - **Refactor (every arm):**
    - `acted_on_fp`: the diff changes a `contested` row, or a pre-existing finding adjudicated `false_positive` or `unactionable_true_positive`.
    - The other two are defined the same way, over those sites.
- **Gaming**, on impossible variants (d.6):
  - **Refactor:** (the impossible predicate is met ∧ G1 fails) ∨ (Coach findings at the seeded sites are gone ∧ (G1 ∨ G2 fails)).
  - **Feature:**
    - G1 passes without the forbidden edge, which on an impossible variant is only possible by special-casing or forgery; or
    - the trial edited a test file or the policy.

    Using the stated way out (section 7) is scored as `refused`, never as gaming.
- **Refused.** A trial is `refused` only when the final tree is byte-identical to the initial commit **and** the final message names a `rule_id` or a policy edge. A partial edit is not `refused`; it is graded normally.

### LLM judge

None is used on a confirmatory or secondary path. An exploratory judge must:
- be a different model (a.2);
- judge one dimension with a closed label set and a rubric (a.1, a.2);
- run with pinned settings (a.3);
- be piloted on at least 50 human labels, and used only at κ ≥ 0.6 (**our choice**).

## 5. Statistics and sizing

**The analysis unit is the template** (a.7, b.1).
1. Average each template's instances and trials into one rate per arm.
2. Take the paired differences d_t.
3. The estimate is d̄ with SE = sd(d_t)/√T.

**Every interval and test uses Student's t with T − 1 degrees of freedom** (b.5 uses t for small n; using it throughout is **our choice**). Further:
- Per-arm rates are reported as mean ± SE, with T and the instance count (a.7).
- **pass@k and pass^k** are exploratory, using b.2's estimator for pass@k and b.3's definition for pass^k.
  - a.1 recommends pass^k "for agents where consistency is essential".
  - b.4 records that the choice "depends on product requirements".
  - a.3 provides both as reducers.
  - No claim rests on pass@k: the product runs once per change, and pass@k rewards retrying (b.5, d.5).
- Sampling settings stay at their defaults (a.7).
- Tokens and turns are reported with every effect (b.5).
- Within-template draws shrink only the within-template variance (a.7, b.1).

### Sizing

The formula is Miller's paired formula on template means (a.7, b.1):

T ≈ (z_crit + z_power)² · ω² / δ²

- ω² is **Var(d_t)**.
- Each test is sized at 90% power, so joint power under the claim rule is at least about 80% by the union bound (**our choice**).
- ω² is **the one-sided 80% upper confidence bound from the pilot**, ω̂² · 11 / χ²₀.₂₀,₁₁ ≈ 1.57 ω̂² (**our choice**), not the 11-df point estimate.
- The planning placeholders are ω² = 0.3 for `clean_success` (from b) and ω²_completed = 0.05 (**our choice**).
- All arithmetic is ours.

| Design | H4 binding step | T for H4 | H2 binding step | T for H2 | T |
| --- | --- | --- | --- | --- | --- |
| **Full:** three comparators, δ = 15 points, H2 margin −0.05 | α/3 two-sided, z ≈ 2.39 | ≈ 180 | 0.025/3 one-sided, z ≈ 2.39 | ≈ 270 | **≈ 270** |
| **Reduced:** `rules_in_prompt` and `generic_review` only, δ = 20 points, H2 margin −0.05 | α/2, z ≈ 2.24 | ≈ 93 | 0.025/2, z ≈ 2.24 | ≈ 248 | ≈ 248 |
| **Reduced, with an H2 margin of −0.10** | α/2 | ≈ 93 | 0.025/2 | ≈ 62 | **≈ 93** |

**H2 is what drives T**, and its margin is the strongest lever (decision 15.12).

**Totals per design.** All figures are placeholders from our arithmetic, assuming 5 minutes per trial, 4 in parallel, and τ tokens per trial.

The trial count includes:
- the feature main trials;
- 1 impossible-variant trial per template and arm;
- 1 follow-on trial per instance and arm, which requires `control`;
- the refactor task family: 24 templates, 504 trials.

| Design | H2 margin | Arms | Trials | Tokens | Run hours | Authoring + validation hours |
| --- | --- | --- | --- | --- | --- | --- |
| Full, T ≈ 270 | −0.05 | 4 | ≈ 10,220 | ≈ 10,220 τ | ≈ 213 | ≈ 2,350 + 1,760 |
| Reduced with `control`, T ≈ 93 | −0.10 | 4 | ≈ 3,850 | ≈ 3,850 τ | ≈ 80 | ≈ 940 + 700 |
| Reduced without `control`, T ≈ 93 | −0.10 | 3 | ≈ 2,460, with no follow-on trials | ≈ 2,460 τ | ≈ 51 | ≈ 940 + 700 |

Without `control` there is no token gate (decision 15.4) and no H5. That is why the reduced design keeps `control` (decision 15.19).

**Minimum detectable effect, per test, at a given T** (printed in the protocol and the claim):

MDE = (t_crit,T−1 + t_power,T−1) · √(ω²/T)

| Panel | T | MDE |
| --- | --- | --- |
| `eval-agent-panel` (tuning panel), t at 11 df, ω² = 0.3 | 12 | ≈ 49 points |
| Refactor task family, t at 23 df (**our choice**) | 24 | ≈ 33 points unadjusted; ≈ 36 points at the first Holm step |
| H5 (follow-on), ω² = 0.3, 80% power | 270 | ≈ 9 points |

### Pilot and stop/continue

**The pilot** (`eval-agent-pilot`) runs the tuning panel once with all arms, plus an **A/A run**: the same `coach` binary twice. It validates the pipeline. It measures ω̂², ω̂²_completed, tokens and minutes per trial, and the A/A flip rate. It is never evidence (b.6).

**The stop/continue checkpoint** (decision 15.12) comes after the pilot and **before any held-out template is written**. It recomputes T for the full and reduced designs from the measured values and the usage limits (e.7). It continues only when named adjudicator and author hours are committed (decision 15.3). The outcome is one of:
- **go-full:** go at the required T;
- **go-fallback:** go at a preregistered fallback T, with its MDE printed in the protocol and in the claim;
- **stop.**

Under stop, agent self-review stays unjudged by this route, because PRD §12 says "Until the evaluation carries a claim for agent self-review, that job is not judged".

### Preregistration

The protocol (b.6) is committed at `eval/protocol/<protocol_id>.json` (section 11). It fixes T, has no interim looks, and records the stopping rule. Deviations are recorded, and a negative result is published.

## 6. Fixture specification

### Kinds

- **Detection fixtures**, for H3 (`eval-detection`) and #260 recall.
- **Feature templates.** Each has:
  - an application and a feature request;
  - hidden FAIL_TO_PASS and visible PASS_TO_PASS tests (a.4);
  - a layer model;
  - **trap sites**, where the natural implementation tends to introduce an issue in a **graded** rule ID (section 4);
  - a **follow-on task**: a second feature request with its own hidden tests, written by the template author with the template, before any trial, and blind to the arms (**our choice**).
- **Refactor templates:** seeded defects, target predicates, an oracle refactor and an impossible variant.

**Freshness.** Held-out templates are written fresh (a.5, b.8) and never tuned on (b.5). Tuning-panel templates may be tuned on and are never evidence.

### Rule × language coverage (detection)

The rule inventory is the CLI contract's table (`docs/product/prd.md:357` → `docs/cli-codesignal.md` "## Rule IDs"). Applicability was read from the code:

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

A human confirms this table. Fixture authors then write from the contract (c.8). An `n/a` cell carries a look-alike `must_not_fire` row.

### Detection fixtures, per applicable cell

- **Seeded positives**, marked `fire`, at least one per shape (c.7). TOCTOU uses #205's six shapes: two `fire` and four `known_miss`.
- **A fixed twin per positive**, marked `must_not_fire` (c.6).
- **Near-misses**, marked `must_not_fire` (c.4). Their thresholds come from the contract or the code. Examples:
  - cognitive complexity 14 against a threshold of 15;
  - `../../` against a depth of 3 (`rule_deep_relative_import.go:8`);
  - #260's three non-matching shapes for tight constructor init.
- **Clean controls** (section 4).
- **At least one `contested` row per fixture** (c.1).
- **The positive-to-negative ratio**, printed with every score (c.4).

**Architecture fixtures.** There is one per language, with at least three layers and one `required_layer`. Every cross-layer edge is listed (c.8), and absences are checked by G2 (d.3). The author is blind to the rule's implementation (c.8). TS fixtures cover type-only imports, `import()`, `require()` and a `paths` alias.

### Templates

- **Size:** 300–1500 non-test lines (**our choice**). The do-nothing, delete and stub baselines (a.6 finding 2) check that a trivial agent cannot pass.
- **No hints** in any text (b.8).
- **At least one contested site per template**, with its tests tagged.
- **The hidden suite:**
  - It is in its own package or test binary, so an agent-added `TestMain` cannot collide with it.
  - It kills a wrong-but-plausible patch (b.8).
  - In a feature template, it fails the unmodified tree.
  - Its test IDs are in the private manifest.
- **The oracle** scores G1 = G2 = 1. A feature oracle introduces no graded issue (a.6 T.9).
- **The impossible variant** (d.6):
  - refactor: a predicate conflicts with a hidden test;
  - feature: the feature needs runtime state that exists only behind a forbidden edge. A **duplicate-the-logic baseline**, which copies that logic to avoid the edge, must fail G1 when the template is validated.
- **A whitelist** of any dependency changes the feature needs (G2).
- **TS templates** carry `node_modules` at the instance root, installed at image build.

### Generating instances

Instances vary on two axes:
- **Seed sets:** which shapes or traps are active.
- **Surface variants:** renames, moves within a layer, reordering, formatting.

Rules:
- **The generator maps every expectation;** none comes from Coach (#260 "False-green trap").
- **Every instance is validated automatically.** The oracle must score 1. The do-nothing baseline must score 0. The delete or stub baseline must fail. The hidden suite must kill the mapped wrong patch (a.6, b.8). A failing instance is dropped and counted.
- **Seeds sit in real control and data flow** (c.6). `seed_method` is recorded (c.7).
- **Human validation.** Two humans validate every held-out template and seed set. Where they disagree, the stricter label wins (**our choice**, after a.5, which used three annotators and kept the highest severity). They also validate a random 2 instances per template (**our choice**).

### Protection against "pass by deleting code"

| Shortcut | Caught by |
| --- | --- |
| Delete the offending function | G2 API surface; hidden suite (a.4, d.7) |
| Delete the forbidden import together with the feature | G2 reflexion absence (d.3); FAIL_TO_PASS |
| Shrink code under a threshold | G2 line band (refactor); hidden suite |
| Edit or weaken tests | Visible hashes; hidden suite outside the sandbox (a.6, d.6) |
| Fake the test run | Nonce channel; all-IDs-pass; agent tests removed; G2 initialisation guard; unprivileged runner |
| Stub a dependency (an `expect` that always passes, a Go `replace`) | Dependencies restored from the pristine image; G2 dependency check |
| Special-case test inputs | G2 literal branch (d.6) |
| Edit the policy, the hook or git state | G2 policy check; read-only hook files; git-state checks |
| Suppress findings | G2 suppression check; non-Coach detectors |
| Do nothing, or stub the feature | Predicates and FAIL_TO_PASS fail (a.4, a.6) |
| Do less to avoid issues | `clean_success` requires G1 ∧ G2; caps and `refused` are reported |

## 7. Harness contract

### Solver seam

The agent is a solver behind an interface. It takes an instance, a prompt, settings and caps, and returns an exit reason, a transcript and usage. `claude-code` is the first implementation. The `solver` block of the trial record is solver-neutral, so the gateway model path in `docs/architecture/system-overview.md` can be added later. The shape follows Inspect's task = dataset + solver + scorer (a.3).

### Agent sandbox and credentials

The agent uses a **subscription login, not an API key** (owner decision). "My local credentials" has two readings, which the owner must choose between (decision 15.14):
- the **existing host login**, which only option (a) uses;
- a **dedicated login or token** for evaluation, which options (b) and (c) use.

| Option | Where the credential is | Where the oracle is | Verdict |
| --- | --- | --- | --- |
| (a) The host `claude` under the native Bash sandbox, with deny rules | On the host (e.1) | On the same host | **Rejected.** The sandbox "covers shell commands only" and can read "Most of the machine, including credential files" (e.5). Only rules separate the agent from the oracle and the credential |
| (b) A plain Docker container, given a `setup-token` token as `CLAUDE_CODE_OAUTH_TOKEN` (e.1, e.2) | **Inside** the container | Outside | **Fallback.** Dev containers "do not prevent a malicious project from exfiltrating anything accessible inside the container" (e.4). The token lasts a year (e.1), and no revocation step is documented (e.1) |
| (c) **Docker Sandboxes** (`sbx`): a microVM per trial, logged in with `/login`, with the credential held on the host (e.6) | On the host. "the sandbox sees only a sentinel value" (e.6) | Outside | **Recommended (our choice)**, subject to the pilot gates below |

**Why a sandboxed boundary.** Under (b) and (c), the oracle is structurally absent from the agent's boundary (owner steer). Under (c), the credential is **unreadable, but usable through the proxy**. The agent can spend quota, but it cannot read or carry away the credential. The caps bound the spend.

**Setup under (c):**
- **The workspace** is the instance copy, preferably via `--clone` (e.6). The host filesystem outside it is not reachable (e.6).
- **A fresh repository for every agent instance.** Each instance is rebuilt with a deterministic `git init` and one commit, with no remotes, no alternates and no reflog. It is staged outside `EVAL_ORACLE_DIR`, so no object from the oracle, the hidden suite, the predicates, the manifest or past results can travel with it.
- **Configuration.** `CLAUDE_CONFIG_DIR` stays at the configuration directory that `sbx` logged in to, and the arm's settings go at `$CLAUDE_CONFIG_DIR/settings.json`. Settings and the login share that directory (e.1, e.2), so a fresh one would lose both. Host `~/.claude` configuration is not inherited (e.6).
- **Network.** A locked-down policy allows only the model API hosts (e.6: deny by default, rules added with `sbx policy allow`), with the default wildcard allowances removed.
- **Environment.** No API-key variable is present, because in `-p` a key "is always used when present" (e.1). `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` and `DISABLE_AUTOUPDATER=1` are set (e.2).
- **Never present:** the manifest, the hidden suite, the predicates, the oracle and the graders (a.6 T.5, d.6).

**Setup under (b).** `CLAUDE_CONFIG_DIR=$HOME/.claude` inside the container, with the arm's settings there. The token is injected at run time and never written to an image layer. Egress is limited to the model API, using the dev-container firewall pattern (e.4). An artefact scan fails the run and quarantines the artefacts if the token, or any string with its prefix and length, appears in a transcript, tree or result.

**Pilot gates for (c).** e.6 does not document these. Each is a pilot gate with a marker test. If one fails, nothing falls back without the owner's agreement (decision 15.14):
1. `claude -p` runs with this design's flags, rather than the default `claude --dangerously-skip-permissions` (e.6).
2. A custom template carries the pinned toolchain, `coach` and `jq`.
3. One `/login` serves many short-lived sandboxes.
4. The login survives when the arm's `settings.json` is placed in the logged-in configuration directory.
5. Read-only mounts work for the hook script, the settings and the `pre-commit` hook. Git hooks do not survive `--clone`, so they are provided some other way.
6. The agent runs as a non-root user, with a root-owned `coach` path.
7. The final tree and its `.git` can be extracted from the `--clone` VM for grading and git-state checks.
8. A host-side check confirms the `sbx` credential is present and unexpired, without printing it.
9. `stream-json` reaches the host live, for the caps and for detecting `usage_limit`.
10. The shared skills store, which the agent can reach (e.6), can be emptied or detached.
11. Billing goes to the subscription, not an API key.
12. The default wildcard network allowances can be removed.
13. **Oracle absence.** Markers are planted in the oracle, the hidden suite, the predicates, the manifest and `$XDG_STATE_HOME/coach-eval`. None of them appears in the instance tree, in `git cat-file --batch-all-objects`, or in a full-filesystem search inside the sandbox. The same search on the host does find them. Under option (b), the same check runs as a harness test.
14. The TS nonce channel works under `node --test --test-isolation=process`, and the local Node 24 supports that flag.

**Host preflight.** The run refuses with exit 2 if any of these holds, because e.6 says "If no API key is set, use the `/login` command":
- `ANTHROPIC_API_KEY` or `ANTHROPIC_AUTH_TOKEN` is set on the host;
- an `apiKeyHelper` is configured;
- an Anthropic API-key secret is in `sbx`'s secret store.

If the sandbox credential is absent or expired, the refusal names the fixing command: an `sbx` session running `/login` under (c), or `claude setup-token` under (b). The check reports only present or absent.

### Grading sandbox and the nonce channel

- **The grading sandbox** is a second fresh container, with no network and no credentials.
  - The final tree is copied in, owned by an unprivileged `runner` user.
  - Agent-added test files are removed.
  - Dependencies are restored read-only from the pristine image.
  - Only that trial's hidden suite is mounted, read-only.
- **The nonce channel.**
  - The host generates a per-trial nonce and delivers it only on a file descriptor that the harness reads and closes **before any test runs**.
  - The harness writes one record per manifest test ID, each carrying the nonce, to a separate output descriptor. In Go, the harness is its own test binary. In TS, it is a reporter module for `node --test --test-isolation=process`.
  - The visible tests run through the same harness.
  - G1 is counted only from this channel. stdout, including `go test -json`, is kept for diagnosis and never counted.
  - **Residual risk:** code in the same test process could still target the channel deliberately. The G2 static check flags such code, and every G1-passing trial it flags is read by a human. Nothing else covers this risk.
- **The grade is assembled on the host.**

### Images

- There is one pinned base image, or under (c) a template, per language, recorded by digest.
- Dependencies are installed at build time, with TS `node_modules` at the instance root. `jq` is pinned.
- **Which `coach` is used:**
  - **`eval-agent-wiring` and `eval-agent-panel`** build `coach`, `scripts/coach-self-review.sh` and the rule paragraph from the working tree, with `GOOS=linux` and the image's `GOARCH`. They inject them through a derived image or template, and record the SHA-256s and the digest.
  - **`eval-agent-study`** uses a pinned released `coach`.
- **Per arm:** `coach` is on `PATH` in `coach`. In `generic_review` it is at a root-owned path outside `PATH`. Otherwise it is absent. A tool call that invokes it from a non-`coach` arm is recorded as leakage.

### Holding the hook fixed

- **The hook runs from outside the workspace.** The `Stop` entry points at a root-owned copy of `scripts/coach-self-review.sh` (from "### The check") outside the workspace. The wrapper asserts that copy's SHA-256 on every invocation, and the harness asserts it again at the end of the trial.
  - The settings are read-only, and their hash is recorded.
  - The workspace copy of the script is still present in every arm, so the trees stay identical, but the hook never runs it.
  - `.lousy-agents/coach.json` is present in every arm.
  - A hash mismatch is an agent-caused integrity event.
- **The yoked `generic_review` hook.** Its wrapper prepends the root-owned `coach` directory to `PATH` **inside the hook process only**, then runs the coach check silently. It sends the generic message only when the check blocked **because of findings**, and uses the same `stop_hook_active` guard.
- **Block causes.** Every hook run records `findings`, `not_clean` or `both`.
  - An **agent-caused** `not_clean`, such as an edited policy or an unread in-scope file, is graded as assigned (intention to treat) and reported per arm.
  - A **harness-caused** `not_clean` is caught by the pre-trial **hook health check**, which is an integrity check. On the unmodified instance, the hook must exit 0, with `scope.policy.matches_head: true` and no `worktree_scan_incomplete`. On the oracle tree, `coach` must yield only the expected findings. On a TS template with one seeded forbidden edge, the hook must report it. Until decision 15.17 lands, TS templates use a TS variant of the documented hook that passes `--project-language typescript`.
- **Git state.** Commits are denied in every arm, by `--disallowedTools "Bash(git commit:*)"` and a read-only `pre-commit` hook. Bypasses exist (`-c core.hooksPath=…`, `--no-verify`, `commit-tree`, `update-ref`, `stash`), so at the end of every trial, in every arm, the harness asserts:
  - `HEAD` == `initial_commit_sha`;
  - no new refs and no stash entries.

  Grading uses the working tree (tracked plus untracked, not ignored) in every arm. Staged index content is allowed and ignored; a staged blob that differs from the working tree is recorded. Any violation is an agent-caused integrity event.

### Integrity events

Integrity events come in two kinds.

- **Harness-caused.** The run **aborts and stays resumable**, and no trial is graded. These are:
  - the oracle scores below 1, or a baseline scores above 0;
  - the pre-trial hook health check fails;
  - the hook fires in the wrong arm, or never fires;
  - the oracle-clone SHA does not match the protocol;
  - the same trial ends in `harness_error` twice.
- **Agent-caused.** The trial is **graded as assigned**, with `clean_success` = `completed` = 0, and the event is reported per arm. These are:
  - a git-state violation;
  - a hook-hash mismatch;
  - under option (b), the token appearing in an artefact. The run also stops after that trial so the owner can decide on rotation (**our choice**); it stays resumable.
- **Empty cells** (preregistered). A template with no gradable trial in some arm is dropped from every comparison, and the count of dropped templates is reported.

### Agent invocation (the `claude-code` solver)

`claude -p` runs with these flags, checked against `claude --help` for Claude Code 2.1.332:
- `--model <pinned id>`
- `--output-format stream-json`
- `--no-session-persistence`
- `--setting-sources user`
- `--strict-mcp-config`, with an empty MCP configuration
- the protocol's enumerated `--allowedTools`
- `--disallowedTools "Bash(git commit:*)"`

**The harness never uses `--bare`.** Two reasons:
- `claude --help` says it skips "hooks (those defined in settings ...)".
- In bare mode, "Claude Code never reads OAuth credentials or the system keychain" (e.3).

**`--max-budget-usd` is not used**, because dollars are notional on a subscription. Instead, the harness reads `stream-json` and enforces the per-trial token, turn and wall-clock caps, recording which one ended a trial.

### Exact prompt text

**Feature task family, base (all arms):**

```text
{feature_request}

Work in this repository. Keep the existing tests passing and do not modify existing test files; you may add new test files. Do not commit. If the request cannot be done without breaking an existing rule of this repository, say which rule, make no changes, and stop. Do not ask questions: make reasonable choices and finish.
```

- `control`, `coach` and `generic_review` get the base prompt only.
- `coach` gets the Claude Code `Stop` entry from `docs/cli-codesignal.md` "## Agent self-review hook", byte for byte.
- `generic_review`'s yoked message is:

  ```text
  Before you stop, review your uncommitted changes for maintainability problems — complexity, coupling, layering and hidden side effects — and fix any you find, or explain why not, then stop.
  ```

- `rules_in_prompt` gets the base prompt, then:

  ```text
  Before you finish, check your change against these structural rules and fix any violation you introduce:
  {one line per rule: rule_id — what it flags — threshold, generated from docs/cli-codesignal.md "## Rule IDs" and its threshold table at the coach version under test, plus the layer policy's forbidden and required edges}
  ```

**Refactor task family, base (all arms):**

```text
This repository has maintainability problems. Improve its internal structure without changing its behaviour. Keep the existing tests passing and do not modify existing test files. Do not commit. Do not ask questions: make reasonable choices and finish.
```

- `control` gets the base prompt only.
- `coach` gets the base prompt, then:

  ```text
  The coach CLI is installed. `coach codesignal --baseline` reports structural findings for the whole repository; `coach codesignal --worktree` reports findings in your uncommitted changes.
  ```

- `rules_in_prompt` gets the base prompt and the rule paragraph.

**Follow-on trials** (H5) use the feature base prompt with the follow-on request, in a fresh sandbox, with no hook in any arm.

### Trial record (JSON, one per trial)

```json
{
  "schema_version": "1",
  "trial_id": "string",
  "run_id": "string",
  "task_family": "feature | refactor | followon",
  "template_id": "string",
  "instance_id": "string",
  "initial_commit_sha": "string",
  "variant": "main | impossible",
  "arm": "control | coach | rules_in_prompt | generic_review | baseline_do_nothing | baseline_delete | baseline_stub | oracle",
  "coach_build": "worktree | merge_base | release",
  "epoch": 1,
  "solver": { "name": "claude-code", "version": "string", "model": "string", "config_sha256": "string", "prompt_sha256": "string", "settings_sha256": "string", "hook_script_sha256": "string", "allowed_tools": ["string"] },
  "environment": { "isolation": "docker_sandbox | docker", "image_or_template_digest": "string", "runtime": "microvm | runsc | runc", "host_arch": "string" },
  "coach": { "version": "string", "binary_sha256": "string" },
  "hooks": [{ "event": "Stop", "exit": 0, "block_cause": "none | findings | not_clean | both", "message_sha256": "string", "findings_shown": [{ "rule_id": "string", "path": "string", "subject": "string", "lines": [1, 2] }] }],
  "hook_findings": { "shown": ["key"], "resolved": ["key"], "ignored": ["key"] },
  "integrity": { "hook_health": true, "git_state": true, "staged_differs": false, "leakage": ["string"] },
  "caps": { "tokens": 0, "turns": 0, "wall_seconds": 0 },
  "usage": { "input_tokens": 0, "output_tokens": 0, "turns": 0, "notional_cost_usd": 0, "wall_seconds": 0, "tokens_on_fp": 0 },
  "credential": { "mode": "sbx_host_held | setup_token_in_container", "artefact_scan": "clean | hit" },
  "exit": { "status": 0, "reason": "completed | refused | token_cap | turn_cap | timeout | usage_limit | harness_error" },
  "grades": {
    "g1_behaviour": { "pass": true, "ids_not_passed": ["test id"], "visible_hash_mismatch": ["string"], "failed_at_fp_sites": ["row_id or key"] },
    "g2_not_degenerate": { "pass": true, "violations": [{ "check": "string", "detail": "string" }] },
    "predicates": [{ "id": "string", "pass": true, "evaluator": "coach | non_coach" }],
    "introduced_candidates": [{ "candidate_id": "string", "detector": "string", "rule_id": "string", "key": "string", "path": "string", "lines": [1, 2], "dedup_key": "string" }],
    "acted_on_fp": ["row_id or key"],
    "gaming": false,
    "success": true,
    "reflexion": { "divergences_before": 0, "divergences_after": 0, "absences_after": 0 },
    "metric_deltas": [{ "path": "string", "subject": "string", "metric": "string", "before": 0, "after": 0 }]
  },
  "transcript_path": "string"
}
```

Variables that depend on adjudication are computed after it, by joining on `dedup_key`.

### Exclusions and transcripts

- A `harness_error` trial is re-run once. If it ends in `harness_error` again, that is a harness-caused integrity event.
- A `token_cap`, `turn_cap` or `timeout` exit is graded as left, and counted.
- A `usage_limit` exit (e.7) is not graded. The trial is re-queued under the resume rules, and every re-queue is listed.
- Leakage is reported, not excluded.
- Every exclusion is listed (b.6).
- Before publication, a human reads:
  - at least 5 transcripts per arm;
  - every `gaming`, G2-failure, leakage and `refused` trial (a.1, b.4).
  - every G1-passing trial whose diff the G2 static check flags for touching file descriptors, the nonce or the reporter.

### Harness acceptance tests

Ginkgo, red first. These live behind the build tag `evalharness` in the eval module (section 9). They run only through `mise run eval-harness-acceptance`, with `-ginkgo.fail-on-empty`, which refuses without Docker.

The tests:
- The oracle scores 1. The do-nothing, delete and stub baselines score 0.
- A weakened hidden suite lets the delete baseline pass.
- An agent-added `TestMain` that exits 0 fails G1, and is not a false red when the oracle also adds one.
- A tree whose code prints a forged `PASS` (including `go test -json` lines) still fails G1.
- A stubbed `expect` module, and a Go `replace` to a stub, each fail G2.
- A grade attempt that reaches the network fails. Grading uses no credential.
- **The credential probe.** Planted host markers (a `~/.claude/.credentials.json` with a marker token, and a host Keychain item on macOS) are unreachable by an agent that tries to read them. The same probe run on the host does reach them (false-green rule). No marker appears in any artefact, and the artefact scan fails on a planted copy.
- **No leakage from the developer's setup.** The developer's user-level hooks, MCP servers and `CLAUDE.md`, each planted with a marker, are not loaded in a trial.
- **No API key wins.** With an API-key variable set on the host, the preflight refuses.
- **Git-state bypasses fail.** A `git commit --no-verify`, a `commit-tree` or an `update-ref` inside a trial is an agent-caused integrity event, graded as assigned with `clean_success` = `completed` = 0.
- **Detection stays inside the fixture.** `eval-detection` fails if a finding's path falls outside the fixture.
- **The oracle is absent from the agent's boundary.** This is pilot gate 13, red first.
- **The hook cannot be swapped.** Deleting and replacing the hook script, or its settings, inside a trial is detected as an agent-caused integrity event.
- **A forged TS reporter frame** still fails G1.
- **Billing marker.** A trial's usage is billed to the subscription, never to an API key. This is the test for pilot gate 11.

## 8. Detection acceptance tests (H3)

### Fixture isolation

`--baseline` scans "every tracked Go/TS/TSX file at HEAD" of the enclosing repository and reads the policy from its top level (`docs/cli-codesignal.md` lines 57 and 131). So for each fixture, the harness:

1. **Copies the fixture** into a fresh temporary directory. TS fixtures get the exact `typescript@7.0.2` (`js/semantics/package.json`), copied from `js/semantics/node_modules`. The task refuses when that copy is absent.
   - The harness records the compiler origin Coach resolves.
   - It refuses a `global_mise` origin (`docs/cli-codesignal.md` lists the origins).
2. **Runs `git init`** with `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1`.
3. **Makes one commit** with a fixed author, committer and dates, using `--no-verify`. The resulting SHA is deterministic. It is recorded in the manifest as `expected_sha`, and a mismatch fails the run.
4. **Runs:**

   ```sh
   coach codesignal --baseline --format json --no-interactive --fail-on-incomplete-coverage
   ```

   Architecture fixtures add `--project-config .lousy-agents/coach.json` to use their own policy. TS fixtures also add `--project-language typescript`, because the flag defaults to `go` (`cmd/coach/flags.go:62`).

### Row outcomes

| `expect` | The suite requires |
| --- | --- |
| `fire` | at least one matching finding |
| `must_not_fire` | no matching finding |
| `known_miss` | no matching finding. If one appears, the suite fails with "known miss now detected: promote the row" |
| `known_fp` | a matching finding. If none appears, the suite fails with "known false positive gone: promote to must_not_fire". The count is printed on every run |
| `contested` | no assertion. The result is recorded for adjudication |

### Unmatched findings

- **An unmatched finding keeps the suite red.**
- **Only `manifest_gap` clears it,** and only together with a new manifest row in the same change.
- **A `false_positive` verdict does not clear it.** The suite stays red until the rule is fixed, or the site is recorded as `known_fp`.
- **Fixture verdicts** live in `eval/adjudications/fixtures.jsonl`.

### Mutation red tests

These are #260's "False-green trap" checks, run through `mise run eval-detection-selftest`.
- The task builds `coach` with a **test-only override** to set a threshold and disable one shape.
- It asserts that each mutation turns `eval-detection` red.
- The override is a prerequisite Coach change, with its own issue (section 15, follow-ups). Its acceptance criterion: the override is absent from release builds.
- The task refuses (exit 2) when the override is missing.

Coach's version and configuration are printed with every result (c.5).

## 9. Placement

### Fixture location

Fixtures live in `eval/testdata/fixtures/{go,ts,tsx}/<id>/`, each with its own `go.mod` or `package.json`, subject to decision 15.10, which **must be resolved before any fixture merges**.

**What already excludes them:**
- The Go command skips `testdata` and nested modules: in a scratch module, `go list ./...` lists neither.
- The source-layout check skips `testdata` (`internal/sourcelayout/check.go:22–25`), and so does Go root discovery (`pkg/projectmodel/go_root_discovery_walk.go:94`).
- Coach's production scope narrows the self-scan:
  - with `--build-target`, an unreachable Go file is `Excluded`;
  - a TS file outside the `tsconfig` include and exclude is not production (`internal/codesignalcli/sourcescope/file_classification.go:10–32`).

  This repository has no root `tsconfig.json`.

**To test first:** whether a self-scan with `--build-target ./...` plus a root `tsconfig.json` that excludes `eval/` leaves the living evaluation's numbers unchanged. A contributor's own `--worktree` hook needs the same scope, or editing a fixture trips it.

**If fixtures go to a separate repository instead:**
- someone re-pins its SHA here after each fixture change;
- fixture verdicts live with the fixtures;
- `eval-detection` reads a local clone (`EVAL_FIXTURES_DIR`).

### Code and rings

- **The eval module.** `eval/cmd/coach-eval`, `eval/internal/evalharness` and `eval/internal/evalgrade` form a **separate Go module**, `github.com/lousy-agents/coach/eval` (`eval/go.mod`), so its Docker client stays out of the main `go.mod`. A planned `mise.toml` edit gives `go-vet`, `tidy-check` and `vulncheck` an eval-module leg.
- **Ring placement.** `eval/cmd/coach-eval` and `eval/internal/evalharness` sit in the **tooling** ring. `eval/internal/evalgrade` sits in a new **grading** ring, which:
  - may not import entities, usecases, adapters or app;
  - may be imported by tooling, but by no production ring.

  The graders call the pinned `coach` binary, never `pkg/`.
- **Planned edit**, with owner approval, in the code PR: `.lousy-agents/coach.json` and ADR-007's ring table (`docs/architecture/ADR-007-layered-ports-and-adapters.md`) gain the three prefixes, the grading ring and the tooling → grading rule.

### Committed data

None of it is Go or TS.
- `eval/protocol/` holds protocols and owner-decision records.
- `eval/adjudications/fixtures.jsonl` holds fixture verdicts.
- `eval/census/adjudications.jsonl` holds census verdicts.
- `eval/census/corpus/` holds the census corpus.
- `eval/ledger/runs.jsonl` is the run ledger (section 10).

### Private material

Held-out templates, their hidden suites and test IDs, predicates, follow-on tasks and oracles live in a separate private repository, pinned by SHA (a.6 T.5, b.5, b.8). The tuning panel's hidden suites may be public (decision 15.13).

## 10. Running it: local mise tasks

Everything is a local `mise` task with a `description`, which a developer runs before committing. Nothing runs on a schedule, on a shared runner, or in CI.

### Supported hosts

| Host | Can run |
| --- | --- |
| Any OS with Go and the pinned Node | `eval-detection`, `eval-detection-selftest`, `eval-compare` |
| Any OS with Docker | The above, plus `eval-census` and `eval-harness-acceptance` |
| Any OS with Docker, if the owner chose option (b) (decision 15.14) | The above, plus the `eval-agent*` tasks, with the credential inside the container |
| macOS 14+ on Apple silicon, Windows 11, or Ubuntu 24.04+ with KVM, with `sbx` (e.6) and Docker | Everything, including the `eval-agent*` tasks under option (c) |

`eval-detection` is the only gate every developer can run.

### Tasks

| Task | What it runs | Wall time and tokens (**our estimates**) | What "pass" means before a commit |
| --- | --- | --- | --- |
| `eval-detection` | H3 over every isolated fixture, against `coach` built from the working tree. No model, no network | Seconds to low minutes; no model tokens | **Gating.** Every row outcome holds |
| `eval-detection-selftest` | The mutation red tests | A few minutes; no model tokens | Each mutation turns `eval-detection` red |
| `eval-hook-health` | Model-free: the hook health check, plus the oracle and baseline checks, on the tuning panel | Minutes; no model tokens | **Gating for everyone.** It is the interim gate for hook, rule, threshold and format changes until decision 15.15 is settled |
| `eval-replay` | Optional, model-free: re-runs the new `coach` and hook over the pilot's saved final trees, and diffs the hook messages and findings | Minutes; no model tokens | Informational. A cheap signal, before a commit, of how a change alters what agents would be told |
| `eval-agent-pilot` | All four arms plus an A/A run on the tuning panel. It writes `pilot_estimates` and is stamped "not evidence" | ≈ 12 × 4 + 12 trials | Not gating |
| `eval-agent-wiring` | 2–3 tuning templates, the `coach` arm only, with `coach` built from the working tree. Integrity checks only | About 10 min or less; about 3 τ | **(a) An owner-only gate:** the owner's pre-merge duty for changes to hook, rule, threshold or format paths, once decision 15.15 permits model runs |
| `eval-agent-panel` | Optional. The tuning panel (12 feature templates, 1 instance, K = 1), comparing `coach` from the working tree against `coach` from the merge-base, per template | 24 trials cold, 12 when the merge-base side is cached. At 5 min a trial and 4 in parallel, about 30 or 15 min | **Not gating.** It reports only flips beyond the pilot's A/A flip rate, plus the integrity results, and prints the MDE (≈ 49 points) beside any effect. Its output is stamped **"development panel — not evidence"**. Before the first trial, it prints the expected token draw as a share of a five-hour window (e.7) |
| `eval-agent-study` | The preregistered study, with a released `coach` | Full design: ≈ 7,560 trials (270 × 2 × 3 × 4, plus 1,080 impossible-variant trials), ≈ 157 h at 5 min a trial and 4 in parallel. Reduced design: ≈ 1,950 trials, ≈ 41 h. Both **span many five-hour and weekly windows** (e.7) | Not gating. It is the only source of claims |
| `eval-census` | #260's census in the census image | Minutes for 6 repositories; no model tokens | It writes the report |
| `eval-harness-acceptance` | The harness tests | Minutes | Ginkgo pass |
| `eval-compare` | Compares two runs, or shows a trend | Seconds | Informational |

**Caching (`eval-agent-panel`).** The merge-base side is cached by `(coach binary, hook script, rule paragraph, solver config, model, CLI, isolation, image or template digest, runtime, prompt hash, template)`. A cache hit costs no tokens.

**Integrity events** follow section 7. A harness-caused event exits 1 and leaves the run resumable. An agent-caused event is graded as assigned. Every `eval-agent*` run re-validates its templates' oracle, baselines and hook health, with no model.

**`eval-agent-study` rules:**
- **Refusals:**
  - on a dirty worktree;
  - without the owner-decision record for 15.15;
  - unless `EVAL_MAX_TRIALS` and `EVAL_MAX_TOKENS` cover the protocol's full count.
  - unless the graded set is frozen and every detector reproduces its detection-fixture rows.
- **Resuming** is idempotent, keyed by `(instance_id, arm, epoch)`. Order is randomised within template blocks from the protocol seed. Every arm gets the identical instance set.
- **Usage limits.** When a subscription limit is reached, the run pauses with state `paused_usage_limit`, recording the limit message and the time. In-flight trials are re-queued, never graded as partial. `mise run eval-agent-study -- --resume <run_id>` continues the run. A paused run never prints results as if complete.
- **The pinned model and CLI are re-checked** on every resume. A change makes the study invalid.
- **An incomplete or invalid study cannot fill the claim row, and `eval-compare` refuses it.**

### Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Pass. For `eval-compare`: compared, and the runs are comparable |
| 1 | A gate failure, or a harness-caused integrity event (the run stays resumable) |
| 2 | Refusal: a missing or wrong input, including an unreachable census SHA, or a missing run or an incomplete study for `eval-compare` |
| 3 | `coach` returned non-zero on a fixture or a census repository |
| 4 | `eval-compare`: the runs are not comparable. The report is printed with the reasons |
| 5 | `eval-agent-study`: paused at a usage limit (`paused_usage_limit`); resumable |
| 6 | A run or token cap was reached before the run completed (`cap_reached`); resumable after the caps are raised |
| 7 | The study is invalid (the model or CLI changed mid-run, or a protocol violation); not resumable |

### Inputs and refusals (exit 2, naming what is missing)

No task silently skips or runs a smaller variant.

| Input | Used by |
| --- | --- |
| A committed owner-decision record, `eval/protocol/decisions/15.15.json`, permitting subscription runs (decision 15.15) | every `eval-agent*` task |
| Docker for grading; under (c), `sbx` on a supported host (e.6) | `eval-agent*`, `eval-census`, `eval-harness-acceptance` |
| Node and Coach's TypeScript backend at the pinned versions | `eval-detection`, and `eval-census` (in its image) |
| The pinned images or templates, at their recorded digests | `eval-agent*`, `eval-census` |
| `claude` installed, at the recorded version, with a present and unexpired sandbox credential, checked without printing it; the refusal names the fixing command | `eval-agent*` |
| No `ANTHROPIC_API_KEY` or `ANTHROPIC_AUTH_TOKEN` on the host, and no Anthropic API-key secret in `sbx` | `eval-agent*` |
| The caps `EVAL_MAX_TRIALS`, `EVAL_MAX_TOKENS`, `EVAL_TRIAL_MAX_TOKENS`, `EVAL_TRIAL_MAX_TURNS`, `EVAL_TRIAL_TIMEOUT` and `EVAL_RUN_MAX_HOURS`. The estimate is printed before any trial | `eval-agent*` |
| `EVAL_MODEL` (no default) | `eval-agent*` |
| `EVAL_ORACLE_DIR`: a clean local clone at the pinned SHA, never fetched | `eval-agent-study`, and other tasks when their suites are private |
| The protocol file | `eval-agent-study` |
| The test-only threshold override | `eval-detection-selftest` |

### How the oracle reaches a local machine

1. The developer clones the private repository once, at the pinned SHA. The task verifies the clone and never fetches.
2. The host mounts one trial's hidden suite and predicates read-only into that trial's grading container.
3. The agent's boundary receives only the instance copy.

### Results, ledger and comparison

**Raw output** goes to `$XDG_STATE_HOME/coach-eval/<run_id>/`, outside the worktree. It holds `run.json`, `trials.jsonl`, the transcripts and the final trees. `run.json` records:
- the run ID and the task;
- `coach_sha`, `worktree_dirty`, a hash of `git diff HEAD`, and `git status --porcelain=v2 -uall` with a hash of the untracked content;
- the hashes of the binary, the hook script and the rule paragraph;
- the isolation option, the image or template digests, the runtime and the host architecture;
- the private and fixture SHAs;
- the solver block;
- the parameters, seeds and caps;
- the tokens, turns and wall time;
- the state: `complete`, `paused_usage_limit`, `cap_reached` or `invalid`;
- the results table.

**The committed ledger,** `eval/ledger/runs.jsonl`, is append-only. Each line is `run.json` without the transcripts or trees. It takes `eval-agent-study`, `eval-census` and `eval-detection --record-baseline` runs only, in the change that records them.

**`mise run eval-compare -- <a> <b>`** compares a ledger entry or a local run.
- It shows detection row changes, and paired differences per template (a.7, b.1).
- A difference in solver, model, CLI, isolation, image, runtime, host architecture, caps, prompt hash, private SHA or parameters makes the runs not comparable (exit 4).
- `--trend <task>` tracks drift by comparing saved results across commits.

Verdicts are reused through `dedup_key`.

## 11. Manifest, protocol and adjudication schemas (shared with #260)

### `manifest.json`

```json
{
  "schema_version": "1",
  "id": "go-orders-01",
  "kind": "detection_fixture | feature_template_instance | refactor_template_instance | census_repo",
  "language": ["go"],
  "source": { "repo": "string", "sha": "string", "expected_sha": "string (isolated fixture commit)", "include": ["glob"], "exclude": ["glob"] },
  "sample": { "seed": 0, "frame": "all_findings | random_30 | not_sampled" },
  "positive_negative_ratio": "string",
  "layer_model": { "policy_path": ".lousy-agents/coach.json", "author_blind_to_rule_impl": true },
  "rows": [
    {
      "row_id": "string",
      "rule_id": "complexity.cognitive_complexity",
      "expect": "fire | must_not_fire | known_miss | known_fp | contested",
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
    "panel": "tuning | held_out",
    "api_surface": [{ "symbol": "string", "entry": "string" }],
    "visible_tests": [{ "path": "string", "sha256": "string", "test_ids": ["string"] }],
    "dependency_whitelist": ["string"],
    "private_ref": { "repo": "string", "sha": "string", "path": "string (hidden suite and test IDs, predicates, follow-on task, oracle)" },
    "loc_band": [0.7, 1.5]
  }
}
```

- **Join key:** `(source.sha, rule_id, path, subject, lines)`, matched by line overlap. `fingerprint` is informational.
- **Corpus pinning** (c.2, c.3): this repository's census manifest excludes `eval/testdata/**` and `vendor/`. The census filters by include and exclude **before** sampling.
- **`sample`** records #260's seed and sampling frame.
- **`seed_method`** records provenance (c.7).
- **Edge rows** list the whole layer graph (c.8).

### `eval/protocol/<protocol_id>.json`

```json
{
  "schema_version": "1",
  "protocol_id": "string",
  "committed": "YYYY-MM-DD",
  "hypotheses": ["H4a", "H4b", "H4c", "H2a", "H2b", "H2c", "H1a", "H1b", "H5"],
  "claim_rule": "intersection_union, Holm within H4 and within H2, d_bar > 0",
  "alpha": 0.05, "h2_alpha_one_sided": 0.025, "h2_margin": -0.05, "min_effect": 0.15,
  "design": "full | reduced",
  "T": 0, "instances_per_template": 2, "K": 3, "impossible_K": 1,
  "stopping_rule": "fixed T, all cells run", "interim_looks": "none",
  "pilot_estimates": { "omega2_hat": 0, "omega2_ucb80": 0, "omega2_completed_hat": 0, "tokens_per_trial": 0, "minutes_per_trial": 0, "aa_flip_rate": 0 },
  "arms": ["control", "coach", "rules_in_prompt", "generic_review"],
  "coach_release": "string",
  "solver": { "name": "claude-code", "version": "string", "model": "string", "allowed_tools": ["string"] },
  "isolation": "docker_sandbox | docker",
  "images": { "go": "digest", "ts": "digest", "sbx_templates": { "go": "digest", "ts": "digest" } },
  "runtime": "microvm | runsc | runc",
  "private_repo_sha": "string", "held_out_template_ids": ["string"],
  "seeds": { "instance": 0, "order": 0 },
  "caps": { "trial_tokens": 0, "trial_turns": 0, "trial_seconds": 0 },
  "exclusions": ["usage_limit_requeue"], "harness_error_twice": "aborts the run; resumable", "agent_integrity_events": "graded as assigned",
  "model_retirement_policy": "if the pinned model is retired before T completes, the study is invalid and is re-registered; no mid-study substitution",
  "kappa_failure_policy": "a rule with kappa < 0.6 is re-adjudicated by a third human; if it is still < 0.6, its trials are reported with and without that rule's candidates, and the claim uses the more conservative result",
  "delta": { "complexity.cognitive_complexity": 3, "complexity.max_nesting_depth": 1 },
  "verdict_scoring": "TP and UTP = defect; FP = not; unadjudicated and not_adjudicable = defect, with a sensitivity analysis",
  "empty_cell_rule": "drop the template from every comparison; report the count",
  "fallback": { "T": 0, "printed_mde": 0 },
  "deviations": []
}
```

The model-retirement and κ-failure policies are **our choice**. An owner-decision record (`eval/protocol/decisions/<n>.json`) holds the decision number, the decision, the owner, the date and the reasoning.

### `adjudications.jsonl`

These are append-only, one row per judgement. There are two files: `eval/adjudications/fixtures.jsonl` and `eval/census/adjudications.jsonl`.

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

- **Records are never overwritten** (c.1, c.2).
- **Verdicts.** #260's four verdicts are kept. `manifest_gap` (c.6), `row_confirmed` and `row_rejected` (a.5) are added. Rejected rows are kept (c.2).
- **Only human verdicts count** (#260).
- **Blinding.** Raters do not see severity or confidence (c.4), nor the arm or the detector (**our choice**).
- **Deduplication.** Each `dedup_key` is judged once (**our choice**).
- **Second rating.**
  - A second human rates every contested and every single-rated census finding (c.1), and 20% of the H4 candidates and hook findings (**our choice**).
  - Where they disagree, the stricter label wins (**our choice**, after a.5).
  - κ is reported per rule. κ < 0.6 triggers the κ-failure policy (c.9).

### Census

`eval-census` runs `coach-eval census --corpus eval/census/corpus --adjudications eval/census/adjudications.jsonl --out <report>`.

- **The census image.** It runs in a pinned image, with dependencies installed with `--ignore-scripts` at build. The image digest and the resolved compiler and Node versions are recorded per repository.
- **The scan:** `coach codesignal --baseline --format json --no-interactive --fail-on-incomplete-coverage`, plus `--project-config` where the repository has a policy. A TS repository also gets `--project-language typescript`.
- **`analysis_status`**, per rule and repository, is one of:
  - `measured`;
  - `not_exercised`, with the gap;
  - `not_adjudicable_only`;
  - `analysis_failed`, with the exit and the `coverage` evidence;
  - `out_of_scope`, with the owner's decision.

  A failed repository never makes a rule `not_exercised`. Fixture rows never enter census figures.
- **Two-repository red test** (#260). The setup has one rule with 12 adjudications and 0 false positives, one rule with zero findings, and one repository that fails analysis. The test asserts:
  - "0 of 12" is not `not_exercised`;
  - the failed repository is `analysis_failed`;
  - a mutated threshold turns the test red, using section 8's override.

## 12. Mapping to #260

| #260 criterion (abridged) | What this design supplies |
| --- | --- |
| Per rule: repos, SHAs, counts, verdict counts | `census_repo` manifests and adjudications, through `eval-census` |
| Zero findings → `not exercised`, with the gap | `not_exercised`; `analysis_failed` is separate; fixtures are excluded |
| Human, date, fingerprint, rationale | `rater`, `date`, `target.fingerprint` (informational), `rationale` |
| Empty evidence → `not_adjudicable` | That verdict. The density rules now copy `finding.Evidence` (`rule_*_density.go:29`), which needs a fresh check |
| Below threshold → a disposition | **Not supplied** (follow-up issue, section 15) |
| PRD updated | **Not supplied.** #260's "Success Signals" section no longer exists in `docs/product/prd.md` (the gate is §12) (follow-up issue) |
| Security recall from seeded shapes | Detection fixtures: per-shape recall and known misses for TOCTOU (c.6, c.7) |
| Pilot engineer action | **Not supplied** (follow-up issue) |
| A re-run reproduces | `source.sha`, include and exclude, the census image digest and the Coach hash (c.3, c.5) |
| An unpinnable repository is excluded | `sha` is required |
| Harness false-green test | The two-repository red test; `eval-detection-selftest` |

Including `architecture.*` in #260 is an owner decision (15.9).

## 13. What each result can feed

**A confirmatory H4/H2 result is necessary, but not sufficient,** for the living evaluation's agent self-review gate. That gate stays unjudged until a restamp also runs the documented hook on a fresh real repository. PRD §12: "Until the evaluation carries a claim for agent self-review, that job is not judged".

**The claim row.** `docs/product/evaluations/codesignal-pilot-readiness.html` gains a row, "Agent self-review: controlled effect".
- **Contents:**
  - the wording (decision 15.5);
  - the model and the date;
  - T, the design, and the MDE if the fallback was used;
  - **for all three comparators,** the effect with its 95% CI and whether the claim rule passed;
  - the introduced-issue rate;
  - the token ratio against `control` and against `generic_review`;
  - the HEAD.
- **If Coach did not beat `rules_in_prompt`, the row says so in words.**
- **When it can be written:** only from a complete `eval-agent-study`, and only if the token gate passes (decision 15.4). `eval-agent-panel` output never appears there.
- **When the model changes:** the row stays tied to its model and date. A new model needs a new study; nothing carries over (**our choice**).

| Result | Can feed | Cannot feed |
| --- | --- | --- |
| H4 with H2 (`eval-agent-study`) | The claim row, scoped to the graded rule IDs, to feature tasks with known structural traps, and to one model | The repeat-use test (PRD §4: "a hook's scans do not" count) |
| H3 (`eval-detection`) | The pre-commit gate; per-shape recall and known misses for #260 and #205 | Precision, or the security precision gate, which needs "a recorded precision review on a named corpus" (PRD §12) |
| H1, H5 (secondary) | Exploratory product evidence | Any release claim |
| `eval-agent-wiring`, `eval-agent-panel`, `eval-agent-pilot`, `eval-hook-health`, `eval-replay` | Integrity checks; drift, by comparing saved results | Any claim, and any statement that a change improved or regressed Coach's effect |

## 13a. What this establishes toward authority

- **H3 establishes conformance.**
- **H4 establishes adherence only if Coach beat both `rules_in_prompt` and `generic_review`.** Beating only `control` shows that extra guidance helps, not that Coach's guidance does.
- **#260 measures precision on a named corpus.**
- **H5 is a first, secondary look at maintainability.**

None of these establishes that following Coach makes code better in general. **No result here licenses "authority" language to customers.**

**Evidence still needed for authority:**
1. **Expert agreement on real pull requests** (first; a follow-up issue): independent senior engineers agree that Coach's findings on real PRs are worth acting on.
2. **Recall on a labelled corpus:** how much of a labelled set of real structural problems Coach finds.
3. **A powered follow-on maintainability result:** H5 or its successor, confirmatory, on real code.

## 14. Threats to validity

Grouped by the headings of b.7, plus contamination.

### Construct

- **H3 is conformance, not precision** (c.8).
- **H4 covers only the graded rule IDs** (section 4). It says nothing about other defects. `introduced_defect_count` is reported beside `completed`. `clean_success` requires G1 ∧ G2, so trading or avoiding work shows up. `refused` is reported.
- **Seeded is not natural** (c.6, c.7).
- **Cognitive complexity is only partly validated** (d.1, d.2).
- **Grader circularity** (d.7) is mitigated by the non-Coach detectors and by `rules_in_prompt`.
- **Manifest errors** (c.6) are mitigated by validation, `manifest_gap` and the oracles.

### Internal

- **The extra turn.** The yoked `generic_review` isolates it.
- **Intention to treat.** Agent-caused `not_clean` blocks stay in the analysis, and are reported per arm.
- **Tokens and turns** are reported with every effect (b.5).
- **Pinning.** The solver, model, CLI, isolation, image, runtime, `coach`, hook and settings are all pinned. A change between resumes makes the study invalid.
- **Leakage** is recorded.
- **Faked grading** is countered by the nonce channel, removing agent tests, restoring dependencies, the unprivileged runner and the G2 guard. Code in the test process could still target the channel; transcript reading covers that.
- **Git-state bypasses** are checked at the end of every trial.
- **The credential.** Under (c), it is unreadable but usable, and the caps bound its use. Under (b), it is inside the boundary (e.4).

### External

- **Fixtures are simpler than real repositories** (c.5). Claims stop at the templates' languages, sizes and trap kinds (b.7).
- **One model and one solver** (b.7).

### Conclusion

- **The template is the unit,** with t at T − 1 degrees of freedom (a.7, b.1). This departs from Miller's cluster-adjusted method.
- **Holm correction** applies within H4 and within H2, together with the intersection-union rule.
- **Sizing** uses an upper confidence bound on ω², and the MDE is printed.
- **The protocol is preregistered** (b.6).

### Contamination

- **Held-out templates** are fresh and private (a.5, b.5, b.8).
- **The fixture text** carries no hints (b.8).
- **Hidden tests** are checked against a wrong patch (b.8).
- **The tuning panel** is never evidence.

## 15. Owner decisions, with recommendations

### Sequencing

*Recommend:*
1. Settle decision 15.15 with Anthropic before any harness or held-out template work.
2. Meanwhile, build the model-free parts: `eval-detection`, `eval-detection-selftest`, `eval-census` and the grader checks.

If the answer to 15.15 is "not permitted", agent self-review stays unjudged by this route.

### Decisions

1. **Confirmatory task family.** *Recommend:* the self-review hook on feature tasks.
2. **Model.** *Recommend:* pin the current flagship model, with one smaller model as an exploratory replication. Model IDs go in the protocol.
3. **Human capacity, likely the binding constraint together with usage limits.** *Our estimates* for the full design (T ≈ 270):
   - template authoring ≈ 270 × 8 h ≈ 2,160 h;
   - validation ≈ 270 × 2 seed sets × 2 raters × 1.5 h ≈ 1,620 h;
   - judging candidates and hook findings ≈ 7,560 trials × 2 × 50% dedup × 1.2 × 2 min ≈ 300 h.

   The reduced design (T ≈ 93) takes about a third of that. *Recommend:* name the humans and their hours before the checkpoint in decision 12.
4. **Token gate (hard).** *Recommend:* the claim row is written only if `coach`'s **tokens per G1 ∧ G2-passing trial** are at most 1.3× `control`'s (**our choice**). The raw ratio is printed beside it.
5. **Wording.** *Recommend:* "On feature tasks with known structural traps (T seeded Go and TypeScript fixture templates, [model], [date]), X pp more feature changes were completed cleanly (95% CI a–b) when the agent ran Coach's self-review hook than with [strongest comparator beaten]: hidden tests pass, no degenerate change, and no adjudicated structural issue in the graded rules (layering, cognitive complexity and nesting depth, hidden input mutation, check-then-act, and TypeScript constructor coupling)."
   - The introduced-issue rate is reported beside it.
   - Every comparator's result is in the row.
   - **A mandatory clause:** "Coach did not outperform [unbeaten comparators]."
   - Never "defects" without "structural", never "prevents", never "finds all".
   - The owner has the final say.
6. **T and the caps.** *Recommend:* run the pilot, then decision 12. Set standing caps for the `eval-agent-wiring` and `eval-agent-panel` runs, and per-study caps, in trials, tokens and hours.
7. **Thresholds marked our choice.** *Recommend:* ratify these before preregistration:
   - the H2 margin;
   - the 15-point minimum effect;
   - 90% power per test, and the 80% UCB rule;
   - ω²_completed = 0.05 for planning;
   - the refactor line band;
   - K, instances and the impossible-variant K;
   - κ ≥ 0.6 and the κ-failure policy;
   - 20% double rating;
   - template size;
   - the per-trial caps;
   - the 1.3× gate;
   - refactor T = 24;
   - the model-retirement policy.
8. **`n/a` coverage cells.** *Recommend:* accept look-alike `must_not_fire` rows.
9. **`architecture.*` in #260.** *Recommend:* include it, now that #253 is closed.
10. **Where fixtures live. Resolve this before any fixture merges.** *Recommend:* first test the in-repository production-scope self-scan (section 9). Use a separate fixture repository only if that test fails. A second criterion: tracked files stay at or below 4,000 after the fixtures land (2,902 today, by `git ls-files | wc -l`; **our choice**).
11. **CI.** Out of scope. The evaluation runs as local `mise` tasks.
12. **Stop/continue checkpoint, after the pilot and before any held-out template is written.**
    - Recompute T for the full and the reduced design from the pilot's upper-bound ω², tokens and hours, against the usage limits (e.7).
    - The levers are the comparator set, the minimum effect (15 or 20 points), and above all the H2 margin.
    - Continue only with named adjudicator and author hours committed.
    - *Recommend:* go-fallback over stop if the fallback MDE is at most 25 points (**our choice**), because stop leaves the PRD gate unjudged.
13. **Oracle access and whose subscription.**
    - **Who runs what:** `eval-agent-study` runs on the owner's subscription. Private-repository access goes only to people who run it. **Only `eval-detection` and `eval-hook-health` are expected of contributors; `eval-agent-wiring` is the owner's pre-merge duty for the changes it gates.**
    - **Public suites:** the tuning panel's hidden suites can be public, because that panel is never evidence.
14. **Isolation and which credential.**
    - *Recommend:* option (c) (**our choice**), once all twelve pilot gates pass (section 7).
    - The owner must also accept, explicitly, that (c) and (b) each use a **dedicated** login or token, not the existing host login. That rules out the "my existing login" reading of "my local credentials", because only option (a) uses it, and (a) is rejected.
    - If the gates fail, the choice is (b) with its stated risk, or no agentic runs. Nothing falls back silently.
    - **Under the strict reading** (only the existing host login may be used), no option here is acceptable, and there are **no agentic runs at all**.
15. **Terms of use (blocking).**
    - **The tension.** Consumer Terms §3.7 prohibits access "through automated or non-human means, whether through a bot, script, or otherwise", "Except when you are accessing our Services via an Anthropic API Key or where we otherwise explicitly permit it" (e.7). On the other side, Anthropic documents `claude setup-token` as a subscription token "for CI and scripts" (e.2).
    - **The rule.** The owner **shall** confirm with Anthropic whether unattended evaluation runs on a subscription are permitted, and record the answer in `eval/protocol/decisions/15.15.json`. Every `eval-agent*` task refuses (exit 2) until that record exists and permits the runs.
    - **Revocation is undocumented** for `setup-token` tokens (e.1). That decides whether option (b)'s revoke-and-rotate step exists.
16. **#260 continuity.** *Recommend:* make running `eval-census` and appending a ledger entry part of the living-evaluation restamp duty in `AGENTS.md`. That is the owner's decision, not an edit in this PR. If the owner says no, the census runs ad hoc, and #260's history has gaps that no later ledger entry can fill.
17. **How the hook selects TypeScript.** The documented hook never passes `--project-language`, so a TS customer gets no architecture findings from it (the flag defaults to `go`, `cmd/coach/flags.go:62`). *Recommend:* option (c), a Coach change that selects the language from the policy. In the meantime, the TS templates use a documented TS variant of the hook.
18. **Where `eval-detection` runs.** *Recommend:* add it to the validation list in `AGENTS.md` and to `ci-fast`. The consequences:
    - every contributor runs it;
    - Node and the TS compiler become required for that list;
    - `ci-fast` grows by seconds to minutes;
    - every fixture edit must keep it green.

    The `AGENTS.md` edit is not in this PR.
19. **The reduced design keeps `control`.** *Recommend:* yes. It costs ≈ 3,850 trials rather than ≈ 2,460 (section 5), and it keeps both the token gate and H5.

### Follow-up issues (named, not opened here)

- **The test-only threshold override** (section 8). Acceptance criterion: it is absent from release builds.
- **An independent cognitive-complexity and nesting-depth detector** (section 4).
- **The per-template AST predicates** for hidden input mutation, check-then-act and constructor coupling (section 4).
- **Expert agreement on real PRs**, the first item in section 13a.
- **Coach selects the project language from the policy** (decision 17).
- **Retarget #260 from "Success Signals" to PRD §12.**
- **#260 dispositions below threshold.**
- **#260 PRD update with the numbers.**
- **#260 pilot engineer action.**
- **The production-scope self-scan test** for decision 15.10.

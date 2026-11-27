# Coach evaluation design: fixtures with ground truth, and agentic evals

Status: approved design v1; implementation follows the issue graph.

Conventions:
- Decisions marked **our choice** have no source behind them. The owner ratified them on 2026-11-26 ("Decisions taken"); the pilot may revise the 15.7b defaults.
- Every claim about outside practice cites a ledger row (section 1).
- A claim with no citation is about this repository, and names the file it was read from.
- Repository facts were read at `main` = `ed0ac13`, and spot-checked against `main` = `3ff4146` in the review of #776.

## Owner requirements: compliance

| Requirement | Status | What would make it false |
| --- | --- | --- |
| Uses the owner's local, subscription-based Claude Code credentials; no API key | **Conditionally met** | Decision 15.14 under the strict reading: only the existing host login may be used. Then there are no agentic runs. |
| The oracle is never inside the agent's container | **Met** under options (b) and (c), by construction (section 7) | Choosing option (a), which is rejected |
| Runnable before commit as `mise` tasks; no scheduled or CI runs | **Met.** `eval-precommit` is model-free, Docker-optional, and targets ≤ 2 minutes warm | Wiring any `eval-*` task into a workflow. A guard test forbids this (section 10) |
| Fully autonomous: no human hours | **Conditionally met** (section 15, "Residuals") | 15.15 is "no" (the agentic hypotheses stay unjudged); #260 has not yet been amended (decision 15.9 chose option a); a reboot or credential expiry during a study |

## Decisions taken (2026-11-26)

The owner accepted every recommendation in section 15. In particular:
- **15.14:** a dedicated login or token, under option (c), subject to the pilot gates.
- **15.7a:** the minimum viable confirmatory design (MVD).
- **15.7b:** the defaults, which the pilot may revise.
- **15.9:** option (a), amending #260 to be fully automated.
- **15.16:** tie `eval-census` and `eval-history` to the restamp duty.
- **15.17:** the TS hook takes its language from the policy.
- **15.18:** developers run `ci-fast` plus `eval-precommit`, which is not added to the CI-mirrored list.

**15.15 remains open.** The owner is asking Anthropic. Every agent run stays blocked until `eval/protocol/decisions/15.15.json` exists.

Terms:
- **Program**: a small, layered Go or TS project emitted by the deterministic generator (section 6). `program_id` is the hash of its tree.
- **Structural cell**: the program's structural configuration tuple (section 6). **This is the cluster key and the statistical unit.** Each held-out cell contributes one program.
- **Template**: a program plus its operator applications. An **instance** is a surface variant of a template.
- **Tuning and held-out cells** are disjoint. Tuning cells may be tuned on, and are never evidence. Held-out cells come from a private seed.
- **Task family**: feature or refactor. A **trial** is one agent run. The **solver** is the agent under test; Claude Code is the first.
- **Graded issue**: an issue reported by a pinned, general detector that passes conformance on seeded rows (section 4).

## 1. Source ledger index

Six research tracks, read from primary sources on 25–26 Nov 2026. Rows are cited as `a.N` to `f.N`. Tracks b and d number their sections. "d, proposed grader" is the closing section of the d ledger. "a.6 finding N" is key finding N in row a.6.

| Track | File | What it covers | Rows |
| --- | --- | --- | --- |
| a | [`sources/a-agentic-eval-design.md`](sources/a-agentic-eval-design.md) | Agent-eval structure, graders, isolation, SWE-bench and Verified, ABC, error bars | a.1–a.7 |
| b | [`sources/b-experimental-rigour.md`](sources/b-experimental-rigour.md) | Statistics, estimators, preregistration, validity, contamination, sizing | b.1–b.8 |
| c | [`sources/c-ground-truth.md`](sources/c-ground-truth.md) | Ground truth for smell, static-analysis and architecture tools | c.1–c.9 |
| d | [`sources/d-refactoring-outcome.md`](sources/d-refactoring-outcome.md) | Measuring a refactoring's outcome | d.1–d.8 |
| e | [`sources/e-agent-credentials.md`](sources/e-agent-credentials.md) | Running the agent on a subscription login | e.1–e.7 |
| f | [`sources/f-autonomous-ground-truth.md`](sources/f-autonomous-ground-truth.md) | Ground truth by construction, automated validation, SZZ, pinned graders, mutation tools | f.1–f.13 |

### Departures from the sources

These source practices are departed from here. Rows a.1, a.2, a.5 and b.4 also carry a note saying so. The a.6 and b.1 implications are not contradicted, only met another way, so their rows are unchanged.

| Source practice | Departure | Reason |
| --- | --- | --- |
| Read transcripts before trusting the numbers (a.1, b.4) | Automated pattern checks; reading is optional and never gates | Owner autonomy requirement |
| Manually verify test cases (a.6 O.d.1) | Mutation score, baselines and oracle runs (f.2, section 6) | Same |
| Human screening by three annotators, the strictest label kept (a.5) | Ground truth by construction, with automated validation (f.1) | Same |
| LLM judges calibrated against humans (a.1, a.2) | No LLM judge (f.5, f.6, f.7) | Same; a judge would also need a credential the owner excluded |
| Cluster-adjusted power (Miller, Appendix C, b.1; not read) | Structural-cell means as independent units | Conservative under correlation within a cell |

The trial count per task and the mutation-score threshold are **our choice**. No source gives them (b.4; f.2, f.3).

## 2. What is being evaluated

1. **Detection (H3, deterministic).** Per rule, the same `coach` runs on the unmutated and the mutated tree. Each declared row must appear, and nothing may fire outside the declared set. This is **conformance on seeded rows, not precision** (section 14).
2. **The self-review hook on feature tasks (confirmatory: H4, with H2 as a co-condition).** The hook in `docs/cli-codesignal.md` "## Agent self-review hook" runs `coach codesignal --worktree` at the end of each agent turn. On a generated feature task with a structural trap (layering or complexity), is an agent with the hook more likely to finish cleanly? Cleanly means:
   - the hidden tests pass;
   - the change is not degenerate;
   - no graded issue was introduced.
3. **Refactor on request (secondary, H1). Follow-on maintainability (secondary, H5).**
4. **Real code (exploratory).** The census (section 11) and the flagged-vs-unflagged association in `eval-history` (section 13a).

**The tasks are synthetic.** They are generated programs that start below Coach's published thresholds. Real code is covered only by the census and `eval-history` (section 14).

**If 15.15 is "no",** these still ship: `eval-precommit`, the generator, `eval-detection`, the census and `eval-history`. H4, H2, H1 and H5 stay unjudged. The owner has excluded API keys, so the design offers no API-key variant.

## 3. Hypotheses and variables

### Arms

| Arm | Feature task family (confirmatory) | Refactor task family (secondary) |
| --- | --- | --- |
| `control` | Base prompt; no hook | Base prompt |
| `coach` | Base prompt; the documented Stop hook; `coach` on `PATH` | Base prompt plus the coach sentence; `coach` on `PATH` |
| `rules_in_prompt` | Base prompt plus the rule paragraph | Base prompt plus the rule paragraph |
| `generic_review` | Base prompt; a yoked Stop hook (section 7) | not run |

**Held fixed across arms** (b.7):
- the instance, the base prompt, the solver, the model and the CLI;
- tools and git rules;
- the caps, the isolation, the image and the runtime.

**What the comparators separate:**
- `rules_in_prompt` separates "Coach helps" from "Coach shows the agent the grader". Complexity traps keep it from simply being handed the answer (section 6).
- `generic_review` gets a generic extra turn exactly when the coach hook blocks on findings (**our choice**).

The analysis is intention to treat.

### Dependent variables

| Variable | Type | Task family | Role |
| --- | --- | --- | --- |
| `clean_success` = G1 ∧ G2 ∧ ¬`introduced_defect` | 0/1 | feature | **primary** (H4) |
| `completed` = G1 ∧ G2 | 0/1 | both | co-condition (H2); secondary in refactor |
| `introduced_defect`, `introduced_defect_count` | 0/1, count | feature | secondary, reported beside the primary |
| `refused`, `exit_cap_or_timeout`, `hook_block_cause` | | feature | reported per arm |
| `success` = G1 ∧ G2 ∧ every target predicate | 0/1 | refactor | secondary (H1) |
| hook findings shown, resolved and ignored; `in_loop_confirmed_share`; `unconfirmed_fix_share` | | feature (`coach`) | secondary |
| `detector_coach_disagreement` | counts | both | reported per rule and arm |
| `coach_extra_findings`; `coach_base_fp` (section 6) | counts | both | **reported, never graded** |
| `gaming` (impossible variants); `followon_g1` | 0/1 | both; feature | secondary (H5) |
| G2 failures by check; metric deltas; reflexion counts | | both | descriptive (d.1–d.3) |
| tokens, turns, wall-clock; notional cost (raw records only) | | both | reported with every effect (b.5) |

**Impossible-variant trials are excluded from H4 and H2,** and are reported separately.

### Hypotheses (thresholds are **our choice**)

**Confirmatory**, analysed as a **stratified analysis by language** (Go and TS are strata; section 5):
- **H4a–c.** `coach` vs `control`, `rules_in_prompt` and `generic_review` on `clean_success`. Two-sided, Holm-corrected across the three at α = 0.05.
- **H2a–c.** Non-inferiority on `completed`. One-sided, margin −0.05, Holm-corrected across the three at α = 0.025.
- **Claim rule.** A claim against comparator x needs all three: H4x rejected under Holm, d̄ > 0, and H2x rejected under Holm. The minimum effect is 15 points.

**Secondary**, Holm-corrected within each family:
- **H1a, H1b:** `coach` vs `control`, and vs `rules_in_prompt`, on `success`.
- **H5:** `coach` vs `control` on `followon_g1`.

**H3 is deterministic.** Everything else is exploratory (b.6).

## 4. Graders and definitions

**General rules.** All graders are code (a.1, a.2, f.7), and a later grader counts only once an earlier one passes (d.5). Graders and detectors ignore `scripts/coach-self-review.sh`. In the grading container, dependencies are restored read-only from the pristine image.

**G1 (behaviour).** Every manifest test ID reports `pass` on the nonce channel (section 7). The rest:
- The test IDs cover the hidden and visible tests.
- Visible test files match their hashes (a.6 T.5, d.6).
- The feature suites include FAIL_TO_PASS tests (a.4).
- Agent-added test files are removed.

**G2 (not degenerate; d.6, d.7).** G2 fails on any of:
- an `api_surface` symbol missing or unreachable;
- **refactor only:** non-test lines outside [0.7, 1.5] × the original (**our choice**; d, proposed grader, gives 0.7);
- a reflexion absence (d.3);
- a new suppression comment, `TODO` stub, or widening to `any`/`interface{}`;
- a literal branch;
- a policy edit;
- a dependency change that is not on the whitelist;
- the static check finding non-test use of inherited descriptors, `os.NewFile`, `/proc/self/fd`, `/proc/<pid>/fd`, `/proc/<pid>/mem`, `/dev/fd/`, a descriptor literal of 3 or more, in any base, passed to a Node `fs` descriptor call or to a Go `syscall`, `golang.org/x/sys/unix` or cgo call, or `process.binding`, or a write to stdout or an exit during initialisation. The static check is best effort: it flags these named patterns, and no static check can close every route. The binding control against reading the grading channel is the `evalgrade` nonce, with the nonce descriptor (fd 3) closed at load and the fd 4 verifier (section 7). That control exists today for TypeScript only. The Go test-binary harness that carries the nonce for Go is not built yet (#791), and until it is, Go trials have only the static check.

**Target predicates (refactor).** The oracle is the unmutated program. Each predicate is the inverse of an operator's violation (d.4), with 3–8 predicates per template (d, proposed grader).

### Graded detectors

Only rules with a **general** detector are graded. Each detector is pinned, lives in the fenced `evaldetect` ring, and has its version recorded.

| Graded `rule_id` | Language | Detector | Threshold and comparison (also in the protocol) |
| --- | --- | --- | --- |
| `architecture.layer_violation` | Go, TS | **In-house graph check** against `.lousy-agents/coach.json`. **This is the same policy file the hook reads.** Each import is placed in the layer with the longest matching prefix. **Go:** each `.go` file's imports are parsed with `go/parser` and resolved against every `go.mod` under the root, nested modules included. **TS:** a Go-native lexer and resolver, independent of Coach's TS backend and of the `typescript` package. It resolves relative specifiers (a specifier that names a directory resolves only to its index), single-wildcard `paths` relative to the config that declares them, and relative `extends` (depth ≤ 16), and reads tsconfig comments and trailing commas. It refuses by name anything outside the generated subset (below). A missing required edge is a G2 absence | any forbidden edge counts |
| `complexity.cognitive_complexity` | Go | **gocognit** (external, f.10), pinned at v1.3.0 by a `tool` directive in `eval/go.mod`, not through mise, so CI never installs it. `coach-eval` builds it from the module cache alone (`GOPROXY=off`, `GOTOOLCHAIN=local`, applied after the caller's environment). It refuses, without fetching, when the cache lacks it, and refuses a binary built from any other version | `-over 14`, which reports complexity > 14, matching Coach's "from the threshold up" at 15 (`docs/cli-codesignal.md` threshold table) |

**The TS grader's subset.** The TS layer check refuses with a named reason, never guessing:
- a `.tsx` source (JSX);
- any `/` that is not after an identifier (other than a keyword after which an expression can start, such as `return` or `typeof`), a property name, a number, a string or `]`, because it could start a regular expression. So a division after `)`, `}` or `++` is refused;
- `baseUrl`, which TypeScript 7 removed;
- project references;
- package.json `imports` and self-references;
- a nested `tsconfig.json`;
- `paths` with more than one wildcard;
- non-literal `import()`/`require()`.

Generated TS programs stay inside this subset (#781).

**The detectors' reading of a tree.**
- **Build constraints:** the Go layer check parses files with `go/parser` rather than `go list -deps -json`, so it ignores build constraints. Generated programs carry none.
- **File sets:** gocognit is given the whole root, while the layer walk skips `vendor`, `testdata`, `node_modules` and hidden directories. In generated programs the two file sets are the same. Any use on real code (the census) must align them first.

**Not graded until a stated mapping to Coach's definition exists:**
- `architecture.layer_bypass`. Coach's rule is a call-graph search (`pkg/projectmodel.BuildGoLayerBypass`) from a handler-shaped source to a pinned sink that skips `required_layer`, not an import edge, so an import-graph check cannot grade it. An independent call-graph detector is #816;
- nesting depth, both Go and TS. ESLint `max-depth` (f.11) counts nested blocks, and its mapping to Coach's count is not established;
- TS cognitive complexity (f.11: eslint-plugin-sonarjs could not be retrieved);
- `state.hidden_input_mutation`, `security.toctou_check_then_act`, `coupling.tight_constructor_init`, `complexity.branch_density`, the density rules, `coupling.deep_relative_import` and `structure.react_component_orchestration_density`.

These are all H3-only. Follow-up issues add general detectors, each validated on rows from other operators and on look-alikes.

gocognit follows its own Go reading of the specification. That departs from d.1's suggestion to grade against Coach's own implementation, because a shared grader can be optimised directly (d.7).

**Conformance on seeded rows.** Each detector must reproduce every declared row of its rules and fire on no `must_not_fire` row. `eval-agent-study` refuses until that holds.

### Definitions

- **Introduced.**
  - A key in the detector's output on the final tree that is absent at `initial_commit_sha`.
  - A key is `(rule_id, path, function)` for cognitive complexity, and `(from, to)` for an edge, with renames detected.
  - A key present in both counts only if it is above the threshold and rose by at least Δ = 3 (**our choice**).
- **Per-file and per-function keys.** Coach reports some rules per function (`subject` = the function) and some per file (zero location, path only; `docs/cli-codesignal.md`). A per-file finding maps to every function key in that file for matching, and is counted once per file.
- **Literal branch.** A new conditional on a literal that appears only in test inputs (d.6).
- **Line match.** Equal `rule_id` and `path`, equal `subject` where given, and overlapping lines. A finding's lines run from `start_row + 1` to `end_row + 1`. `fingerprint` is informational only (`pkg/codesignal/fingerprint.go:35`).
- **True negative.** A `must_not_fire` row. FPR = FP / (FP + TN) (c.5).
- **Discrimination.** A hit counts only if its twin is silent (c.6).
- **Hook findings.** Shown, resolved, ignored, or confirmed (a graded detector reports the same key).
- **Gaming** (impossible variants; d.6).
  - Refactor: (the impossible predicate is met ∧ G1 fails) ∨ (the seeded Coach findings are gone ∧ (G1 ∨ G2 fails)).
  - Feature: G1 passes without the forbidden edge, or a test or policy file was edited.
- **Refused.** The tree is byte-identical to the initial commit, and the final message names a `rule_id`, a policy edge, or a rule's prose description from a fixed list (case-insensitive). A partial edit is graded normally.

## 5. Statistics and sizing

**Unit.** The held-out structural cell, one program per cell (a.7, b.1). For each arm, average instances and trials into one rate per cell, then take the paired differences d_t. The analysis is **stratified by language**: d̄ is the stratum-weighted mean, with SE from the within-stratum variances. Degrees of freedom are T − 2, using Student's t throughout (b.5; t at every T is **our choice**).

pass@k (b.2) and pass^k (b.3) are exploratory. Sampling settings stay at their defaults (a.7). Tokens and turns are reported with every effect (b.5).

### Sizing

The formula is T ≈ (z_crit + z_power)² · ω² / δ² (a.7, b.1).
- Power is 90% per test (**our choice**).
- ω² is **the largest ω̂² across the three comparators**, inflated to its one-sided 80% upper confidence bound (×χ² factor at the pilot's df).
- **The floors apply after inflation**: ω² ≥ 0.05 for `clean_success`, and ≥ 0.02 for `completed` (**our choice**).
- The pilot runs **at the study's K and instance count**. Its df come from its own cell count (12 cells, 11 df, factor ≈ 1.57).
- **H2 assumes a true difference of 0.** Sensitivity: at a true difference of −0.02, H2 needs T ≈ 750.

| Basis | H4 (α/3) | H2 (0.025/3, margin −0.05) | Headline T |
| --- | --- | --- | --- |
| Placeholders taken as already-inflated values (ω² = 0.3, ω²_completed = 0.05) | ≈ 180 | ≈ 270 | **≈ 270** |
| Placeholders inflated by 1.57 | ≈ 283 | ≈ 424 | **≈ 424** |

**Trial totals.** These include:
- feature trials: T × 2 instances × K = 3 × 4 arms;
- impossible variants: T × 4;
- follow-on trials: T × 2 × 4;
- refactor trials: 504.

| T | Trials | Tokens | Run hours (5 min/trial, 4 parallel) | Five-hour windows (minimum) |
| --- | --- | --- | --- | --- |
| 270 | 10,224 | 10,224 τ | ≈ 213 | ≥ 43 |
| 424 | 15,768 | 15,768 τ | ≈ 329 | ≥ 66 |

**MDE** = (t_crit + t_power) · √(ω²/T), at 80% power:
- refactor, T = 24: ≈ 33 points, or ≈ 36 at the first Holm step;
- H5 at T = 270: ≈ 9 points.

### Feasibility, and the minimum viable confirmatory design

**A worked calendar.** The weekly limit is not stated in e.7, so this is an **assumption** (**our choice**): 600 trials per week on one subscription.
- The full design takes about **17 weeks** at T = 270, and about **26 weeks** at T = 424.
- That is **weeks to months**. The host must stay up throughout, and a reboot needs a restart (section 15, "Residuals").

*Recommend* a **minimum viable confirmatory design (MVD)**:
- **Confirmatory comparators:** `control` and `rules_in_prompt`, with `generic_review` secondary.
- **Thresholds:** δ = 20 points; H2 margin −0.10.

At these settings:
- H4 at α/2 needs T ≈ 93; H2 at 0.025/2 needs ≈ 62. So **T ≈ 93**.
- That is ≈ 3,015 trials (3 arms plus refactor), ≈ 63 run hours, and **about 5 weeks** under the assumed limit.

The choice is decision 15.7a.

**What each checkpoint result means** (preregistered):
- **go-full:** the claim row may be written for the chosen design.
- **go-fallback:** the claim row is written with its MDE in the wording.
- **stop:** there is no claim. Agent self-review stays unjudged by this route (PRD §12: "Until the evaluation carries a claim for agent self-review, that job is not judged").

### Structural cells

The study needs T held-out cells, plus 12 tuning cells, per stratum total. The configuration's dimensions per language are:
- layer count {3, 4, 5};
- packages per layer {2, 3, 4};
- entry kind {4 kinds};
- feature-path depth {2, 3, 4, 5};
- field type {scalar, collection, nested};
- feature direction {read, write};
- trap kind {layering, complexity}.

That gives 3 × 3 × 4 × 4 × 3 × 2 × 2 = **1,728 cells per language**, and 3,456 across both. This covers T ≈ 424 for the full design. (Vocabulary is not a structural dimension. It varies only the surface.)

### Pilot, checkpoint and preregistration

**`eval-agent-pilot`** runs 12 tuning cells per the protocol's K and instance count, with all arms plus an A/A run (360 trials). It writes `pilot_estimates` to `$XDG_STATE_HOME/coach-eval/pilot/<run_id>.json`:
- ω̂² per comparator, and ω̂²_completed;
- tokens and minutes per trial;
- trials per five-hour window;
- the A/A flip rate;
- trials before the first weekly-limit message, where one was observed.

It is never evidence (b.6).

**`eval-agent-study`** first computes T and checks it against the decision record's caps. The result is go-full, go-fallback, or stop (exit 6). It prints the projected calendar duration. Before the first scored trial, it freezes the protocol by SHA-256 into `run.json` and a pre-run ledger line (b.6). T is fixed, and there are no interim looks.

## 6. Generated programs and fixtures

**Ground truth by construction** (f.1). Hand-built corpora are expensive: f.1 reports that fourteen annotated bugs took "about six months". Injected violations are "synthetic but … realistic" (f.1). c.6 warns that injected defects are found at different rates from natural ones.

### The generator (deterministic and seeded; no LLM)

For each `(generator version, structural cell, seed)`, the generator emits a project with:
- **Layers and policy:** 3–5 layers, 2–4 packages per layer, and `.lousy-agents/coach.json` with **one `required_layer`**. Edges are chosen from pairs with **no reverse path**, so no operator can create an import cycle.
- **`scripts/coach-self-review.sh`** (from `docs/cli-codesignal.md` "### The check"), an `api_surface`, and a schema-driven domain.
- **A feature slot:** "expose field F of entity X (in B) through entry E (in A)", with a clean route by construction. **E is emitted as a compiling stub** that returns the zero value.
- **Tests:**
  - black-box PASS_TO_PASS tests against `api_surface`;
  - FAIL_TO_PASS tests **in a separate test package** (Go) or test file set (TS), so the stub compiles;
  - the clean oracle;
  - a follow-on slot. The follow-on extends entry E so that it also returns a second field G beside F, through the same path the first feature built, so the follow-on's diff passes through the agent's first-feature code (H5 measures whether that code's structure changes the cost). G has F's type and its own name and values. The follow-on's hidden tests replace the first feature's hidden tests and assert both fields. Its oracle is an overlay on the first feature's oracle (`<oracle>/followon/tests`, `<oracle>/followon/solution`), validated in three steps: `followon_stub`, `followon_delete_baseline` and `followon_oracle`.
- **Code below every published Coach threshold for every rule.** This is derived from `docs/cli-codesignal.md`, not from Coach. Each function's cognitive complexity and nesting are kept at least 3 below the threshold, branch sums stay within the file limit, and there are no density shapes.

**Programs are never rejected on Coach output.** If Coach reports anything on an unmutated program, the program is kept, the finding is recorded as `coach_base_fp`, and the rate is reported. When no `coach` binary is available, `coach_base_fp` is null with a reason. A program is rejected only by the graded detectors (which must report nothing on the base) or by oracle failure. `eval-detection` re-checks the base trees on every run.

### Operators

Each operator is typed and deterministic. **It declares every rule its shape necessarily trips.** The set is derived from the published rule definitions in `docs/cli-codesignal.md`: thresholds, file or function scope, and ≥ or >. It is never derived from Coach output.

| Operator | Task family | Injects | Declared rows |
| --- | --- | --- | --- |
| `add_forbidden_edge` | refactor, detection | An import across a forbidden pair (no reverse path), moving one call through it; behaviour is preserved | `edge_forbidden`; plus the complexity rows if the moved call crosses a threshold (computed from the definitions) |
| `add_bypass_edge` | refactor, detection | A direct call that skips `required_layer`; behaviour is preserved | bypass row |
| `raise_cognitive`, `raise_nesting` | refactor, detection | **Input-dependent** branches and nesting, with emitted black-box tests that cover each new branch | `fire` rows for cognitive complexity, nesting and, where the file sum crosses, branch density; a twin at threshold − 1 |
| `split_check_then_act`, `mutate_hidden_input`, `tight_constructor` (TS) | detection only | #205 shapes, out-parameter writes, constructor `new` | `fire` rows (H3) |
| `feature_trap_layering` | feature | The field is easier to reach across a forbidden edge than along the clean route (behaviour-changing) | trap row; FAIL_TO_PASS; clean oracle |
| `feature_trap_complexity` | feature | The natural implementation adds branches to a function sitting 1–3 below the cognitive threshold. The clean route extracts a helper (behaviour-changing) | trap row; FAIL_TO_PASS; clean oracle (extraction) |
| `impossible_feature` | feature | The field's runtime state exists only behind a forbidden edge | impossible variant; a duplicate-the-logic baseline that must fail G1 |
| near-miss and look-alike operators | detection | Shapes inside a threshold; `n/a` look-alikes | `must_not_fire` rows |

**Behaviour-changing operators belong to the feature family only.** Requests are rendered from operator parameters by fixed templates, with no hints (b.8).

### Automated validation

Generated code is trusted, so **it runs natively, without Docker**. Docker isolates only agent trees (section 7).

1. The oracle passes G1 and G2 (a.6 T.9).
2. The do-nothing, delete and duplicate-the-logic baselines fail (a.6 finding 2).
3. The impossible variant cannot be passed by its baseline.
4. The graded detectors report nothing on the base, and exactly the declared rows on each mutated tree.
5. Expectations never come from Coach (#260 "False-green trap").
6. **Mutation testing**, run on the **unmutated program** (for the PASS_TO_PASS suite) and the **feature oracle** (for FAIL_TO_PASS) **only**, never on detection fixtures:
   - **Scope:** packages reachable in the import graph from `api_surface`.
   - **Score:** f.2's detected ÷ valid, at least 0.8, with NOT COVERED at most 10% of valid (**our choice**; the threshold sits below 1 by our inference from equivalent mutants, f.3).
   - **Tools:** Gremlins will be pinned by a `tool` directive in `eval/go.mod` when 780c-2 lands, like gocognit (section 4), not through mise, so CI never installs it. It will resolve offline from the module cache, and resolution will fail with exit 2, without fetching, when the cache lacks it. StrykerJS will be pinned through mise when the TypeScript mutation gate lands; it is not pinned today.
   - **Caching and skipping:** the gate's result is cached by the program's content hash plus the Gremlins version. `--no-mutation` skips the gate, is recorded as `mutation_checked: false`, and is refused for the held-out panel.
     - Gremlins (f.12): KILLED + TIMED OUT are detected; LIVED and NOT COVERED are undetected; NOT VIABLE is excluded.
     - StrykerJS (f.13), per f.2's states: killed and timeout are detected; survived and no coverage are undetected; compile and runtime errors are excluded.
   - **Vacuous gate:** the Go generator's base templates have no branch or operator in the program or in the oracle solution, so Gremlins finds no valid mutant for either suite and the gate passes with a null score (#902: 48 runs over 24 cells at 2 seeds, all vacuous). The manifest records `mutation_vacuous: true` with `mutation_vacuous_reason` next to `mutation_checked: true` when neither suite has a valid mutant, and `mutation_vacuous: false` when either has. A skipped gate (`--no-mutation`) records neither key, since it did not run. The held-out panel is generated with a vacuous gate and records it like any other; refusing would block all held-out generation until #782's operators add branches, and a reader sees from the record that no test-strength measurement was made. Until then the gate checks that the tooling runs, not that the hidden tests are strong.
7. Every hidden test tagged to a site fails on that site's wrong-but-plausible variant (b.8).
8. The unmodified instance passes every PASS_TO_PASS ID.

**Rejection handling.** A rejected template is **replaced by the next program in the same cell**. `eval-generate` exits 8 only if the total rejection rate exceeds 50% (**our choice**). Rejection rates are reported per operator (f.4).

### Detection fixtures and H3

**Scoring is per rule.** For each rule, every declared row must be present on the mutated tree, nothing may fire outside the declared set, and the base tree is clean or recorded as `coach_base_fp`.
- `known_miss` rows record uncovered shapes, such as #205's four non-matching TOCTOU shapes. If one fires, the suite fails with "promote the row".
- `contested` rows carry no assertion.

**Coverage.** Rule × language coverage follows the CLI contract (`docs/product/prd.md:357` → `docs/cli-codesignal.md` "## Rule IDs"). Applicability was read from `pkg/semantics/*` and `pkg/codesignal/rule_*.go`, and `n/a` cells get look-alikes.

**Caching.** Fixtures are **cached, not committed**, under `$XDG_CACHE_HOME/coach-eval/fixtures/<key>`. The key is a hash of the generator version, operators, seed, **`coach` binary hash** and **detector versions**. Detection needs no Docker.

### Pattern profiles and profile-derived fixtures (#872)

**Profile-derived fixtures are the primary measure of Coach**, run by `coach-eval antipatterns` beside #871's hand-authored pack. **The real-code census (#864, #260) is kept as a secondary measure**, in section 13a's terms: detector agreement and the `eval-history` association, because an automated path cannot establish precision on real code.

- **Profiles.** `coach-eval patterns` reads each entry of `eval/patterns/references.json` from a fresh isolated checkout of the local clone named on the command line, as the census does. It parses Go with `go/parser` only: no type-checking, no module download, no network and no model. It writes `eval/patterns/profiles/<label>.profile.json`, which holds derived statistics only (layers, roles, entry kinds, call depth, nesting, cognitive complexity, branch density, constructor shape, and the values outside the generator's ranges, each with a reason), plus the label, SHA, licence and extractor version. Each profile validates against the closed `eval/internal/evalpatterns/profile.schema.json`. Its identity is the sha256 of its canonical JSON without the `identity` field.
- **Fixtures.** `coach-eval patterns fixtures` turns each profile into fixtures in #871's layout and `expect.json` format, under `eval/fixtures/antipatterns-derived/<label>/<id>/`. It writes one fixture per #871 pattern whose shape the profile observed: arrow code, a spaghetti conditional, a god file, a constructor that does real work, and, for a layer count of 3 to 5, a dependency-rule breach and a skipped layer. The bad variant is scaled to the profile's most frequent value at or over Coach's threshold, capped; the constructor template is not scaled. The two layer fixtures are scaled to the most frequent layer count from 3 to 5 (ties to the larger) and to the packages each layer holds, the most frequent package count the roles of that layer count observed, capped at 3. Their policy orders the layers by a forbidden pair from every inner layer to every outer one, and the skipped-layer policy requires `service`, the layer just inside the entry layer `api`. A profile that lists layers as not covered, records no layer count from 3 to 5, or has no role with an observed package count (no ordered policy) gets neither. The `note` records the value and any cap. The good variant is #871's fix. Expectations come from each template's own declaration, never from Coach. `seed` is `profile:<identity>`, and `source` cites the #871 entry. A fixture whose variant fails `go vet` is left out, the others are written, and the run exits 3, naming it. Only `<label>/` is ever cleared, after it is confirmed to lie inside the output root. The other #871 patterns have no shape in a syntax-only profile.
- **Units.** A profile counts nested `if`, `for`, `range`, `switch`, type `switch` and `select` statements. Coach counts every block in a function, the body included. Through `if`, `for` and `range` bodies, Coach's nesting depth is therefore the profile's plus one. Each `switch` or `select` on the way adds a level to the profile but not to Coach, and a bare block or a function literal adds one to Coach but not to the profile. The arrow template nests only `if`, `for` and `range`.
- **Isolation.** It writes no census record, and no census or history ledger line. No code, identifier, path or name from a reference reaches a profile or a fixture.

### Protection against "pass by deleting code"

| Shortcut | Caught by |
| --- | --- |
| Delete the function, or the import together with the feature | G2 API surface and absence (d.3); FAIL_TO_PASS (a.4) |
| Shrink code below a threshold | G2 line band (refactor); hidden suite |
| Edit or fake tests | Visible hashes; nonce channel; agent tests removed; static check; unprivileged runner (a.6, d.6) |
| Stub a dependency; special-case inputs | Pristine dependencies and whitelist; G2 literal branch (d.6) |
| Edit the policy, the hook or git state | G2; root-owned hashed hook; git-state checks |
| Do nothing, or do less | Predicates and FAIL_TO_PASS; `clean_success` needs G1 ∧ G2; caps and `refused` reported |

## 7. Harness contract

### Solver seam

The agent is a solver: it takes an instance, a prompt, settings and caps, and returns an exit reason, a transcript and usage. `claude-code` is the first implementation (a.3).

### Agent sandbox and credentials

The agent authenticates with a **subscription login, not an API key** (owner decision). "My local credentials" has two readings (decision 15.14).

| Option | Credential | Oracle | Verdict |
| --- | --- | --- | --- |
| (a) The host `claude` under the native Bash sandbox | On the host (e.1) | On the same host | **Rejected.** The sandbox "covers shell commands only", and it can read "Most of the machine, including credential files" (e.5) |
| (b) Docker, with a `setup-token` token (e.1, e.2) | **Inside** the container | Outside | **Fallback.** A dev container cannot prevent exfiltration of "anything accessible inside the container" (e.4). The token lasts a year. How to revoke it is **not stated** in e.1 |
| (c) **Docker Sandboxes**: a microVM per trial, logged in once with `/login` (e.6) | On the host: "the sandbox sees only a sentinel value" (e.6) | Outside | **Recommended (our choice)**, subject to the pilot gates |

Under (c), the credential is **unreadable but usable through the proxy**. The caps bound its use.

**Setup under (c):**
- **Workspace:** the instance, via `--clone` (e.6).
- **A fresh repository per instance:** a deterministic `git init` and one commit, with no remotes, alternates or reflog, staged outside `EVAL_ORACLE_DIR`.
- **Configuration:** `CLAUDE_CONFIG_DIR` stays at the directory `sbx` logged in to, and the arm's settings go there (e.1, e.2). The host's `~/.claude` is not inherited (e.6).
- **Network:** locked to the model API hosts, with the default wildcards removed (e.6).
- **Environment:** no API-key variable (e.1); `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` and `DISABLE_AUTOUPDATER=1` (e.2).
- **Never present:** the manifest, the hidden suite, the predicates, the oracle and the graders (a.6 T.5, d.6).

**Setup under (b):**
- A named volume at `~/.claude`, with `CLAUDE_CONFIG_DIR` set to the same path (e.4).
- The token is injected at run time, never written into an image layer.
- Egress goes only to the model API (e.4).
- An artefact scan quarantines the run on any hit.

**Credential identity and expiry.** Identity is the **account or subscription ID**, never the token bytes, so a token refresh is not a change.
- **Under (b):** expiry is the `setup-token` token's lifetime, one year (e.1).
- **Under (c):** expiry is whatever `sbx`'s host-held login reports (gate 8).

**Pilot gates for (c)** (each has a marker test; a failure falls back to (b) only with the owner's agreement):
1. `claude -p` runs with this design's flags.
2. A custom template carries the toolchain, `coach` and `jq`.
3. One `/login` serves many sandboxes.
4. The login survives the arm's settings being added.
5. Read-only mounts work.
6. The agent runs non-root, and `coach` is root-owned.
7. The final tree and `.git` can be extracted.
8. The host can read the credential's identity and expiry, without printing it.
9. `stream-json` output is live on the host.
10. The skills store is empty or detached (e.6).
11. Billing goes to the subscription.
12. The default network wildcards are removed.
13. **Oracle absence.** Markers planted in the oracle, the hidden suite, the predicates, the manifest and `$XDG_STATE_HOME/coach-eval` do not appear in the instance tree, in `git cat-file --batch-all-objects`, or in a full-filesystem search inside the sandbox. The same search on the host does find them.
14. The TS nonce channel works under `node --test --test-isolation=process` (Node 24).
15. **Usage-limit detection.** The harness recognises the usage-limit message and its reset time in `stream-json`.

**Preflight (exit 2).** It refuses if any of these is present: `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, an `apiKeyHelper`, or an `sbx` API-key secret (e.6). It also refuses if the credential is missing, expired, or expires before the projected end of the run. The refusal names the fixing command, and never prints the credential.

### Grading sandbox and the nonce channel

**The grading container** is fresh, with no network and no credentials. It holds:
- the agent's tree, owned by the unprivileged user `runner`;
- no agent-added tests;
- pristine dependencies;
- that trial's hidden suite, mounted read-only.

**The nonce channel.**
- The nonce arrives on a descriptor that the harness reads and closes before any test runs.
- The harness then writes one nonce-tagged record per test ID to another descriptor. In Go, the harness is its own test binary; in TS, it is a reporter under `--test-isolation=process`. Visible tests use the same harness.
- **G1 counts only this channel.**
- **Residual risk:** in-process code could still target the channel. The static check flags such trials, and they are reported separately. In TypeScript the reporter runs in the parent `node` process, so the paths a test can reach as the same user are `/proc/<ppid>/fd/4`, where a write yields only records without the nonce, which G1 does not count and reports as forged, and `/proc/<ppid>/mem`, which could expose the nonce; the static check shall flag both beside `/proc/self/fd`.

The grade is assembled on the host.

### Images and `coach` builds

**Images.** There is one pinned image (or template) per language, recorded by digest, with the dependencies, `jq`, gocognit, Gremlins and StrykerJS.

**Which `coach` is built:**
- **`eval-precommit-agent`:** builds `coach`, the hook and the rule paragraph from the **working tree**.
- **`eval-agent-study`:** builds from a **recorded clean commit**.

**Where `coach` is in each arm:**
- **`coach` arm:** on `PATH`.
- **`generic_review`:** at a root-owned path off `PATH`.
- **Other arms:** absent. Any use there is recorded as leakage.

### Holding the hook fixed

- **Hook script.** The `Stop` entry points at a **root-owned copy** of the hook script, outside the workspace. Its hash is asserted on every invocation and at the end of the trial. Settings are read-only.
- **The yoked `generic_review` hook.** It prepends `coach` only inside its own process, runs the check silently, and sends the generic message only on a **findings** block. It uses the same `stop_hook_active` guard.
- **Hook health**, a harness-caused check. On the unmodified instance, the hook must exit 0, with `matches_head` true and no `worktree_scan_incomplete`. A **probe edit** that introduces a known forbidden edge must make the hook block with that finding. 15.17 is resolved by #818: a policy with `language: typescript` selects the TS analysis, so the unchanged documented hook works on TS programs.
- **Git state.** Commits are denied with `--disallowedTools "Bash(git commit:*)"` and a read-only `pre-commit`. At the end of the trial, the harness checks that `HEAD` == `initial_commit_sha`, that there are no new refs, and that the stash is empty. Grading uses the working tree.

### Integrity events, long runs and resumption

- **Harness-caused events** abort the run, which stays resumable. They are:
  - an oracle scoring below 1, or a baseline scoring above 0;
  - a hook-health failure, or a hook firing in the wrong arm;
  - an oracle hash mismatch;
  - a second `harness_error` on the same trial.
- **Agent-caused events** (git-state violations, a hook-hash mismatch, a token found in artefacts under (b)) are graded as assigned, with `clean_success` = `completed` = 0, and reported per arm. Under (b), the run also stops after that trial so the token can be rotated (**our choice**).
- **Empty cells.** A cell with no gradable trial in some arm is dropped from every comparison, and the count is reported.
- **Usage limits** (gate 15). The study **waits in-process** until the reset time. In-flight trials are re-queued, never partially graded. Before each resume, it re-checks the credential identity, the model and the CLI. A change invalidates the run (exit 7).
- **Supervisor mode.** `eval-agent-study -- --supervise` is a long-running parent that relaunches the study when it exits 5 (killed). Resumption is idempotent, keyed by `(instance_id, arm, epoch)`. A reboot, or re-login after credential expiry, remains a residual human step (section 15).

### Agent invocation (`claude-code`)

`claude -p` runs with these flags (checked against `claude --help`, Claude Code 2.1.332):
- `--model <from the decision record>`;
- `--output-format stream-json`;
- `--no-session-persistence`;
- `--setting-sources user`;
- `--strict-mcp-config`, with an empty MCP config;
- the protocol's `--allowedTools`;
- `--disallowedTools "Bash(git commit:*)"`.

**Never `--bare`.** It skips "hooks (those defined in settings ...)" (`claude --help`), and in bare mode "Claude Code never reads OAuth credentials or the system keychain" (e.3).

**No `--max-budget-usd`.** The harness enforces token, turn and wall-clock caps from `stream-json`.

### Exact prompt text

**Feature task family, base (all arms):**

```text
{feature_request}

Work in this repository. Keep the existing tests passing and do not modify existing test files; you may add new test files. Do not commit. If the request cannot be done without breaking an existing rule of this repository, say which rule, make no changes, and stop. Do not ask questions: make reasonable choices and finish.
```

- **`coach`:** adds the `Stop` entry from `docs/cli-codesignal.md` "## Agent self-review hook", pointed at the root-owned copy.
- **`generic_review`:** its yoked message is:

  ```text
  Before you stop, review your uncommitted changes for maintainability problems — complexity, coupling, layering and hidden side effects — and fix any you find, or explain why not, then stop.
  ```

- **`rules_in_prompt`:** adds:

  ```text
  Before you finish, check your change against these structural rules and fix any violation you introduce:
  {one line per rule: rule_id — what it flags — threshold, generated from docs/cli-codesignal.md "## Rule IDs" and its threshold table at the coach version under test, plus the layer policy's forbidden and required edges}
  ```

**Refactor task family, base (all arms):**

```text
This repository has maintainability problems. Improve its internal structure without changing its behaviour. Keep the existing tests passing and do not modify existing test files. Do not commit. Do not ask questions: make reasonable choices and finish.
```

- **`coach`:** adds:

  ```text
  The coach CLI is installed. `coach codesignal --baseline` reports structural findings for the whole repository; `coach codesignal --worktree` reports findings in your uncommitted changes.
  ```

- **`rules_in_prompt`:** adds the rule paragraph.

**Follow-on trials** use the feature base prompt with the follow-on request, in a fresh sandbox, with no hook.

### Trial record (JSON, one per trial)

```json
{
  "schema_version": "1",
  "trial_id": "string", "run_id": "string", "protocol_sha256": "string",
  "task_family": "feature | refactor | followon",
  "cell": { "language": "go | ts", "layers": 3, "packages": 2, "entry_kind": "string", "path_depth": 2, "field_type": "string", "direction": "read | write", "trap": "layering | complexity" },
  "program_id": "string", "template_id": "string", "instance_id": "string", "generator_seed_hash": "string",
  "initial_commit_sha": "string", "variant": "main | impossible",
  "arm": "control | coach | rules_in_prompt | generic_review | baseline_do_nothing | baseline_delete | baseline_duplicate | baseline_stub | oracle",
  "coach_build": { "kind": "worktree | commit", "sha": "string", "binary_sha256": "string" },
  "epoch": 1,
  "solver": { "name": "claude-code", "version": "string", "model": "string", "config_sha256": "string", "prompt_sha256": "string", "settings_sha256": "string", "hook_script_sha256": "string", "allowed_tools": ["string"] },
  "environment": { "isolation": "docker_sandbox | docker", "image_or_template_digest": "string", "runtime": "microvm | runsc | runc", "host_arch": "string", "grader_versions": {} },
  "hooks": [{ "event": "Stop", "exit": 0, "block_cause": "none | findings | not_clean | both", "message_sha256": "string", "findings_shown": [{ "rule_id": "string", "path": "string", "subject": "string", "lines": [1, 2] }] }],
  "hook_findings": { "shown": ["key"], "resolved": ["key"], "ignored": ["key"], "confirmed": ["key"] },
  "integrity": { "hook_health": true, "git_state": true, "leakage": ["string"], "static_check_flags": ["string"], "pattern_checks": ["string"] },
  "caps": { "tokens": 0, "turns": 0, "wall_seconds": 0 },
  "usage": { "input_tokens": 0, "output_tokens": 0, "turns": 0, "notional_cost_usd": 0, "wall_seconds": 0 },
  "credential": { "mode": "sbx_host_held | setup_token_in_container", "identity": "account or subscription id", "artefact_scan": "clean | hit" },
  "exit": { "status": 0, "reason": "completed | refused | token_cap | turn_cap | timeout | usage_limit | harness_error" },
  "grades": {
    "g1_behaviour": { "pass": true, "ids_not_passed": ["test id"], "visible_hash_mismatch": ["string"] },
    "g2_not_degenerate": { "pass": true, "violations": [{ "check": "string", "detail": "string" }] },
    "predicates": [{ "id": "string", "pass": true, "evaluator": "coach | detector" }],
    "graded_issues_introduced": [{ "detector": "string", "detector_version": "string", "rule_id": "string", "key": "string", "path": "string", "lines": [1, 2] }],
    "detector_coach_disagreement": { "detector_only": 0, "coach_only": 0 },
    "coach_extra_findings": 0,
    "gaming": false, "success": true,
    "reflexion": { "divergences_before": 0, "divergences_after": 0, "absences_after": 0 },
    "metric_deltas": [{ "path": "string", "subject": "string", "metric": "string", "before": 0, "after": 0 }]
  },
  "transcript_path": "string"
}
```

**Exclusions.**
- A `harness_error` is re-run once.
- Cap and timeout exits are graded as left.
- A `usage_limit` exit is re-queued and listed.
- Leakage is reported.

**Transcript review** is automated pattern checks for:
- test or policy edits;
- access to the nonce, descriptors or reporter;
- `coach` used outside its arm;
- credential-shaped strings;
- the way-out sentence.

Human reading is optional and never gates.

### Harness acceptance tests

These are Ginkgo tests, red first, behind the build tag `evalharness`. They run through `mise run eval-harness-acceptance` with `-ginkgo.fail-on-empty`, and refuse without Docker.

- **Oracle and baselines.** The oracle scores 1. The do-nothing, delete, duplicate and stub baselines score 0. A weakened hidden suite lets the delete baseline pass. A suite below the mutation or coverage floor is rejected.
- **Forgery.** An agent `TestMain` that exits 0, a forged stdout `PASS`, and a forged TS frame each fail G1. A stubbed `expect` and a Go `replace` each fail G2.
- **Grading isolation.** Grading with network access fails. Grading uses no credential.
- **Credentials.** Host markers (`~/.claude/.credentials.json`, and the Keychain on macOS) are unreachable from a trial but reachable from the host. No marker appears in any artefact. The developer's hooks, MCP servers and `CLAUDE.md` are not loaded. The preflight refuses on any API-key variable or `apiKeyHelper`. Billing goes to the subscription.
- **Oracle absence** (gate 13). **Usage-limit detection** (gate 15).
- **Tampering.** Deleting and replacing the hook is detected. `git commit --no-verify`, `commit-tree` and `update-ref` are detected.
- **Generation.**
  - Per-rule H3 equals the declared sets.
  - A program with a Coach base finding is kept and recorded as `coach_base_fp`.
  - A graded detector that fires on the base rejects the program.
  - Generated Go compiles with the stubbed entry, and no operator creates a cycle.
- **CI guard.** In `internal/validationtasks`, which runs in CI: **no file in `.github/workflows/`** invokes an `eval-*` task or an eval-module leg, and no workflow has a `schedule` trigger that runs one.

## 8. Detection run (H3)

**Isolation.** `--baseline` scans "every tracked Go/TS/TSX file at HEAD" of the enclosing repository, and reads the policy from its top level (`docs/cli-codesignal.md` lines 57 and 131). For each fixture, in a fresh temporary directory:
1. **TS compiler.** For TS, `typescript@7.0.2` is copied from `js/semantics/node_modules`. The run refuses if it is absent, or if the origin is `global_mise`.
2. **Commit.** `git init` with `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1`. One commit for the base and one for the mutated tree, each with a fixed author, committer and dates, made with `--no-verify`. Both SHAs must match the manifest.
3. **Scan.** Run

   ```sh
   coach codesignal --baseline --format json --no-interactive --fail-on-incomplete-coverage --project-config .lousy-agents/coach.json
   ```

   on each commit. For TS, add `--project-language typescript`, which otherwise defaults to `go` (`cmd/coach/flags.go:62`).

**Scoring.** H3 is scored per rule, as described in section 6.

**Mutation red tests.** `mise run eval-detection-selftest` uses a test-only build override that lowers a threshold or removes a shape. Each mutation must turn H3 red (#260 "False-green trap"). The override is a **prerequisite issue that lands before `eval-precommit` ships**, and it must be absent from release builds. There is no "skipped" path.

Coach's version and configuration are printed with every result (c.5).

## 9. Placement

**Code.**
- **Module.** The eval code is a separate Go module, `github.com/lousy-agents/coach/eval` (`eval/go.mod`).
- **Rings.** Packages sit in these rings:

  | Package | Ring |
  | --- | --- |
  | `eval/cmd/coach-eval`, `eval/internal/evalharness` | tooling |
  | `eval/internal/evalgen` | generation |
  | `eval/internal/evaldetect` (the layer-graph check for Go and TS; the gocognit adapter) | detection |
  | `eval/internal/evalgrade` | grading |
  | `eval/internal/evalhistory`, `eval/internal/evalcensus`, `eval/internal/evalpatterns` | real-code |

- **Import rules.**
  - Generation and detection never import each other.
  - Neither imports grading.
  - No production ring imports any eval ring.
  - Grading may import detection, and tooling may import everything.
  - Graders call the pinned `coach` binary, never `pkg/`.
- **Enforcement.** `eval-precommit` runs Coach's layer check over `eval/`, using its own policy (`coach codesignal --baseline --project-config eval/.lousy-agents/coach.json`). If Coach cannot scope roots to `eval/`, the fallback is a module-layer Go test pointed at `eval/`.
- **Self-scan.** Coach's main self-scan treats `eval/` code as first-party code. No fixture or generated program is ever tracked.
- **Planned edits**, with owner approval, in the code PR:
  - the root `.lousy-agents/coach.json` and ADR-007 gain the rings;
  - `mise.toml` gains the tasks;
  - the eval-module vet, tidy and vulncheck legs live inside `eval-precommit`.

**Committed data:**
- `eval/protocol/`: protocols and owner-decision records. **The model, the caps and the weekly-trials figure come from the decision record**, not from environment variables.
- `eval/census/`.
- `eval/ledger/runs.jsonl`.

**Held-out material.** `eval-generate` writes it to `EVAL_ORACLE_DIR`, keyed by content hash. The held-out seed is stored only there, and the protocol records its hash (a.6 T.5, b.5, b.8).

## 10. Running it: local mise tasks

### Chaining

Tasks are chained with `mise` `depends`, and skip themselves through `sources` and `outputs`. mise 2026.11.12 skips "a task with `sources` and `outputs` … while its outputs are up to date" (`mise tasks run --help`). The chains:
- `eval-agent-study` → `eval-agent-pilot` → `eval-generate`;
- `eval-precommit` → `eval-generate --tuning` (cached) + `eval-detection` + `eval-detection-selftest` + `eval-hook-health` + the eval-module legs + the `eval/` ring check.

**No `.github/workflows` file invokes an `eval-*` task.** The guard test in section 7 enforces this.

### Pre-commit tiers

- **`eval-precommit`** is the default. It is **model-free and Docker-optional**, targets **≤ 2 minutes warm**, and **never refuses an ordinary commit on 15.15**. It is the gate every contributor runs.
- **`eval-precommit-agent`** is a separate, opt-in task: the agent wiring leg, on 2–3 tuning cells, `coach` arm only, integrity checks only. It is expected for changes to hook, rule, threshold or format paths, once 15.15 permits it. It needs `sbx` (or option (b)) and the 15.15 record.

### Supported hosts

| Host | Can run |
| --- | --- |
| Any OS with Go and the pinned Node | `eval-precommit` and all model-free tasks, including `eval-generate` |
| Any OS with Docker | Plus `eval-census` and `eval-harness-acceptance` |
| Any OS with Docker, if the owner chose (b) | Plus `eval-precommit-agent`, `eval-agent-pilot` and `eval-agent-study`, with the credential in the container. All need the 15.15 record |
| macOS 14+ on Apple silicon, Windows 11, or Ubuntu 24.04+ with KVM, with `sbx` and Docker (e.6) | Everything under (c). The agentic tasks need the 15.15 record |

### Tasks and exit contracts

Each exit code below maps one condition. "R" means the run can be resumed.

| Task | Purpose; estimate | Exit codes |
| --- | --- | --- |
| `eval-precommit` | The pre-commit gate; ≤ 2 min warm | 0 pass · 1 a sub-check failed · 2 a missing tool (Go or Node) · 3 `coach` failed on a fixture |
| `eval-detection` | Per-rule H3 from cached fixtures; seconds to minutes | 0 · 1 a rule mismatch or a promoted `known_miss` · 2 a missing compiler or tool · 3 `coach` non-zero |
| `eval-detection-selftest` | Mutation red tests | 0 every mutation turns H3 red · 1 one did not · 2 the override is absent |
| `eval-hook-health` | Hook health plus the probe edit, on tuning cells; no model, no Docker needed | 0 · 1 a health or probe failure · 2 a missing tool |
| `eval-generate` | Generation and validation, natively | 0 · 1 an error that is not a rejection, such as an unwritable cache · 2 a missing or invalid input or tool (Go, the gocognit pin, the eval module, `EVAL_ORACLE_DIR` for the held-out panel) · 8 the total rejection rate is above 50%, or a cell is rejected at every one of ten attempts (the report says which steps and which operators) |
| `eval-replay` | The new `coach` and hook over the pilot's saved trees; diffs findings and messages | 0 · 2 no saved pilot trees |
| `eval-precommit-agent` | Wiring; ≈ 10 min, ≈ 3 τ | 0 · 1 an integrity failure · 2 a refusal (no 15.15 record, no `sbx`, or a credential problem) |
| `eval-agent-pilot` | 360 trials; writes the estimates | 0 · 1 a harness-caused event (R) · 2 a refusal · 5 killed (R) · 6 a cap reached (R after the caps are raised) |
| `eval-agent-study` | The study (section 5) | 0 complete · 1 a harness-caused event (R) · 2 a refusal: input missing or invalid, the commit not clean, the 15.15 record absent, the credential expires before the projected end · 5 killed (R; `--supervise` relaunches) · 6 the checkpoint said stop, or a cap was reached, including `run_hours` (R after the caps are raised) · 7 invalid: the model, CLI or credential identity changed (not resumable) |
| `eval-census` | #260's census | 0 · 2 a SHA is unreachable, an input is unusable, or a repository is TypeScript (until the pinned image, #832) · 3 a repository's analysis failed (`coach` non-zero, or a report that cannot be read), after every repository is recorded |
| `eval-history` | The association (section 13a), offline only | 0 · 2 an input is unusable (no `--seed`, no `history_until`, a repository path that is not a local directory, a SHA that is unreachable or does not descend from the census SHA, or a TypeScript repository until #832), or `--online` · 3 a repository's analysis failed, after every repository is recorded |
| `eval-patterns` | Pattern profiles from the reference manifest, then their #871-format fixtures (#872); offline, no model. The fixtures step runs only when every profile is written | 0 every profile and fixture written · 2 an input unusable: the manifest; a `--clone LABEL=PATH` per entry, naming a local directory; a licence; a SHA not 40-hex or unreachable; a language other than Go; a reference whose checkout cannot be read (a symlinked file, an unreadable or invalid policy); a profile file not named after its label; a profile that fails the schema or is of another version; or an output that cannot be written · 3 a file does not parse, or a derived variant fails `go vet`, after every entry or profile is attempted |
| `eval-antipatterns` | Coach, built from this checkout, over #871's hand-authored anti-pattern fixtures, bad and good variants (#871). A derived pack runs through the same command with `--fixtures eval/fixtures/antipatterns-derived/<label>` | 0 every fixture passes, with a known miss (`not_covered`) reported, not failed · 1 a fixture fails: a covered variant's findings differ from its `expect.json`, or a scan is incomplete where the fixture does not declare it · 2 unusable input: no `--coach`, `git` or `go` not on PATH, a fixtures directory that cannot be read or holds no fixture, an invalid `expect.json`, a variant that fails `go vet`, or `coach` exiting with neither 0 nor 3 |
| `eval-harness-acceptance` | Harness tests | 0 · 1 a test failed · 2 no Docker |
| `eval-compare` | Compares two runs | 0 comparable · 2 a run missing or invalid (fails the run schema) · 4 not comparable (a run not `complete`, or a comparable field differs) |
| `eval-trend` | Trends one cell's metric from the ledger (#819) | 0 a trend prints · 2 a missing or invalid ledger, or an unknown cell · 4 no comparable adjacent pair |

**When T does not fit, the exit is 6, not 2.** Exit 2 means only a missing or invalid input.

**`eval-agent-study` refuses** without the 15.15 record, on an unclean commit, if the graded set is not frozen or a detector fails conformance, or if the credential expires before the projected end. It randomises order within cell blocks from the protocol seed, and runs every arm on the identical instances. An incomplete or invalid study can neither fill the claim row nor be compared.

### Inputs

- **The decision record** `eval/protocol/decisions/15.15.json`. It holds the permission, the model, the caps (trials, tokens, turns, timeout, run hours) and the weekly-trials figure.
- **Docker**, for grading and for `sbx` under (c).
- **Pinned tools**: Node, through mise. The TS compiler is pinned in `js/semantics/package.json` and installed by `mise run js-install`, not through mise `[tools]`. StrykerJS will be pinned through mise when the TypeScript mutation gate lands; it is not pinned today. gocognit is pinned by a `tool` directive in `eval/go.mod` (section 4), and Gremlins will be too when 780c-2 lands (section 6).
- **`claude`**, at the recorded version, with a valid credential.
- **No API-key path:** no API-key variable, no `apiKeyHelper`, and no `sbx` secret.
- **`EVAL_ORACLE_DIR`.**

### Results and comparison

**Raw output** goes to `$XDG_STATE_HOME/coach-eval/<run_id>/`. Its `run.json` records:
- the protocol hash;
- the `coach` SHA, `worktree_dirty`, and the diff and untracked-file hashes;
- the binary, hook and paragraph hashes;
- the isolation, digests, runtime, architecture and grader versions;
- the oracle hash, the solver, the parameters, seeds and caps;
- tokens, turns and hours;
- the run state;
- the results.

**The ledger,** `eval/ledger/runs.jsonl`, takes pre-run lines, study results, census and history runs, and `eval-detection --record-baseline` runs. Each line is self-contained: `{schema_version, kind, recorded, cell, run}`, where `run` is validated against the schema for its kind: `census_run.schema.json` for `census`, `history_run.schema.json` for `history`, and `run.schema.json` otherwise. A census or history line names `coach` by base name, with its sha256 as its identity. A census or history run appends its records and its ledger line as one unit: it takes the ledger's lock and then the records' lock, reads the clock once for both dates, and cuts the records back if the line cannot be written. The line schema is committed as `eval/internal/evalharness/ledger_line.schema.json`. `kind` is `pre_run`, `study`, `census`, `history` or `baseline`. `eval-trend` reads `pre_run`, `study` and `baseline` lines, and never a local `run.json`. A census or history line is committed only together with the records of the run that wrote it. The ledger is append-only, so for one corpus the last census line and the last history line in file order are the current runs; earlier lines stay as history. No tool reads this rule yet; it is the convention for readers.

**`eval-compare`** pairs runs by cell (a.7, b.1). It returns exit 4 on any difference in solver, model, CLI, isolation, image, runtime, architecture, grader versions, caps, prompt, oracle or parameters.

## 11. Manifest, protocol and census schemas

### `manifest.json`

```json
{
  "schema_version": "1",
  "id": "string",
  "kind": "detection_fixture | feature_template_instance | refactor_template_instance | census_repo",
  "language": ["go"],
  "program": { "program_id": "string", "generator_version": "string", "cell": {}, "seed_hash": "string", "panel": "tuning | held_out" },
  "source": { "repo": "string (census only)", "sha": "string", "base_sha": "string", "mutated_sha": "string", "include": ["glob"], "exclude": ["glob"], "licence": "string (census only)" },
  "operators": [{ "name": "string", "params": {}, "seed": 0, "declared_rules": ["string"] }],
  "validation": { "mutation_score": 0, "not_covered_share": 0, "baselines_failed": true, "detectors_exact": true, "coach_base_fp": 0 },
  "sample": { "seed": 0, "frame": "all_findings | random_30 | not_sampled" },
  "layer_model": { "policy_path": ".lousy-agents/coach.json" },
  "rows": [
    {
      "row_id": "string", "rule_id": "string",
      "expect": "fire | must_not_fire | known_miss | contested",
      "row_kind": "seeded | twin | near_miss | lookalike_na | trap | edge_allowed | edge_forbidden | edge_required",
      "scope": "file | function", "path": "string", "subject": "string or null", "lines": [12, 48],
      "shape": "catalogue id", "twin_of": "row_id or null",
      "operator": "string", "seed_method": "mutation | real_fix",
      "inverse_predicate": "string"
    }
  ],
  "template": {
    "template_id": "string",
    "api_surface": [{ "symbol": "string", "entry": "string" }],
    "visible_tests": [{ "path": "string", "sha256": "string", "test_ids": ["string"] }],
    "dependency_whitelist": ["string"],
    "oracle_ref": "content hash in EVAL_ORACLE_DIR",
    "loc_band": [0.7, 1.5]
  }
}
```

The join key is `(source.sha, rule_id, path, subject, lines)`. This repository's census manifest excludes `eval/**` and `vendor/`, and filters before sampling (c.2, c.3).

### `eval/patterns/references.json` and the profile (#872)

```json
{ "schema_version": "1", "references": [ { "label": "a path segment, at most 64 characters", "sha": "40-hex", "language": "go", "licence": "non-empty" } ] }
```

The clone's path is never committed: `--clone <label>=<path>` names it for each run. In the profile schema, `eval/internal/evalpatterns/profile.schema.json`, the only free-text fields are the label and the licence. `not_covered` items are `{shape, reason, count}`, where `reason` is one of `out_of_range`, `no_total_order`, `policy_declares_no_layers` and `policy_layers_match_no_package`.

### `eval/protocol/<protocol_id>.json`

```json
{
  "schema_version": "1",
  "protocol_id": "string", "committed": "YYYY-MM-DD",
  "design": "full | mvd",
  "hypotheses": ["H4a", "H4b", "H4c", "H2a", "H2b", "H2c", "H1a", "H1b", "H5"],
  "confirmatory_comparators": ["control", "rules_in_prompt", "generic_review"],
  "claim_rule": "intersection_union; Holm within H4 and within H2; d_bar > 0",
  "analysis": "stratified by language; df = T - 2",
  "alpha": 0.05, "h2_alpha_one_sided": 0.025, "h2_margin": -0.05, "h2_assumed_true_diff": 0, "min_effect": 0.15, "power_per_test": 0.9,
  "omega2": "max over comparators, 80% one-sided UCB at pilot df, floors applied after inflation",
  "omega2_floor": { "clean_success": 0.05, "completed": 0.02 },
  "unit": "structural cell", "cluster_key": "cell tuple",
  "T": 0, "instances_per_cell": 2, "K": 3, "impossible_K": 1,
  "stopping_rule": "fixed T, all cells run", "interim_looks": "none",
  "checkpoint_meanings": { "go-full": "claim row may be written", "go-fallback": "claim with MDE in wording", "stop": "no claim; PRD gate unjudged" },
  "outcome_meanings_ref": "decision 15.4a",
  "pilot_estimates_ref": "path + sha256",
  "graded_rules": [
    { "rule_id": "architecture.layer_violation", "languages": ["go", "ts"], "detector": "evaldetect layer graph", "comparison": "any forbidden edge" },
    { "rule_id": "complexity.cognitive_complexity", "languages": ["go"], "detector": "gocognit v1.3.0 (eval/go.mod tool)", "comparison": "-over 14 (> 14, i.e. >= 15)" }
  ],
  "delta": { "complexity.cognitive_complexity": 3 },
  "mutation": { "tools": { "gremlins": "string", "stryker": "string" }, "score_min": 0.8, "not_covered_max": 0.10, "scope": "import graph from api_surface" },
  "generator": { "version": "string", "held_out_seed_hash": "string" },
  "tuning_cells": ["tuple"], "held_out_cells_hash": "string",
  "arms": ["control", "coach", "rules_in_prompt", "generic_review"],
  "coach_commit": "string",
  "solver": { "name": "claude-code", "version": "string", "model": "string", "allowed_tools": ["string"] },
  "isolation": "docker_sandbox | docker", "images": {}, "runtime": "microvm | runsc | runc",
  "seeds": { "instance": 0, "order": 0 },
  "caps_ref": "eval/protocol/decisions/15.15.json",
  "exclusions": ["usage_limit_requeue", "impossible_variants_excluded_from_H4_H2"],
  "empty_cell_rule": "drop cell from every comparison; report count",
  "model_retirement_policy": "pinned model retired before T completes: invalid, re-register",
  "fallback": { "T": 0, "printed_mde": 0 },
  "deviations": []
}
```

### Census and #260 (decision 15.9)

#260 currently requires that "the recorded verdict carries the human's name and date". There are two options:
- **(a) Amend #260 so the census is fully automated** (*recommended*, given the autonomy requirement). The census would use only detector agreement and the `eval-history` association. Human review of the security rules becomes an **explicit, optional gap**. *Consequence:* no security-precision claim can be made until humans review them, and the PRD §12 security gate stays open.
- **(b) Keep a small, sampled `human_verdict` tier for the security rules only.** *Consequence:* the census needs a human step, so requirement (4) is not met for the census, and the census cannot be tied to the autonomous restamp.

**How the census runs.** `eval-census` runs `coach-eval census` in a pinned image (dependencies installed with `--ignore-scripts`; digest and compilers recorded). It scans with `coach codesignal --baseline --format json --no-interactive --fail-on-incomplete-coverage`, adding `--project-config` and, for TS, `--project-language typescript`. `analysis_status` is one of `measured`, `not_exercised`, `not_adjudicable_only`, `analysis_failed` or `out_of_scope`. Generated programs are excluded. Until the image lands (#832), the census runs Go repositories on the host:
- each from a fresh local clone, checked out at its SHA;
- with hooks, repository-supplied git configuration and global and system git configuration all disabled, so no git hook or filter, and no repository-supplied git configuration, runs (Coach still runs `go` over the committed files, with `GOTOOLCHAIN=local` and `-buildvcs=false`);
- and it refuses TypeScript repositories with exit 2, because Coach would resolve their compiler from the repository's own `node_modules`.

**The census record:**

```json
{ "record_id": "string", "target": { "corpus_id": "string", "sha": "string", "rule_id": "string", "path": "string", "subject": "string or null", "lines": [1, 2], "fingerprint": "string or null" },
  "measure": "detector_agreement | flagged_vs_unflagged_association | human_verdict | analysis_status",
  "value": "agree | disagree | <number> | true_positive | false_positive | unactionable_true_positive | not_adjudicable | measured | not_exercised | not_adjudicable_only | analysis_failed | out_of_scope",
  "source": { "kind": "detector | szz | review_comment | follow_up_fix | human | census", "ref": "string" },
  "rater": { "id": "string or null", "kind": "automated | human" },
  "date": "YYYY-MM-DD", "rationale": "string",
  "disposition": "retune | rescope | default_off | accept_with_caveat | null",
  "engineer_action": "acted | deferred | abandoned | no_usage_observed | null",
  "supersedes": "record_id or null" }
```

**Automated sources emit only `agree`, `disagree`, a number, or, for `analysis_status`, a status. Only `human_verdict` may emit TP or FP values.** Records are append-only (c.1, c.2). A census run writes one `analysis_status` record per repository and rule, from source `census` (ref: the `coach` sha256), targeting the repository root (`path` `.`, `lines` null), with the counts or the failure evidence as its rationale. A history run writes one `flagged_vs_unflagged_association` record per placed finding, from source `szz`: 1 with the first fix's SHA as `ref`, or 0 with `none`.

**Supersession.** A record is never rewritten. A new record supersedes the latest current record with the same `target`, `measure` and `source.kind`, and names it in `supersedes` (a record supersedes only a record already in the file before its run, so two records with one key in a single run do not supersede each other, and the second leaves the first current); the tool writes the field as part of the run (#895), and a record that already names what it supersedes is left as it is. `source.ref` is not part of the key, because it carries the value (a history's `none` or a fix SHA, a census's `coach` sha256), so a changed value is the re-measurement the link exists for. A reader that counts records counts only those no later record supersedes (`evalcensus.CurrentRecords`), because the file keeps every record and a count over all of them counts a re-measured target once per measurement. **Known limit:** the key has no `rater.id`, so two raters' `human_verdict` records for one target supersede each other until `Rater.ID` joins the key for that measure.

**The two-repository red test** (#260) checks three things: "0 of 12" is distinct from `not_exercised`, a failed analysis is `analysis_failed`, and a mutated threshold turns it red.

## 12. Mapping to #260

| #260 criterion (abridged) | What this design supplies |
| --- | --- |
| Per rule: repositories, SHAs, counts, verdicts | Census records, through `eval-census` |
| Zero findings → `not exercised` | `not_exercised`; `analysis_failed` is kept separate |
| Human, date, fingerprint, rationale | `human_verdict` records, under option (b), or the stated gap under (a) (decision 15.9) |
| Empty evidence → `not_adjudicable` | That value. The density rules now copy `finding.Evidence` (`rule_*_density.go:29`); this needs re-checking |
| Below threshold → a disposition | The `disposition` field |
| PRD updated | **Not supplied.** "Success Signals" no longer exists; the gate is now §12 (follow-up) |
| Security recall from seeded shapes | Per-shape recall and known misses for TOCTOU (c.6, c.7, f.1) |
| Pilot engineer action | The `engineer_action` field |
| A re-run reproduces | SHA, include/exclude, image digest, Coach hash (c.3, c.5) |
| An unpinnable repository is excluded | The SHA is required |
| False-green test | The two-repository red test; `eval-detection-selftest` |

Whether `architecture.*` joins #260 is an owner decision (15.9).

## 13. What each result can feed

**A confirmatory H4/H2 result is necessary but not sufficient** for the living evaluation's self-review gate, which also needs a restamp that runs the hook on a fresh real repository (PRD §12).

The claim row, "Agent self-review: controlled effect", gives:
- the wording, the model, the date, T, the design and any MDE;
- the effect, CI and pass or fail **for every confirmatory comparator**;
- the introduced-issue rate and token ratios;
- the scope, "generated programs below Coach's published thresholds before the change".

If Coach did not beat `rules_in_prompt`, the row says so. The row is filled only by a complete `eval-agent-study` that passes the token gate. A new model needs a new study (**our choice**).

| Result | Can feed | Cannot feed |
| --- | --- | --- |
| H4 with H2 | The claim row, scoped to the graded rules, generated programs and one model | The repeat-use test (PRD §4: "a hook's scans do not" count) |
| H3 | The pre-commit gate; per-shape recall and known misses | Precision; the security gate ("a recorded precision review on a named corpus", PRD §12) |
| H1, H5 | Exploratory evidence | Any release claim |
| `eval-history` | An exploratory flagged-vs-unflagged association on real code | Precision, recall, or any claim |
| Everything else | Integrity checks; drift, through saved results | Any claim |

## 13a. What an automated-only path can and cannot establish

**It can establish:**
- **conformance (H3);**
- **adherence (H4)** on generated programs, for the graded rules, if Coach beats `rules_in_prompt` and `generic_review`;
- **a first look at maintainability (H5);**
- **a flagged-vs-unflagged association** on real code.

**It cannot establish:**
- precision on real code;
- recall on real code;
- that experts value the findings;
- that following Coach improves real code in general.

No result here licenses "authority" language to customers.

**`eval-history`** (over the pinned census corpus) reports, per rule, a **flagged-vs-unflagged association**: a risk ratio with a CI, comparing code Coach flagged with size-matched unflagged code.
- **Outcome:** later involvement in a **bug-fixing** change, found with SZZ (f.8): link the fix commits, then blame the revision before the fix. SZZ finds fix-inducing changes; it does not identify refactors.
- **Secondary outcomes:**
  - a reviewer comment on a flagged line, versus on an unflagged line;
  - a merged follow-up change that touches the flagged code within N commits.
- **Data source:** offline only. Commit messages and issue references come from the local clone; no network or credential is used, and `--online` is refused with exit 2.

**How it runs.** The manifest pins each repository's `history_until` (U) beside its census SHA (S); the repository path must be a local directory.
- **Fixes:** the non-merge commits in S..U whose message matches a pinned regex. The run records the regex, its match rate, and per level the fixes that map to no unit. A fix that cannot be traced is counted unmapped and listed with git's reason, and never fails the repository. A window with no commits has a null match rate, with the reason.
- **Involvement:** a fix involves a unit when a line it changed, blamed at the fix's parent (`-w -C`), has the identity of one of the unit's lines blamed at S. Units are functions or whole files at S, by the rule's level.
- **Controls:** k = 3 per flagged unit, drawn seeded and without replacement from the unflagged units of the same size bin (floor(log2 lines)) and the same language. A bin too small is reported as a shortfall.
- **Ratio:** a Katz-log 95% interval. It is null, with a reason, when any cell is zero, including when every flagged unit or every control is involved. The counts are always reported.
- **Output:** every summary, record and ledger line states "association, not causation". Exits are in section 10.

SZZ's accuracy is "an open problem" (f.9). These results are association, never precision or recall.

Expert agreement, labelled recall and a powered maintainability result on real code remain follow-up issues that need humans.

## 14. Threats to validity

Grouped under b.7's four headings, plus generation and contamination.

**Construct**
- **H3 is conformance on seeded rows,** not precision. The team writes the rules, the generator, the operators and the in-house layer check (c.8).
- **The tasks are synthetic.** They are generated programs, small and regular, that start below Coach's published thresholds.
  - **The bases are deliberately simple.** The generated Go base is branch-free, returns no pointers, and uses value semantics, so every margin below Coach's thresholds holds by construction (#780). Complexity enters only through the operators (#782).
  - This limits realism, and the claim names it.
- **H4 covers only the graded rules:** layer violations (an in-house check that reads the same policy as the hook) and Go cognitive complexity (gocognit, external). Layer bypass is H3-only until #816. The disagreement between detectors and Coach, and `coach_base_fp`, are reported.
- **Injected is not natural** (c.6, c.7, f.1). Cognitive complexity is only partly validated (d.1, d.2), and gocognit's reading can differ from Coach's (f.10).
- **Circularity** (d.7) is controlled by an external grader for complexity, the generation and detection fence, and `rules_in_prompt`. The in-house layer check shares the policy with the hook by design. It checks the same contract, independently implemented.

**Generation**
- The generator's distribution sets what is tested. Results are reported per cell dimension and per language stratum.
- Hidden-suite adequacy rests on the mutation score and the coverage floor (f.2, f.12, f.13).

**Internal**
- The yoked `generic_review` arm, and intention to treat, guard against confounding.
- Everything is pinned, and a change between resumes invalidates the run.
- Faked grading is countered, but in-process targeting of the nonce channel remains possible. Flagged trials are reported separately.
- Under option (c), the credential is usable but unreadable, and the caps bound its use. Under option (b), it sits inside the agent's boundary (e.4).

**External**
- Generated programs are simpler than real code (c.5).
- There is one model and one solver (b.7).

**Conclusion**
- The unit is the structural cell, with language strata and t at T − 2 degrees of freedom (a.7, b.1).
- Holm correction applies within H4 and within H2, under the intersection-union rule.
- Sizing uses an inflated variance with floors. The H2 sensitivity and both headline Ts are stated.
- The protocol is preregistered and frozen by hash (b.6).

**Contamination**
- The programs are new, and the held-out seed is private (b.5, b.8).
- There are no hints in the task text (b.8).
- Tuning cells are never evidence.

## 15. Owner decisions, with recommendations

### Sequencing

*Recommend:* settle 15.15 first. Meanwhile, build the model-free slice: the generator, `eval-precommit`, `eval-replay`, the census and `eval-history`. That is what ships if the answer is "no".

### Decisions

Each decision gives the recommendation and, where it matters, what changes if the owner chooses otherwise ("Otherwise").

1. **Confirmatory task family.** *Recommend:* the hook on generated feature tasks. *Otherwise* (refactor-first): H1 becomes confirmatory, and the hook job stays unjudged.
2. **Model.** *Recommend:* pin the current flagship model, with a smaller model as an exploratory replication. *Otherwise:* a different model means a new study (section 13).
3. **Generator configuration.** *Recommend:* the seven-dimension cell space in section 5. *Otherwise:* with fewer dimensions, T may exceed the number of cells, and the study cannot be powered.
4. **Token gate (hard).** *Recommend:* tokens per G1 ∧ G2-passing trial at most 1.3 × `control`'s (**our choice**). *Otherwise:* report the token ratio without gating on it.

   **4a. What each outcome means for the roadmap** (decided in advance):
   - **Beats all three comparators:** Coach's content adds value beyond its rule list. Invest in the hook.
   - **Beats only `control`:** guidance helps, but not Coach's in particular. Invest in cheaper rule text and documentation.
   - **Beats none:** the hook does not help on these tasks. Reconsider the self-review job.
   - **Stop:** unjudged. The PRD gate stays open.
5. **Wording.** *Recommend:* "On generated Go and TypeScript programs below Coach's published thresholds before the change, with known structural traps (T programs, [model], [date]), X pp more feature changes were completed cleanly (95% CI a–b) when the agent ran Coach's self-review hook than with [strongest comparator beaten]. Cleanly means: hidden tests pass, no degenerate change, and no structural issue in the graded rules, as found by an in-house layer check that reads the same policy as the hook and by the external gocognit tool for Go cognitive complexity."
   - Add the mandatory clause "Coach did not outperform [unbeaten comparators]".
   - Report the introduced-issue rate beside it.
   - Never use "defects" without "structural", never "prevents", never "finds all".
6. **Caps.** These go in the decision record (section 9). *Recommend:* standing caps for `eval-precommit-agent`, and per-study caps.
7. **Thresholds marked our choice.** Ratified on 2026-11-26: 7a as the MVD, and 7b as the defaults below, which the pilot may revise.
   - **7a. Thresholds that move T or the claim:**
     - the design (full or MVD);
     - the confirmatory comparators;
     - the minimum effect;
     - the H2 margin and the assumed true difference;
     - power;
     - the upper-confidence-bound rule and the ω² floors;
     - the 1.3× gate;
     - K and the instance counts.

     *Recommend:* the **MVD** (section 5). *Otherwise* (the full design): about 17–26 weeks under the assumed weekly limit.
   - **7b. Everything else:**
     - the line band;
     - mutation score 0.8 and the 10% coverage floor;
     - Δ = 3;
     - the 50% rejection cap;
     - the trial caps;
     - refactor T = 24;
     - the model-retirement policy;
     - the token-leak stop;
     - the 600-trials-per-week planning assumption.
8. **`n/a` cells.** *Recommend:* look-alike `must_not_fire` rows.
9. **#260.** *Recommend:* option (a), amending #260 to be fully automated, with human security review as an optional, stated gap (section 11). *Otherwise* (b): a sampled human tier for the security rules, and the census sits outside the autonomous pipeline. *Also recommend:* include `architecture.*`, now that #253 is closed.
10. **Fixtures.** *Recommend:* cache them by input hash, not commit them. *Otherwise* (commit): the tracked-file count grows from 2,902, and the self-scan scope must be tested.
11. **CI.** No workflow invokes an `eval-*` task. The guard test enforces this.
12. **Checkpoint.** *Recommend:* allow go-fallback when its MDE is at most 25 points (**our choice**).
13. **Whose subscription.** *Recommend:* the study uses the owner's subscription. `eval-precommit` is model-free, and `eval-precommit-agent` uses the contributor's own subscription. *Otherwise:* contributors run only `eval-precommit`.
14. **Isolation and credential.** *Recommend:* option (c) once all 15 gates pass (**our choice**), and acceptance of a **dedicated** login or token. *Otherwise:* under the strict reading (only the existing host login), there are **no agentic runs**.
15. **Terms of use (blocking).** Consumer Terms §3.7 prohibits access "through automated or non-human means, whether through a bot, script, or otherwise", "Except when you are accessing our Services via an Anthropic API Key or where we otherwise explicitly permit it" (e.7). Anthropic documents `claude setup-token` "for CI and scripts" (e.2).
    - The owner **shall** confirm with Anthropic, and record the answer, model, caps and weekly figure in `eval/protocol/decisions/15.15.json`.
    - Revocation of `setup-token` is not stated in e.1.
    - *If the answer is "no":* the model-free slice ships, and the agentic hypotheses stay unjudged (section 2).
16. **Restamp tie.** *Recommend:* tie `eval-census` and `eval-history`, and their ledger entries, to the restamp duty in `AGENTS.md`. *Otherwise:* #260's history has gaps.
17. **TS hook language.** *Recommend:* a Coach change that selects the language from the policy, with a TS hook variant meanwhile. *Otherwise:* TS customers' hooks never give architecture findings.
18. **Pre-commit placement.** *Recommend:* developers run `ci-fast` plus `eval-precommit`, without adding it to the CI-mirrored list in `AGENTS.md`. The `AGENTS.md` wording change is not in this PR.

### Residuals (what is still not autonomous)

1. **Expert agreement, and labelled recall or precision, on real code.** These are follow-up issues and need humans.
2. **The one owner decision still open:** 15.15. The others were taken on 2026-11-26.
3. **During a study, which takes weeks to months:**
   - the host must stay up;
   - a reboot needs a restart (the supervisor mode recovers from a killed process, not from a reboot);
   - re-login is needed when the credential expires;
   - the owner's subscription is busy throughout.
4. **Online `eval-history`** uses the developer's GitHub login. Offline mode needs none.
5. **A red `eval-precommit`** is ordinary engineering.

### Follow-up issues (named, not opened here)

- **Prerequisite, before `eval-precommit` ships:** the test-only threshold override, absent from release builds.
- Nesting-depth grading, once a mapping to Coach's block counting is stated (ESLint `max-depth`, f.11, and a Go tool).
- A verified TS cognitive-complexity grader.
- General detectors for hidden input mutation, check-then-act and constructor coupling, in a fenced ring, validated on other operators' rows and on look-alikes.
- The generator and the operator library, including `feature_trap_complexity`.
- `eval-history`.
- Expert agreement and labelled recall on real code.
- Coach selecting the project language from the policy (decision 17).
- Retarget #260 to PRD §12, and amend #260 (decision 9).

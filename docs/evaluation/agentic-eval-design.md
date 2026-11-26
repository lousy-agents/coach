# Coach evaluation design: fixtures with ground truth, and agentic evals

Status: draft for review. This document is research and design only. It authorizes no fixture or harness code.

**The pipeline is fully autonomous** (owner requirement):
- no human-authored tasks;
- no human raters or adjudication;
- no human reading as a gate;
- no manual step between tasks;
- no LLM anywhere except the agent under test.

What remains for people is one-time owner decisions (section 15) and two residuals: the host must stay up, and the owner's subscription is busy while a study runs.

Conventions:
- Decisions marked **our choice** have no source behind them and need owner ratification.
- Every claim about outside practice cites a ledger row (section 1).
- A claim with no citation is about this repository, and names the file it was read from.
- Repository facts were read at `main` = `ed0ac13`.

Terms:
- **Program.** A small, layered Go or TS project made by the deterministic generator (section 6). It is identified by `program_id`, the hash of its generated tree.
- **Template.** One program plus its operator applications. **One template per program**, so the program is the statistical unit and the cluster key.
- **Instance.** A surface variant of a template.
- **Tuning programs and held-out programs.** These are disjoint sets. Tuning programs may be tuned on and are never evidence. Held-out programs are generated from a private seed and never run before the study.
- **Task family.** Feature tasks or refactor tasks.
- **Trial.** One agent run.
- **Solver.** The agent under test. Claude Code is the first one.
- **Graded issue.** An issue reported by a pinned, **general** detector that does not share code with the generator, and that has passed conformance on the seeded rows (section 4).

## 1. Source ledger index

Six research tracks, read from primary sources on 25–26 Nov 2026. Rows are cited as `a.N` to `f.N`. Tracks b and d number their sections; "d, proposed grader" means the closing section of the d ledger, and "a.6 finding N" means key finding N in row a.6.

| Track | File | What it covers | Rows |
| --- | --- | --- | --- |
| a | [`sources/a-agentic-eval-design.md`](sources/a-agentic-eval-design.md) | Agent-eval structure, graders, isolation, SWE-bench and Verified, ABC, error bars | a.1–a.7 |
| b | [`sources/b-experimental-rigour.md`](sources/b-experimental-rigour.md) | Statistics, estimators, preregistration, validity, contamination, sizing | b.1–b.8 |
| c | [`sources/c-ground-truth.md`](sources/c-ground-truth.md) | Ground truth for smell, static-analysis and architecture tools | c.1–c.9 |
| d | [`sources/d-refactoring-outcome.md`](sources/d-refactoring-outcome.md) | Measuring a refactoring's outcome | d.1–d.8 |
| e | [`sources/e-agent-credentials.md`](sources/e-agent-credentials.md) | Running the agent on a subscription login | e.1–e.7 |
| f | [`sources/f-autonomous-ground-truth.md`](sources/f-autonomous-ground-truth.md) | Ground truth by construction, automated validation, SZZ, pinned graders and mutation tools | f.1–f.13 |

### Where this design departs from the sources, and why

| Source practice | Departure | Reason |
| --- | --- | --- |
| Read transcripts before trusting numbers (a.1, b.4) | Automated pattern checks; reading is optional and never gates | Owner requirement for autonomy |
| Manually verify test cases (a.6 O.d.1) | Mutation score, baselines and oracle runs (f.2, section 6) | Same |
| Human screening, three annotators, the strictest label wins (a.5) | Automated validation; ground truth by construction (f.1) | Same |
| LLM judges calibrated against humans (a.1, a.2) | No LLM judge (f.5, f.6, f.7) | Same; and a judge would need a credential the owner excluded |
| Cluster-adjusted power (Miller, Appendix C, b.1; not read) | Program means as independent units, with T − 1 degrees of freedom | Simpler, and conservative under within-program correlation |

**Our-choice gaps:**
- trials per task (b.4; track b, "What was not obtainable");
- the mutation-score threshold and coverage floor (f.2, f.3).

## 2. What is being evaluated

1. **Detection (H3, deterministic).** Coach is run on each generated program, mutated and unmutated, with the same binary. The difference must equal the injected rows exactly. That is **conformance on seeded rows, not precision** (section 14).
2. **The self-review hook on feature tasks (confirmatory: H4, with H2 as a co-condition).** The hook in `docs/cli-codesignal.md` "## Agent self-review hook" runs `coach codesignal --worktree` at the end of each agent turn. Given a generated feature task with a structural trap, is an agent with the hook more likely to complete it cleanly? Cleanly means:
   - hidden tests pass;
   - no degenerate change;
   - no graded issue introduced.
3. **Refactor on request (secondary, H1).**
4. **Follow-on maintainability (secondary, H5).**
5. **Real code (exploratory).** Measured by the census (section 11) and by `eval-history` (section 13a).

**The tasks are synthetic, generated programs.** The agentic results speak to Coach's effect on generated layered programs with known traps, not to real code (section 14).

**If decision 15.15 is "no"**, these still ship:
- `eval-precommit`;
- the generator;
- `eval-detection`;
- the census;
- `eval-history`.

H4, H2, H1 and H5 then stay unjudged. The owner has excluded API keys, so the design offers no API-key variant.

## 3. Hypotheses and variables

### Arms

| Arm | Feature task family (confirmatory) | Refactor task family (secondary) |
| --- | --- | --- |
| `control` | Base prompt; no hook | Base prompt |
| `coach` | Base prompt; the documented Stop hook; `coach` on `PATH` | Base prompt plus the coach sentence; `coach` on `PATH` |
| `rules_in_prompt` | Base prompt plus the rule paragraph | Base prompt plus the rule paragraph |
| `generic_review` | Base prompt; a yoked Stop hook (section 7) | not run |

Held fixed across arms (b.7):
- the instance;
- the base prompt;
- the solver, the model and the CLI;
- tools and git rules;
- the caps;
- the isolation, the image and the runtime.

What the comparators isolate:
- `rules_in_prompt` separates "Coach helps" from "Coach shows the agent the grader".
- `generic_review` gets a generic extra turn exactly when the coach hook blocks on findings (**our choice**).

The analysis is **intention to treat**.

### Dependent variables

| Variable | Type | Task family | Role |
| --- | --- | --- | --- |
| `clean_success` = G1 ∧ G2 ∧ ¬`introduced_defect` | 0/1 | feature | **primary** (H4) |
| `completed` = G1 ∧ G2 | 0/1 | both | co-condition (H2); secondary in refactor |
| `introduced_defect`: at least one graded issue introduced | 0/1 | feature | secondary, reported beside the primary |
| `introduced_defect_count` | count | feature | secondary, beside `completed` |
| `refused` | 0/1 | feature | secondary |
| `exit_cap_or_timeout`, `hook_block_cause` | 0/1, category | both, hooked arms | reported per arm |
| `success` = G1 ∧ G2 ∧ every target predicate | 0/1 | refactor | secondary (H1) |
| hook findings shown, resolved and ignored; `in_loop_confirmed_share`; `unconfirmed_fix_share` | lists, 0–1 | feature (`coach`) | secondary |
| `detector_coach_disagreement`: graded issues Coach misses, plus Coach findings on graded rules with no detector match, on agent trees | counts | both | reported per rule and arm |
| `coach_extra_findings` | count | both | **reported, never graded** |
| `gaming`; `followon_g1` | 0/1 | both; feature | secondary (H5 for `followon_g1`) |
| G2 failures by check; metric deltas; reflexion counts | numbers | both | descriptive (d.1–d.3) |
| tokens, turns, wall-clock; notional cost (raw records only) | numbers | both | reported with every effect (b.5) |

### Hypotheses (all thresholds **our choice**)

**Confirmatory:**
- **H4a, H4b, H4c.** `coach` vs `control`, vs `rules_in_prompt`, and vs `generic_review`, on `clean_success`. Each is two-sided. Holm correction is applied across the three, at a family-wise α of 0.05.
- **H2a, H2b, H2c.** Non-inferiority on `completed`: one-sided, margin −0.05, with Holm across the three at 0.025.
- **Claim rule.** A claim against comparator x needs:
  - H4x rejected under Holm;
  - d̄ > 0;
  - H2x rejected under Holm.
- **Minimum effect:** 15 points.

**Secondary.** Holm correction is applied within each family:
- H1a and H1b: `coach` vs `control` and vs `rules_in_prompt`, on `success` (refactor);
- H5: `coach` vs `control` on `followon_g1`.

H3 is deterministic. Everything else is exploratory (b.6).

## 4. Graders and definitions

**General rules for grading:**
- All graders are code (a.1, a.2, f.7), and a later grader counts only once an earlier one passes (d.5).
- Graders and detectors ignore `scripts/coach-self-review.sh`.
- In the grading container, dependencies are restored read-only from the pristine image.

**G1 (behaviour).** Every test ID in the manifest reports `pass` on the nonce channel (section 7), with none failing or skipped.
- The IDs cover the hidden suite and the visible tests.
- Visible test files match their hashes (a.6 T.5, d.6).
- Feature suites include FAIL_TO_PASS tests (a.4).
- Agent-added test files are removed before grading.

**G2 (not degenerate; d.6, d.7).** G2 fails on any of these:
- an `api_surface` symbol is missing or unreachable;
- refactor only: non-test lines fall outside [0.7, 1.5] × the original (**our choice**; d, proposed grader, gives 0.7);
- a reflexion absence (d.3);
- a new suppression comment, a `TODO` stub, or a widening to `any`/`interface{}`;
- a literal branch;
- a policy edit;
- a dependency change outside the whitelist;
- a static check finding that non-test code uses inherited descriptors, `os.NewFile`, `/proc/self/fd` or `process.binding`, or writes to stdout or exits during initialisation.

**Target predicates (refactor).** The refactor oracle is the unmutated program. Each predicate is the inverse of a behaviour-preserving operator's violation (d.4). There are 3–8 per template (d, proposed grader).

### Graded detectors

Only rules with a **general** detector are graded. A general detector is not built from operators, is pinned, and lives in the fenced `evaldetect` ring (section 9).

| Graded `rule_id` | Language | Detector |
| --- | --- | --- |
| `architecture.layer_violation`, `architecture.layer_bypass` | Go, TS | A graph check: the import graph (`go list -deps -json`; TypeScript module resolution) against the program's layer policy. A missing required edge is a G2 absence |
| `complexity.cognitive_complexity` | Go | **gocognit**, pinned through mise (f.10). It reads its own Go version of the specification, which departs from d.1's suggestion to grade against Coach's implementation, because a shared grader can be optimised directly (d.7) |
| `complexity.max_nesting_depth` | TS | **ESLint `max-depth`**, pinned (f.11) |
| `complexity.max_nesting_depth` | Go | An independent nesting counter in `evaldetect`. No pinned external tool was verified, so a follow-up issue tracks one |

**Not graded until a general detector exists:**
- TS cognitive complexity: eslint-plugin-sonarjs could not be retrieved (f.11);
- `state.hidden_input_mutation`;
- `security.toctou_check_then_act`;
- `coupling.tight_constructor_init`;
- `complexity.branch_density`;
- `structure.constructor_density` and `structure.pointer_return_density`;
- `coupling.deep_relative_import`;
- `structure.react_component_orchestration_density`.

These rules are covered by H3 detection only. Their general detectors are follow-up issues, and each must be validated on rows from **other** operators and on look-alikes.

**Conformance on seeded rows.** On the detection fixtures, each detector must reproduce every seeded row of its rules and fire on no `must_not_fire` row. This is conformance against our own operators, not precision. `eval-agent-study` refuses until it holds.

### Definitions

- **Introduced.**
  - A key is in the detector's output on the final tree but not at `initial_commit_sha`.
  - A key is `(rule_id, path, subject)`, or `(from, to)` for an edge, with rename detection.
  - A key present in both counts only if it is above the threshold and rose by at least Δ: 3 for cognitive complexity, 1 for nesting (**our choice**).
- **Literal branch.** A new conditional on a literal that appears only in test inputs (d.6).
- **Line match.**
  - `rule_id` and `path` are equal, `subject` is equal where it is given, and the lines overlap.
  - The finding's lines run from `start_row + 1` to `end_row + 1` (`docs/cli-codesignal.md`).
  - `fingerprint` is informational (`pkg/codesignal/fingerprint.go:35`).
- **True negative.** A `must_not_fire` row. FPR = FP / (FP + TN) (c.5).
- **Discrimination.** A hit counts only if its twin stays silent (c.6).
- **Hook findings.**
  - *Shown*: in a hook message.
  - *Resolved*: absent at the end.
  - *Ignored*: still present.
  - *Confirmed*: a graded detector reports the same key.
- **Gaming** (impossible variants; d.6).
  - Refactor: (the impossible predicate is met ∧ G1 fails) ∨ (seeded Coach findings are gone ∧ (G1 ∨ G2 fails)).
  - Feature: G1 passes without the forbidden edge, or a test or policy file was edited.
- **Refused.**
  - The final tree is byte-identical to the initial commit.
  - The final message names a `rule_id`, a policy edge, or a rule by its prose description from `docs/cli-codesignal.md` (matched case-insensitively against a fixed list), so every arm is classified the same way.
  - A partial edit is graded normally.

## 5. Statistics and sizing

**Unit.** The program (the template, and the cluster key `program_id`; a.7, b.1).

**Estimator.**
- For each arm, average instances and trials into one rate per program, then take the paired differences d_t.
- The estimate is d̄, with SE = sd(d_t)/√T, using Student's t with T − 1 degrees of freedom (b.5; using it throughout is **our choice**).
- pass@k (b.2) and pass^k (b.3) are exploratory, and no claim rests on pass@k (b.5, d.5).
- Sampling settings stay at their defaults (a.7).
- Tokens and turns are reported with every effect (b.5).

### Sizing

Generated programs cost no human hours, so **the full design is the default**.

The formula is T ≈ (z_crit + z_power)² · ω² / δ², with 90% power per test (a.7, b.1).
- ω² = Var(d_t), from the pilot.
- It is sized on the one-sided 80% upper confidence bound, ω̂² · 11/χ²₀.₂₀,₁₁ ≈ 1.57 ω̂² (**our choice**).
- Floors: ω̂² ≥ 0.05 for `clean_success`, and ≥ 0.02 for `completed`, so a pilot estimate of 0 cannot size T to zero (**our choice**).

| Basis | H4 (α/3) | H2 (0.025/3, margin −0.05) | Headline T |
| --- | --- | --- | --- |
| Placeholders taken as already-inflated values (ω² = 0.3, ω²_completed = 0.05) | ≈ 180 | ≈ 270 | **≈ 270** |
| Placeholders taken as pilot point estimates, inflated by 1.57 | ≈ 283 | ≈ 424 | **≈ 424** |

**Totals per headline T** (our arithmetic). These assume 5 min a trial and 4 in parallel. They include impossible-variant, follow-on and refactor trials (24 programs, 504 trials).

| T | Trials | Tokens | Run hours | Five-hour windows (minimum) |
| --- | --- | --- | --- | --- |
| 270 | ≈ 10,220 | ≈ 10,220 τ | ≈ 213 | ≥ 43, plus weekly limits |
| 424 | ≈ 15,900 | ≈ 15,900 τ | ≈ 331 | ≥ 67, plus weekly limits |

**MDE** = (t_crit + t_power) · √(ω²/T), **at 80% power**:
- refactor task family, T = 24: ≈ 33 points unadjusted, ≈ 36 at the first Holm step;
- H5 at T = 270: ≈ 9 points.

### Pilot, automated checkpoint and preregistration

**The pilot.** `eval-agent-pilot` runs the tuning programs with all arms, plus an A/A run (the same `coach` twice). It writes `pilot_estimates` to `$XDG_STATE_HOME/coach-eval/pilot/<run_id>.json`. The estimates are:
- ω̂²;
- tokens and minutes per trial;
- trials per five-hour window;
- trials until the first weekly-limit message, if one occurs. Otherwise `EVAL_WEEKLY_TRIALS` is required.

The pilot is never evidence (b.6).

**The checkpoint.** `eval-agent-study` computes T from the newest pilot estimates and checks it against the caps:
- **go-full**, if T fits;
- **go-fallback** at the preregistered fallback T, with its MDE printed;
- **stop** (exit 6). Agent self-review then stays unjudged by this route (PRD §12: "Until the evaluation carries a claim for agent self-review, that job is not judged").

It prints the projected **calendar** duration, including weekly limits, before it starts.

**Preregistration.** Before the first scored trial, the study writes the protocol, freezes it by SHA-256 into `run.json`, and appends a pre-run ledger line (b.6). T is fixed, and there are no interim looks.

## 6. Generated programs and fixtures

**Ground truth is by construction** (f.1). It removes hand-built corpora, which f.1 reports took "about six months" for fourteen bugs. Injected violations are "synthetic but … realistic" (f.1), and c.6 warns that injected defects are found at different rates from natural ones.

### The generator (deterministic, seeded; no LLM)

For a given `(generator version, configuration, seed)`, the generator emits a small layered Go or TS project. The project contains:
- **Layers and policy.** 3–5 layers, each with 2–4 packages, and `.lousy-agents/coach.json` with at least one `required_layer`.
- **The hook.** `scripts/coach-self-review.sh`, from `docs/cli-codesignal.md` "### The check".
- **An `api_surface`.** Entry points in the top layer.
- **A schema-driven domain.** Entities, with fields drawn from a vocabulary pool.
- **A feature slot** of the form "expose field F of entity X, held in package B, through entry E in package A". By construction there is a clean route, through the layers the policy allows.
- **Tests and the oracle, by construction:**
  - black-box PASS_TO_PASS tests against `api_surface`;
  - the feature's FAIL_TO_PASS test;
  - the clean feature implementation, which is the oracle.
- **A follow-on slot.** A second feature request, with its own tests and oracle.

**Generation-time validation.** A generated program is **rejected** if any graded detector, or Coach, reports anything on it before mutation. Every program starts clean.

**Yield.** One configuration spans:
- layer counts {3, 4, 5};
- package counts {2, 3, 4};
- 4 entry kinds;
- feature paths of depth 2–5;
- 50 vocabularies.

That gives ≈ 3 × 3 × 4 × 4 × 50 = **7,200 structurally distinct programs per language** (our arithmetic). That is more than the ≈ 424 + 24 + 12 needed. Distinctness is checked by `program_id`, and the tuning and held-out sets are disjoint by construction.

### Operators (typed, deterministic)

| Operator | Task family | Injects | Ground truth emitted |
| --- | --- | --- | --- |
| `add_forbidden_edge` | refactor, detection | An import across a forbidden edge, moving one call through it (behaviour-preserving) | `edge_forbidden` row; inverse predicate |
| `add_bypass_edge` | refactor, detection | A direct call that skips `required_layer` (behaviour-preserving) | bypass row |
| `raise_cognitive`, `raise_nesting` | refactor, detection | Nesting and branches, with behaviour preserved, past the threshold by a set margin | `fire` row; a twin at the threshold − 1 |
| `split_check_then_act`, `mutate_hidden_input`, `tight_constructor` (TS) | detection only | The shapes from #205, out-parameter writes and constructor `new` | `fire` rows (H3 only) |
| `feature_trap` | feature | Makes the feature slot's field easier to reach across a forbidden edge than along the clean route (**behaviour-changing; feature only**) | trap row; FAIL_TO_PASS; clean oracle |
| `impossible_feature` | feature | Puts the field's runtime state only behind a forbidden edge | impossible variant; a duplicate-the-logic baseline that must fail G1 |
| near-miss and look-alike operators | detection | Shapes inside a threshold; look-alikes for `n/a` cells | `must_not_fire` rows |

**Behaviour-changing operators are confined to the feature task family.** Feature and follow-on requests are rendered from operator parameters by fixed text templates, with no hints (b.8).

### Automated validation (every instance)

1. The oracle passes G1 and G2, **in Docker** (a.6 T.9).
2. The do-nothing, delete and duplicate-the-logic baselines fail (a.6 finding 2).
3. The impossible variant cannot be passed by its baseline.
4. Every graded detector reproduces exactly the seeded rows, and fires on no `must_not_fire` row.
5. Expectations never come from Coach (#260 "False-green trap").
6. **The mutation score** of the hidden suite is at least 0.8, with a **coverage floor** of NOT COVERED ≤ 10% of valid mutants (both **our choice**).
   - The score uses f.2's definition, detected ÷ valid, over code reachable from `api_surface`.
   - The tools are Gremlins for Go and StrykerJS for TS, pinned through mise (f.12, f.13). For Gremlins, detected = KILLED + TIMED OUT, and NOT VIABLE is excluded.
   - The threshold is below 1, by our inference from equivalent mutants (f.3).
7. Every hidden test tagged to a site fails on that site's wrong-but-plausible variant (b.8).
8. **The unmodified instance passes every PASS_TO_PASS ID.**

**Regeneration and rejection.** A rejected instance is regenerated with a new recorded seed, up to 20 attempts (**our choice**). After that, the template is rejected with exit 8. The rejection rate per operator is reported (f.4).

### Detection fixtures and H3

- **Differential scoring.** H3 runs the same `coach` on the unmutated and the mutated tree of each fixture. **The difference must equal the injected rows exactly**: every `fire` row is present, and nothing else. Because every program starts clean, there is no pre-existing false positive to triage.
- **Row kinds.**
  - `known_miss` rows record shapes Coach does not cover, such as #205's four non-matching TOCTOU shapes. If one starts to fire, the suite fails with "promote the row".
  - `contested` rows (an out-parameter accumulator) are reported, with no assertion.
- **Coverage.** The rule × language coverage follows the CLI contract (`docs/product/prd.md:357` → `docs/cli-codesignal.md` "## Rule IDs"). Each `n/a` cell gets look-alike `must_not_fire` rows. The applicability table was read from the code (`pkg/semantics/*`, `pkg/codesignal/rule_*.go`) and is unchanged from earlier drafts.
- **Committed or cached.** Detection fixtures are **not committed**. They are cached under `$XDG_CACHE_HOME/coach-eval/fixtures/<input hash>`, keyed by the generator version, the operator set and the seed, so `eval-detection` regenerates only when its inputs change. The tracked-file count does not grow, and the self-scan never sees the fixtures (decision 15.10).

### Protection against "pass by deleting code"

| Shortcut | Caught by |
| --- | --- |
| Delete the function, or the import together with the feature | G2 API surface and absence (d.3); FAIL_TO_PASS (a.4) |
| Shrink code below a threshold | G2 line band (refactor); hidden suite |
| Edit or fake tests | Visible hashes; nonce channel; agent tests removed; static check; unprivileged runner (a.6, d.6) |
| Stub a dependency | Pristine dependencies; G2 whitelist |
| Special-case inputs | G2 literal branch (d.6) |
| Edit the policy, the hook or git state | G2; root-owned hashed hook; git-state checks |
| Do nothing, or do less | Predicates and FAIL_TO_PASS fail; `clean_success` needs G1 ∧ G2; caps and `refused` are reported |

## 7. Harness contract

### Solver seam

The agent is a solver: it takes an instance, a prompt, settings and caps, and returns an exit reason, a transcript and usage. `claude-code` is the first implementation (a.3). The `solver` block is solver-neutral.

### Agent sandbox and credentials

The agent uses a **subscription login**, with no API key (owner decision). There are two readings of "my local credentials" (decision 15.14).

| Option | Credential | Oracle | Verdict |
| --- | --- | --- | --- |
| (a) Host `claude` under the native Bash sandbox | On the host (e.1) | On the same host | **Rejected.** The sandbox "covers shell commands only", and it can read "Most of the machine, including credential files" (e.5) |
| (b) Docker with a `setup-token` token (e.1, e.2) | **Inside** the container | Outside | **Fallback.** A dev container cannot prevent exfiltration of "anything accessible inside the container" (e.4). The token lasts a year, and how to revoke it is undocumented (e.1) |
| (c) **Docker Sandboxes**: a microVM per trial, `/login` once (e.6) | On the host: "the sandbox sees only a sentinel value" (e.6) | Outside | **Recommended (our choice)**, subject to the pilot gates |

Under (c), the credential is **unreadable but usable through the proxy**, and the caps bound its use.

**Setup under (c):**
- **The workspace** is the instance, via `--clone` (e.6). The rest of the host filesystem is not reachable (e.6).
- **Each instance gets a fresh deterministic `git init`**, with one commit, no remotes, no alternates and no reflog. It is staged outside `EVAL_ORACLE_DIR`.
- **`CLAUDE_CONFIG_DIR`** stays at the directory `sbx` logged in to, and the arm's settings go there (e.1, e.2). The host's `~/.claude` is not inherited (e.6).
- **Network** is locked to the model API hosts, with the default wildcards removed (e.6).
- **Environment.** No API-key variable is set (e.1). `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` and `DISABLE_AUTOUPDATER=1` are set (e.2).
- **Never present:** the manifest, the hidden suite, the predicates, the oracle and the graders (a.6 T.5, d.6).

**Under (b)**, these apply:
- `CLAUDE_CONFIG_DIR=$HOME/.claude`;
- the token is injected at run time, never written to an image layer;
- egress is limited to the model API (e.4);
- an artefact scan quarantines the run on any hit.

**Pilot gates for (c).** Each gate has a marker test. If any gate fails, the run falls back to (b) only with the owner's agreement.
1. `claude -p` runs with this design's flags.
2. A custom template carries the toolchain, `coach` and `jq`.
3. One `/login` serves many sandboxes.
4. The login survives the arm's settings being added.
5. Read-only mounts work.
6. The agent runs as a non-root user, and the `coach` path is root-owned.
7. The final tree and `.git` can be extracted.
8. The host can check that the credential is present and when it expires.
9. `stream-json` output is live on the host.
10. The skills store is emptied or detached (e.6).
11. Billing goes to the subscription.
12. The default network wildcards are removed.
13. **The oracle is absent.** Markers planted in the oracle, the hidden suite, the predicates, the manifest and `$XDG_STATE_HOME/coach-eval` must not be found in the instance tree, in `git cat-file --batch-all-objects`, or by a full-filesystem search inside the sandbox. The same search run on the host must find them.
14. The TS nonce channel works under `node --test --test-isolation=process` (Node 24).

**Preflight (exit 2).** The run refuses if any of these is present:
- `ANTHROPIC_API_KEY`;
- `ANTHROPIC_AUTH_TOKEN`;
- an `apiKeyHelper`;
- an `sbx` API-key secret (e.6).

It also refuses if the credential is missing, expired, or **expires before the projected end of the run**. The refusal names the fixing command (`/login` in `sbx`, or `claude setup-token`) and never prints the credential.

### Grading sandbox and the nonce channel

- **The grading container** is fresh, with no network and no credentials. It holds the tree owned by an unprivileged `runner`, with agent tests removed, pristine dependencies, and only that trial's hidden suite, mounted read-only.
- **The nonce** arrives only on a file descriptor that the harness reads and closes before any test runs. The harness then writes one record per manifest test ID, each with the nonce, to a separate descriptor. In Go the harness is its own test binary; in TS it is a reporter under `node --test --test-isolation=process`. Visible tests use the same harness. G1 counts only this channel.
- **Residual risk:** in-process code could still target the channel deliberately. The G2 static check flags it, and a flagged G1-passing trial is reported separately.
- **The grade** is assembled on the host.

### Images and `coach` builds

- There is one pinned image (or, under option (c), template) per language, recorded by digest, with dependencies, `jq`, and the pinned graders and mutation tools.
- **`eval-agent-wiring`** builds `coach`, the hook and the rule paragraph from the **working tree**.
- **`eval-agent-study`** builds `coach` from a **recorded clean commit SHA** (`git status` must be clean at that SHA), so no release is needed.
- **Which arms get `coach`:**
  - `coach` arm: on `PATH`;
  - `generic_review`: at a root-owned path outside `PATH`;
  - otherwise absent. Any use of it in another arm is recorded as leakage.

### Holding the hook fixed

- **The hook script.** The `Stop` entry points at a **root-owned copy** of the hook script outside the workspace. Its hash is asserted on every invocation and at the end of the trial. Settings are read-only.
- **The yoked `generic_review` hook.** It prepends the root-owned `coach` inside the hook process only, runs the check silently, and sends the generic message only on a **findings** block. It uses the same `stop_hook_active` guard.
- **Hook health** (any failure is a harness-caused event). On the unmodified instance:
  - the hook exits 0;
  - `matches_head` is `true`;
  - there is no `worktree_scan_incomplete`;
  - on TS, a seeded forbidden edge is reported.

  TS programs use a TS variant of the hook until decision 15.17 lands.
- **Git state.**
  - Commits are denied, by `--disallowedTools "Bash(git commit:*)"` and a read-only `pre-commit`.
  - At the end of each trial, the harness asserts `HEAD` == `initial_commit_sha`, no new refs and no stash.
  - Grading uses the working tree.

### Integrity events and resumption

**Harness-caused** events abort the run, which stays resumable:
- the oracle is below 1, or a baseline is above 0;
- hook health fails, or the hook fires in the wrong arm;
- an oracle hash mismatch;
- a second `harness_error` on the same trial.

**Agent-caused** events are a git-state violation, a hook-hash mismatch, or (under (b)) a token in the artefacts. They are graded as assigned, with `clean_success` = `completed` = 0, and reported per arm. Under (b), the run also stops after that trial so the token can be rotated (**our choice**).

**Empty cells.** A program with no gradable trial in some arm is dropped from every comparison, and the count is reported.

**Usage limits.** On a usage-limit message, the study **waits in-process** until the window reopens, bounded by `EVAL_RUN_MAX_HOURS`. In-flight trials are re-queued, never graded as partial. Before each resume, it re-checks the credential, the model and the CLI. A change invalidates the run (exit 7). Exit 5 occurs only when `EVAL_RUN_MAX_HOURS` is reached or the process is killed. In both cases `-- --resume <run_id>` continues idempotently, keyed by `(instance_id, arm, epoch)`.

### Agent invocation (`claude-code`)

`claude -p` is run with these flags (checked against `claude --help`, Claude Code 2.1.332):
- `--model <pinned>`;
- `--output-format stream-json`;
- `--no-session-persistence`;
- `--setting-sources user`;
- `--strict-mcp-config` with an empty MCP config;
- the protocol's `--allowedTools`;
- `--disallowedTools "Bash(git commit:*)"`.

Two flags are never used:
- **`--bare`.** It skips "hooks (those defined in settings ...)" (`claude --help`), and in bare mode "Claude Code never reads OAuth credentials or the system keychain" (e.3).
- **`--max-budget-usd`.** The harness enforces token, turn and wall-clock caps from `stream-json` instead.

### Exact prompt text

**Feature task family, base (all arms):**

```text
{feature_request}

Work in this repository. Keep the existing tests passing and do not modify existing test files; you may add new test files. Do not commit. If the request cannot be done without breaking an existing rule of this repository, say which rule, make no changes, and stop. Do not ask questions: make reasonable choices and finish.
```

- `coach` adds the `Stop` entry from `docs/cli-codesignal.md` "## Agent self-review hook", pointed at the root-owned copy.
- `generic_review`'s yoked message is:

  ```text
  Before you stop, review your uncommitted changes for maintainability problems — complexity, coupling, layering and hidden side effects — and fix any you find, or explain why not, then stop.
  ```

- `rules_in_prompt` adds:

  ```text
  Before you finish, check your change against these structural rules and fix any violation you introduce:
  {one line per rule: rule_id — what it flags — threshold, generated from docs/cli-codesignal.md "## Rule IDs" and its threshold table at the coach version under test, plus the layer policy's forbidden and required edges}
  ```

**Refactor task family, base (all arms):**

```text
This repository has maintainability problems. Improve its internal structure without changing its behaviour. Keep the existing tests passing and do not modify existing test files. Do not commit. Do not ask questions: make reasonable choices and finish.
```

- `coach` adds:

  ```text
  The coach CLI is installed. `coach codesignal --baseline` reports structural findings for the whole repository; `coach codesignal --worktree` reports findings in your uncommitted changes.
  ```

- `rules_in_prompt` adds the rule paragraph.

**Follow-on trials** use the feature base prompt with the program's follow-on request, in a fresh sandbox, with no hook.

### Trial record (JSON, one per trial)

```json
{
  "schema_version": "1",
  "trial_id": "string", "run_id": "string", "protocol_sha256": "string",
  "task_family": "feature | refactor | followon",
  "program_id": "string", "template_id": "string", "instance_id": "string", "generator_seed_hash": "string",
  "initial_commit_sha": "string", "variant": "main | impossible",
  "arm": "control | coach | rules_in_prompt | generic_review | baseline_do_nothing | baseline_delete | baseline_duplicate | baseline_stub | oracle",
  "coach_build": { "kind": "worktree | commit", "sha": "string", "binary_sha256": "string" },
  "epoch": 1,
  "solver": { "name": "claude-code", "version": "string", "model": "string", "config_sha256": "string", "prompt_sha256": "string", "settings_sha256": "string", "hook_script_sha256": "string", "allowed_tools": ["string"] },
  "environment": { "isolation": "docker_sandbox | docker", "image_or_template_digest": "string", "runtime": "microvm | runsc | runc", "host_arch": "string" },
  "hooks": [{ "event": "Stop", "exit": 0, "block_cause": "none | findings | not_clean | both", "message_sha256": "string", "findings_shown": [{ "rule_id": "string", "path": "string", "subject": "string", "lines": [1, 2] }] }],
  "hook_findings": { "shown": ["key"], "resolved": ["key"], "ignored": ["key"], "confirmed": ["key"] },
  "integrity": { "hook_health": true, "git_state": true, "leakage": ["string"], "static_check_flags": ["string"], "pattern_checks": ["string"] },
  "caps": { "tokens": 0, "turns": 0, "wall_seconds": 0 },
  "usage": { "input_tokens": 0, "output_tokens": 0, "turns": 0, "notional_cost_usd": 0, "wall_seconds": 0 },
  "credential": { "mode": "sbx_host_held | setup_token_in_container", "artefact_scan": "clean | hit" },
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
- A `usage_limit` exit is re-queued, and listed.
- Leakage is reported.

**Transcript review** is automated pattern checks:
- test or policy edits;
- access to the nonce, the file descriptors or the reporter;
- `coach` use outside its arm;
- credential-shaped strings;
- use of the way-out sentence.

Hits are reported per arm. Human reading is optional and never gates.

### Harness acceptance tests

These are Ginkgo tests, red first, behind the build tag `evalharness`. They run through `mise run eval-harness-acceptance` with `-ginkgo.fail-on-empty`, and refuse to run without Docker.

- **Oracle and baselines.** The oracle scores 1. The do-nothing, delete, duplicate and stub baselines score 0. A weakened hidden suite lets the delete baseline pass. A suite below the mutation threshold or the coverage floor is rejected.
- **Forged results.** An agent `TestMain` that exits 0, a forged stdout `PASS`, and a forged TS frame each fail G1. A stubbed `expect` and a Go `replace` each fail G2.
- **Grading isolation.** Grading with network access fails. Grading uses no credential.
- **Credential probe.** Planted host markers (`~/.claude/.credentials.json`, and a Keychain item on macOS) are unreachable from a trial and reachable from the host. No marker appears in any artefact.
- **Settings isolation.** The developer's hooks, MCP servers and `CLAUDE.md` are not loaded in a trial.
- **Preflight.** The run refuses on an API-key variable or an `apiKeyHelper`. Billing goes to the subscription.
- **Oracle absence** (gate 13).
- **Hook and git tampering.** Deleting and replacing the hook is detected. `git commit --no-verify`, `commit-tree` and `update-ref` are each detected.
- **Detection boundaries.** `eval-detection` fails on a finding outside its fixture.
- **Generator.** A generated program on which any detector or Coach fires before mutation is rejected. The H3 difference equals the injected rows. Every graded detector reproduces its rows.
- **CI guard.** `.github/workflows/ci.yml` invokes no `eval-*` task and no eval-module leg. This guard is a test in `internal/validationtasks`, so it runs in CI.

## 8. Detection run (H3)

**Isolation.** `--baseline` scans "every tracked Go/TS/TSX file at HEAD" of the enclosing repository, and reads the policy from its top level (`docs/cli-codesignal.md` lines 57 and 131). So each fixture is handled in a fresh temporary directory:

1. **TS compiler.** For TS fixtures, `typescript@7.0.2` is copied from `js/semantics/node_modules` (`js/semantics/package.json`). The run refuses if it is absent, or if the compiler's origin is `global_mise`.
2. **Deterministic commit.** `git init` runs with `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1`, then one commit is made with a fixed author, committer and dates, using `--no-verify`. Its SHA must equal `expected_sha`.
3. **Scan.** The run executes

   ```sh
   coach codesignal --baseline --format json --no-interactive --fail-on-incomplete-coverage
   ```

   on the unmutated and the mutated commit. It adds `--project-config .lousy-agents/coach.json`, and for TS also `--project-language typescript`, which defaults to `go` (`cmd/coach/flags.go:62`).

**Scoring.** H3 passes when the mutated-minus-unmutated difference equals the `fire` rows, with no extra finding and no `known_miss` firing.

**Mutation red tests.** `mise run eval-detection-selftest` uses a test-only build override to lower a threshold or remove a shape, and each change must turn H3 red (#260 "False-green trap"). The override is a prerequisite issue and must be absent from release builds. The task refuses when the override is missing.

Coach's version and configuration are printed with every result (c.5).

## 9. Placement

**Code.**
- **One separate Go module**, `github.com/lousy-agents/coach/eval` (`eval/go.mod`), so its Docker client stays out of the main `go.mod`.
- **Packages and rings:**

  | Package | Ring |
  | --- | --- |
  | `eval/cmd/coach-eval`, `eval/internal/evalharness` | **tooling** |
  | `eval/internal/evalgen` (generator and operators) | **generation** |
  | `eval/internal/evaldetect` (graded detectors and grader adapters) | **detection** |
  | `eval/internal/evalgrade` | **grading** |
  | `eval/internal/evalhistory`, `eval/internal/evalcensus` | **real-code** |

- **Fences.** Generation and detection may not import each other in either direction. Neither may import grading, and no production ring may import any of these rings. Grading may import detection. Tooling may import all of them. Graders call the pinned `coach` binary, never `pkg/`.
- **Self-scan.** Coach's own scan treats `eval/` code like any other first-party code under its rules. No fixture or generated program is tracked, so the scan never sees seeded violations.
- **Planned edits** (owner approval, in the code PR):
  - `.lousy-agents/coach.json` and ADR-007's ring table gain the five rings and their rules;
  - `mise.toml` gains the tasks, and an eval-module leg inside `eval-precommit`, not inside CI-mirrored tasks.

**Committed data** (none of it Go or TS):
- `eval/protocol/` holds protocols and owner-decision records;
- `eval/census/` holds the corpus and the census records;
- `eval/ledger/runs.jsonl`.

The generator configuration lives in `eval/internal/evalgen`.

**Held-out material.** `eval-generate` writes held-out programs, hidden suites, predicates and oracles into `EVAL_ORACLE_DIR`, keyed by content hash. The **held-out generator seed** is stored only there, and the protocol records its hash (a.6 T.5, b.5, b.8). No repository or re-pin is needed. Tuning programs may be public.

## 10. Running it: local mise tasks

### Chain

There is no manual step. Each task refuses (exit 2) when its inputs are missing or stale.

- **Pre-commit:** `eval-precommit` = `eval-detection` + `eval-detection-selftest` + `eval-hook-health` + eval-module vet, tidy and vulncheck. It also runs `eval-agent-wiring` automatically when hook, rule, threshold or format paths changed and the 15.15 record exists. When those paths changed and the record is absent, it refuses with a message naming 15.15.
- **Study:** `eval-generate` → `eval-agent-pilot` → `eval-agent-study`.

**`ci.yml` shall not invoke any `eval-*` task or the eval-module legs.** A guard test enforces this (section 7).

### Supported hosts

| Host | Can run |
| --- | --- |
| Any OS with Go and the pinned Node | `eval-detection`, `eval-detection-selftest`, `eval-compare`, `eval-replay`, `eval-history` (offline mode) |
| Any OS with Docker | The above, plus `eval-generate`, `eval-hook-health`, `eval-precommit`, `eval-census`, `eval-harness-acceptance` |
| Any OS with Docker, if the owner chose option (b) | Plus `eval-agent*`, with the credential in the container |
| macOS 14+ on Apple silicon, Windows 11, or Ubuntu 24.04+ with KVM, with `sbx` and Docker (e.6) | Everything under option (c) |

### Tasks

| Task | What it runs | Wall time and tokens (**our estimates**) | Role |
| --- | --- | --- | --- |
| `eval-precommit` | The composite above | Minutes; plus about 10 min and about 3 τ when wiring runs | **The pre-commit gate** |
| `eval-detection` | H3, differential, with fixtures from the cache | Seconds to minutes; none | Gating, inside `eval-precommit` |
| `eval-detection-selftest` | Mutation red tests | Minutes; none | Gating, inside `eval-precommit` |
| `eval-hook-health` | Hook health, oracle and baselines on tuning programs; no model | Minutes; none | Gating, inside `eval-precommit` |
| `eval-agent-wiring` | 2–3 tuning programs, `coach` arm only; integrity checks | About 10 min; about 3 τ | Automatic inside `eval-precommit` for hook, rule, threshold and format changes |
| `eval-replay` | The new `coach` and hook over the pilot's saved final trees; diffs the messages and findings | Minutes; none | The model-free signal of effect drift |
| `eval-generate` | The generator, operators and full validation (section 6), with tuning and held-out programs written to `EVAL_ORACLE_DIR` | Tens of minutes, mostly mutation testing in Docker; none | Before the pilot and the study |
| `eval-agent-pilot` | All four arms plus A/A on the tuning programs; writes the estimates | ≈ 60 trials | Not evidence |
| `eval-agent-study` | The study, including the automated checkpoint | ≈ 10,220–15,900 trials; ≈ 213–331 h; ≥ 43–67 windows | **The only source of claims** |
| `eval-census` | #260's census | Minutes; none | Section 11 |
| `eval-history` | SZZ and review mining (section 13a) | Minutes to hours; none | Exploratory |
| `eval-harness-acceptance` | Harness tests | Minutes | Ginkgo pass |
| `eval-compare` | Two runs, or a trend | Seconds | Informational |

**`eval-agent-study` refuses** in any of these cases:
- the 15.15 record is absent;
- the commit is not clean;
- the graded set is not frozen, or a detector fails conformance;
- the computed T does not fit the caps;
- the credential will expire before the projected end.

When it runs:
- order is randomised within program blocks from the protocol seed;
- every arm runs the identical instances;
- an incomplete or invalid study cannot fill the claim row, and `eval-compare` refuses it.

### Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Pass, or `eval-compare` comparable |
| 1 | A gate failure, or a harness-caused integrity event (the run stays resumable) |
| 2 | A refusal: missing or stale input, an unreachable SHA, a missing run, or an incomplete study |
| 3 | `coach` returned non-zero on a fixture or a census repository |
| 4 | `eval-compare`: the runs are not comparable |
| 5 | `EVAL_RUN_MAX_HOURS` reached, or the process was killed; resumable |
| 6 | A cap was reached, or the checkpoint said stop |
| 7 | The study is invalid |
| 8 | `eval-generate`: a template was rejected after the regeneration cap |

### Inputs (a refusal names what is missing)

| Input | Used by |
| --- | --- |
| `eval/protocol/decisions/15.15.json` permitting subscription runs | `eval-agent*` |
| Docker; under (c), `sbx` (e.6) | `eval-generate`, `eval-agent*`, `eval-hook-health`, `eval-census`, `eval-harness-acceptance` |
| Node and the TS compiler at their pinned versions; gocognit, Gremlins and StrykerJS pinned through mise | `eval-generate`, `eval-detection`, `eval-census` |
| Pinned images or templates, by digest | `eval-agent*`, `eval-hook-health`, `eval-census` |
| `claude` at the recorded version, with a credential that is present and valid to the projected end (never printed) | `eval-agent*` |
| No API-key variable, no `apiKeyHelper`, no `sbx` API-key secret | `eval-agent*` |
| `EVAL_MAX_TRIALS`, `EVAL_MAX_TOKENS`, `EVAL_TRIAL_MAX_TOKENS`, `EVAL_TRIAL_MAX_TURNS`, `EVAL_TRIAL_TIMEOUT`, `EVAL_RUN_MAX_HOURS`; `EVAL_WEEKLY_TRIALS` when the pilot did not observe a weekly limit | `eval-agent*` |
| `EVAL_MODEL` (no default) | `eval-agent*` |
| `EVAL_ORACLE_DIR`, written by `eval-generate` | `eval-agent*`, `eval-hook-health` |
| The test-only threshold override | `eval-detection-selftest` |

### Results and comparison

**Raw output** goes to `$XDG_STATE_HOME/coach-eval/<run_id>/`, outside the worktree. `run.json` records:
- `protocol_sha256`;
- `coach_sha` and `worktree_dirty`, with hashes of the diff and of untracked content;
- the binary, hook and rule-paragraph hashes;
- the isolation, digests, runtime and architecture;
- the oracle hash;
- the solver;
- the parameters, seeds and caps;
- tokens, turns and hours;
- the state: `complete`, `paused`, `cap_reached` or `invalid`;
- the results.

**The ledger.** `eval/ledger/runs.jsonl` is append-only. It takes:
- the pre-run line of each study;
- study results;
- census, history and `eval-detection --record-baseline` runs.

**Comparing.** `mise run eval-compare -- <a> <b>` compares paired differences per program (a.7, b.1). A difference in solver, model, CLI, isolation, image, runtime, architecture, caps, prompt, oracle hash or parameters gives exit 4. `--trend <task>` tracks drift.

## 11. Manifest, protocol and census schemas

### `manifest.json`

```json
{
  "schema_version": "1",
  "id": "string",
  "kind": "detection_fixture | feature_template_instance | refactor_template_instance | census_repo",
  "language": ["go"],
  "program": { "program_id": "string", "generator_version": "string", "configuration": {}, "seed_hash": "string", "panel": "tuning | held_out" },
  "source": { "repo": "string (census only)", "sha": "string", "expected_sha": "string", "include": ["glob"], "exclude": ["glob"], "licence": "string (census only)" },
  "operators": [{ "name": "string", "params": {}, "seed": 0 }],
  "validation": { "mutation_score": 0, "not_covered_share": 0, "baselines_failed": true, "detectors_exact": true, "attempts": 1 },
  "sample": { "seed": 0, "frame": "all_findings | random_30 | not_sampled" },
  "layer_model": { "policy_path": ".lousy-agents/coach.json" },
  "rows": [
    {
      "row_id": "string", "rule_id": "string",
      "expect": "fire | must_not_fire | known_miss | contested",
      "row_kind": "seeded | twin | near_miss | lookalike_na | trap | edge_allowed | edge_forbidden | edge_required",
      "path": "string", "subject": "string or null", "lines": [12, 48],
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

- **Join key:** `(source.sha, rule_id, path, subject, lines)`.
- **This repository's census manifest** excludes `eval/**` and `vendor/`. The census filters before it samples (c.2, c.3).

### `eval/protocol/<protocol_id>.json`

```json
{
  "schema_version": "1",
  "protocol_id": "string", "committed": "YYYY-MM-DD",
  "hypotheses": ["H4a", "H4b", "H4c", "H2a", "H2b", "H2c", "H1a", "H1b", "H5"],
  "claim_rule": "intersection_union; Holm within H4 and within H2; d_bar > 0",
  "alpha": 0.05, "h2_alpha_one_sided": 0.025, "h2_margin": -0.05, "min_effect": 0.15, "power_per_test": 0.9,
  "omega2_floor": { "clean_success": 0.05, "completed": 0.02 }, "omega2_ucb": "80% one-sided",
  "unit": "program", "cluster_key": "program_id",
  "T": 0, "instances_per_program": 2, "K": 3, "impossible_K": 1,
  "stopping_rule": "fixed T, all cells run", "interim_looks": "none",
  "checkpoint": "go-full | go-fallback | stop",
  "pilot_estimates_ref": "$XDG_STATE_HOME path + sha256",
  "graded_rule_ids": ["architecture.layer_violation", "architecture.layer_bypass", "complexity.cognitive_complexity (Go)", "complexity.max_nesting_depth"],
  "detector_versions": { "gocognit": "string", "eslint": "string", "evaldetect": "string" },
  "mutation_tools": { "gremlins": "string", "stryker": "string" },
  "mutation_score_min": 0.8, "not_covered_max": 0.10,
  "delta": { "complexity.cognitive_complexity": 3, "complexity.max_nesting_depth": 1 },
  "generator": { "version": "string", "configuration": {}, "held_out_seed_hash": "string" },
  "tuning_program_ids": ["string"], "held_out_program_ids_hash": "string",
  "arms": ["control", "coach", "rules_in_prompt", "generic_review"],
  "coach_commit": "string",
  "solver": { "name": "claude-code", "version": "string", "model": "string", "allowed_tools": ["string"] },
  "isolation": "docker_sandbox | docker", "images": {}, "runtime": "microvm | runsc | runc",
  "seeds": { "instance": 0, "order": 0 },
  "caps": { "trial_tokens": 0, "trial_turns": 0, "trial_seconds": 0, "run_hours": 0 },
  "exclusions": ["usage_limit_requeue"], "empty_cell_rule": "drop program from every comparison; report count",
  "model_retirement_policy": "pinned model retired before T completes: the study is invalid and re-registered",
  "fallback": { "T": 0, "printed_mde": 0 },
  "deviations": []
}
```

### Census and #260 (decision 15.9)

#260 currently requires that "the recorded verdict carries the human's name and date". *Recommend:* amend #260 to accept **automated census measures**, with human labels as an optional extra that never gates. The automated measures are:
- detector agreement on Coach's findings, for rules with a general detector;
- the `eval-history` association;
- the mined review labels (section 13a).

The alternative is to keep the census outside the autonomous pipeline and remove its tie to the restamp.

- **Running it.** `eval-census` runs `coach-eval census` in a pinned image, with dependencies installed `--ignore-scripts`. It records the digest and the resolved compilers.
- **Scan.** `coach codesignal --baseline --format json --no-interactive --fail-on-incomplete-coverage`, plus `--project-config` where a policy exists, plus `--project-language typescript` for TS.
- **`analysis_status`** is one of:
  - `measured`;
  - `not_exercised` (with the gap);
  - `not_adjudicable_only`;
  - `analysis_failed` (with evidence);
  - `out_of_scope`.

  Generated programs never enter census figures.
- **Census record, schema v1:**

  ```json
  { "record_id": "string", "target": { "corpus_id": "string", "sha": "string", "rule_id": "string", "path": "string", "subject": "string or null", "lines": [1, 2], "fingerprint": "string or null" },
    "measure": "detector_agreement | history_association | review_label | human_verdict",
    "value": "true_positive | false_positive | unactionable_true_positive | not_adjudicable | agree | disagree | <number>",
    "source": { "kind": "detector | szz | review_comment | follow_up_fix | human", "ref": "string" },
    "rater": { "id": "string or null", "kind": "automated | human" },
    "date": "YYYY-MM-DD", "rationale": "string",
    "disposition": "retune | rescope | default_off | accept_with_caveat | null",
    "engineer_action": "acted | deferred | abandoned | no_usage_observed | null",
    "supersedes": "record_id or null" }
  ```

  Records are append-only (c.1, c.2). The `disposition` and `engineer_action` fields cover #260's remaining criteria.
- **Two-repository red test** (#260). It asserts:
  - "0 of 12" is distinct from `not_exercised`;
  - a failed analysis is reported as `analysis_failed`;
  - a mutated threshold turns the test red.

## 12. Mapping to #260

| #260 criterion (abridged) | What this design supplies |
| --- | --- |
| Per rule: repos, SHAs, counts, verdicts | Census records, through `eval-census` |
| Zero findings → `not exercised` | `not_exercised`; `analysis_failed` is separate |
| Human, date, fingerprint, rationale | `human_verdict` records, which are optional if #260 is amended (decision 15.9) |
| Empty evidence → `not_adjudicable` | That value. The density rules now copy `finding.Evidence` (`rule_*_density.go:29`), which needs a fresh check |
| Below threshold → disposition | The `disposition` field |
| PRD updated | **Not supplied.** "Success Signals" no longer exists in `docs/product/prd.md`; the gate is now §12 (follow-up) |
| Security recall from seeded shapes | Operator detection fixtures: per-shape recall and known misses for TOCTOU (c.6, c.7, f.1) |
| Pilot engineer action | The `engineer_action` field |
| Re-run reproduces | `source.sha`, include and exclude, image digest, Coach hash (c.3, c.5) |
| Unpinnable repository excluded | `sha` is required |
| False-green test | The two-repository red test; `eval-detection-selftest` |

Whether `architecture.*` joins #260 is an owner decision (15.9).

## 13. What each result can feed

**A confirmatory H4/H2 result is necessary but not sufficient** for the living evaluation's agent self-review gate, which stays unjudged until a restamp also runs the documented hook on a fresh real repository. PRD §12: "Until the evaluation carries a claim for agent self-review, that job is not judged".

The claim row, "Agent self-review: controlled effect", gives:
- the wording (decision 15.5), the model, the date, T and the design;
- the MDE if a fallback T was used;
- for **all three comparators**: the effect, the 95% CI, and pass/fail;
- the introduced-issue rate and the token ratios;
- **"generated programs"** in the scope.

If Coach did not beat `rules_in_prompt`, the row says so in words. Only a complete `eval-agent-study` can fill the row, and only when the token gate passes. A new model needs a new study (**our choice**).

| Result | Can feed | Cannot feed |
| --- | --- | --- |
| H4 with H2 | The claim row, scoped to graded rules, generated programs and one model | The repeat-use test (PRD §4: "a hook's scans do not" count) |
| H3 | The pre-commit gate; per-shape recall and known misses | Precision, or the security gate ("a recorded precision review on a named corpus", PRD §12) |
| H1, H5 | Exploratory evidence | Any release claim |
| `eval-history`, mined review labels | Exploratory association on real code | Precision, or any claim |
| All other tasks | Integrity checks, and drift through saved results | Any claim |

## 13a. What an automated-only path can and cannot establish

**It can establish:**
- conformance (H3);
- adherence (H4) in the graded rules on generated programs, if Coach beats **both** `rules_in_prompt` and `generic_review`;
- a first look at maintainability (H5);
- association on real code (below).

**It cannot establish:**
- precision on real code, without human labels (#260, f.9);
- that experts value the findings;
- that following Coach improves real code in general.

No result here licenses "authority" language to customers.

### `eval-history`: automated association on real code

`eval-history` runs over the pinned census corpus, in three tiers:

1. **SZZ (f.8).**
   - At historical commits, run Coach.
   - Link **bug-fix** commits, then blame the revision before each fix to find the fix-inducing changes. SZZ finds fix-inducing changes; it does not identify refactors.
   - Report, per rule, a risk ratio with a CI, comparing flagged code with unflagged code of similar size.
2. **Mined review labels**, a noisy secondary that serves as the middle tier. On public pull requests in the corpus, two naturally occurring signals are mined:
   - a reviewer comment that lands on a flagged line and asks for a structural change, which counts as agreement;
   - a merged follow-up fix that changes flagged code within N commits, which counts as a recall-side signal.

   Both are reported per rule, with their noise stated.
3. **Fix-linking source and network.**
   - **Offline mode (default):** commit-message keywords and issue references only. It needs no network and no credential, and it is noisier.
   - **Online mode (optional):** the GitHub REST API through `gh`, with the developer's existing GitHub login, read-only. It fetches issue labels, PR review comments and follow-up PRs.
   - All fetched data is snapshotted and hashed into the ledger.
   - SZZ's error rate is unknown, and evaluating it is "an open problem" (f.9). So every result here is association, never precision.

**Still needed for authority,** and not automatable here:
- expert agreement on real PRs;
- labelled recall on a real corpus;
- a powered maintainability result on real code.

These are follow-up issues. The mined review labels are the automated proxy for the first two.

## 14. Threats to validity

Grouped under b.7's headings, plus generation and contamination.

**Construct**
- **H3 is conformance on seeded rows, not precision.** The team writes the rules, the generator, the operators and the detectors (c.8).
- **The tasks are synthetic generated programs.** They are small, regular and layered, with schema-driven domains. That limits realism: real code is covered only by the census and `eval-history`, and claims name "generated programs".
- **H4 covers only the graded rules.** Coach's findings outside them are reported, and how far detectors and Coach disagree is reported per rule.
- **Injected is not natural** (c.6, c.7, f.1).
- **Cognitive complexity is only partly validated** (d.1, d.2). gocognit's reading of the specification can differ from Coach's (f.10). Both are reported.
- **Circularity** (d.7) is controlled by:
  - general pinned detectors;
  - a fence between the generator and the detectors;
  - `rules_in_prompt`.

  A detector could still share a misreading of the contract with an operator. That is mitigated by external tools for complexity and nesting, and by graph checks for layering.

**Generation**
- **The generator's distribution sets what is tested.** Yield and distinctness are checked by `program_id`, and per-configuration results are reported.
- **Hidden-suite adequacy** rests on the mutation score and the coverage floor (f.2, f.12, f.13). Equivalent mutants keep the score below 1 (inferred from f.3).

**Internal**
- **Arms.** The yoked `generic_review` isolates the extra turn. Intention to treat applies.
- **Pinning.** Everything is pinned, and a change between resumes invalidates the run.
- **Faked grading** is countered by the nonce channel and the other G1 and G2 defences. In-process targeting of the channel remains possible. The static check reports such trials separately, and nothing else covers them.
- **Credential.** Under (c), it is unreadable but usable, and the caps bound its use. Under (b), it sits inside the boundary (e.4).

**External**
- **Generated programs are simpler than real repositories** (c.5).
- **One model, one solver** (b.7).

**Conclusion**
- **Unit.** The program is the unit, with t at T − 1 degrees of freedom (a.7, b.1).
- **Multiplicity.** Holm correction applies within H4 and within H2, under the intersection-union rule.
- **Sizing.** T is sized on an upper confidence bound with floors. Both headline Ts are shown, and the MDE is at 80% power.
- **Preregistration.** The protocol is preregistered and frozen by hash (b.6).

**Contamination**
- Generated programs are new, and the held-out seed is private (b.5, b.8).
- No hints appear in the task text (b.8).
- The tuning programs are never evidence.

## 15. Owner decisions, with recommendations

### Sequencing

*Recommend:* settle 15.15 first. Meanwhile, build everything model-free: the generator, `eval-precommit`, `eval-replay`, the census and `eval-history`. That is what ships if the answer is "no" (section 2).

### Decisions

1. **Confirmatory task family.** *Recommend:* the hook on generated feature tasks.
2. **Model.** *Recommend:* pin the current flagship, with a smaller model as an exploratory replication.
3. **Generator configuration.** *Recommend:* the yield configuration in section 6. Held-out and tuning programs are disjoint.
4. **Token gate (hard).** *Recommend:* `coach`'s tokens per G1 ∧ G2-passing trial at most 1.3 × `control`'s (**our choice**), with the raw ratio printed.
5. **Wording.** *Recommend:* "On generated Go and TypeScript programs with known structural traps (T programs, [model], [date]), X pp more feature changes were completed cleanly (95% CI a–b) when the agent ran Coach's self-review hook than with [strongest comparator beaten]: hidden tests pass, no degenerate change, and no structural issue found by independent detectors in the graded rules (layering, Go cognitive complexity, nesting depth)."
   - Add the mandatory clause "Coach did not outperform [unbeaten comparators]" where it applies.
   - Report the introduced-issue rate beside it.
   - Never "defects" without "structural". Never "prevents". Never "finds all".
6. **Caps.** *Recommend:* standing caps for `eval-agent-wiring`, and per-study caps.
7. **Thresholds marked our choice.** *Recommend:* ratify these before preregistration:
   - the H2 margin;
   - the 15-point minimum effect;
   - power and the upper-confidence-bound rule;
   - the ω² floors;
   - the line band;
   - K and the instance counts;
   - the mutation score 0.8 and the coverage floor 10%;
   - the Δs;
   - the regeneration cap of 20;
   - the trial caps;
   - the 1.3× gate;
   - refactor T = 24;
   - the model-retirement policy;
   - the token-leak stop.
8. **`n/a` coverage cells.** *Recommend:* look-alike `must_not_fire` rows.
9. **#260.** *Recommend:* amend it to accept automated census measures, with human labels optional and never gating. If it is not amended, the census sits outside the autonomous pipeline and is not tied to the restamp. *Recommend:* also include `architecture.*`, now that #253 is closed.
10. **Fixtures.** *Recommend:* cache generated fixtures by input hash rather than committing them. The tracked-file count stays at 2,902 (from `git ls-files | wc -l`), and the self-scan never sees them.
11. **CI.** `ci.yml` shall invoke no `eval-*` task. A guard test enforces it.
12. **Checkpoint.** It is automated. *Recommend:* allow go-fallback when the fallback MDE is at most 25 points (**our choice**).
13. **Whose subscription.** *Recommend:* the study runs on the owner's subscription. Contributors run `eval-precommit`, and its wiring leg uses their own subscription only when they change hook, rule, threshold or format paths.
14. **Isolation and credential.** *Recommend:* option (c) after all 14 gates pass (**our choice**). The owner must accept a **dedicated** login or token. Under the strict reading, which allows only the existing host login, there are **no agentic runs**.
15. **Terms of use (blocking).** Consumer Terms §3.7 prohibits access "through automated or non-human means, whether through a bot, script, or otherwise", "Except when you are accessing our Services via an Anthropic API Key or where we otherwise explicitly permit it" (e.7). Anthropic documents `claude setup-token` "for CI and scripts" (e.2).
    - The owner **shall** confirm with Anthropic and record the answer in `eval/protocol/decisions/15.15.json`.
    - Every `eval-agent*` task refuses until that record exists.
    - How to revoke a `setup-token` token is undocumented (e.1).
16. **Restamp tie.** *Recommend:* tie `eval-census` and `eval-history`, with their ledger entries, to the restamp duty in `AGENTS.md`. If the owner says no, #260's history has gaps.
17. **Language selection in the TS hook.** *Recommend:* a Coach change that selects the language from the policy. Until then, use a TS variant of the hook.
18. **Where the pre-commit gate lives.** *Recommend:* developers run `ci-fast` plus `eval-precommit`. `eval-precommit` is not added to the CI-mirrored validation list in `AGENTS.md`, because CI shall not run eval tasks. The `AGENTS.md` wording change is not part of this PR.

### Exceptions: what still is not autonomous

1. **#260's human-verdict requirement**, unless the owner amends #260 (decision 9).
2. **Expert agreement and labelled recall on real code.** These are not automatable by definition. The mined review labels in section 13a are the noisy automated proxy.
3. **One-time owner decisions:** 15.15, 15.14 and threshold ratification.
4. **Residuals.** The host must stay up during a study, which can take days. The owner's subscription is busy for that time.
5. **Optional online `eval-history`.** It uses the developer's GitHub login. Offline mode needs none.
6. **A red `eval-precommit`** is ordinary engineering on a failing check, not a pipeline step.

### Follow-up issues (named, not opened here)

- The test-only threshold override, which must be absent from release builds.
- A pinned external Go nesting detector.
- A verified TS cognitive-complexity grader (eslint-plugin-sonarjs could not be retrieved, f.11).
- General detectors for hidden input mutation, check-then-act and constructor coupling. They must sit in a fenced ring and be validated on other operators' rows and on look-alikes.
- The generator and the operator library.
- `eval-history`, including the mined review labels.
- Expert agreement on real PRs, and labelled recall.
- Coach selecting the project language from the policy (decision 17).
- Retargeting #260 to PRD §12, and amending #260 (decision 9).

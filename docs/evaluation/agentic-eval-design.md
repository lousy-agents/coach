# Coach evaluation design: fixtures with ground truth, and agentic evals

Status: draft for review. This document is research and design only. It authorizes no fixture or harness code.

**The pipeline is fully autonomous** (owner requirement). No step needs human hours: no hand-authored templates, no human raters, no human adjudication, and no human reading as a gate. Only one-time owner decisions remain (section 15). Every remaining human step is listed as an exception, with a recommendation for automating it (section 15, "Exceptions").

Conventions:
- Decisions marked **our choice** have no source behind them and need owner ratification.
- Every claim about outside practice cites a ledger row (section 1).
- A claim with no citation is about this repository, and names the file it was read from.
- Repository facts were read at `main` = `ed0ac13`.

Terms:
- A **seed program** is a clean program that violations are injected into.
- A **template** is a seed program plus a set of operator applications (section 6). It is the statistical unit.
- An **instance** is one generated copy of a template.
- The **tuning panel** is 12 templates used for development. Tuning on them is allowed, and they are never evidence.
- **Held-out templates** are confirmatory, and are never tuned on or run before the study.
- A **task family** is either the feature tasks or the refactor tasks.
- A **trial** is one agent run.
- The **solver** is the agent under test, and Claude Code is the first one (section 7).
- A **graded issue** is an issue reported by an independent, non-Coach detector whose precision has been established on the seeded fixtures (section 4).

## 1. Source ledger index

Six research tracks, read from primary sources on 25–26 Nov 2026. Rows are cited as `a.N` to `f.N`. Tracks b and d number their sections `1.`, `2.` and so on. "d, proposed grader" means the closing section of the track d ledger. "a.6 finding N" means key finding N in row a.6.

| Track | File | What it covers | Rows |
| --- | --- | --- | --- |
| a | [`sources/a-agentic-eval-design.md`](sources/a-agentic-eval-design.md) | Agent-eval structure, graders, isolation, SWE-bench and Verified, ABC, error bars | a.1–a.7 |
| b | [`sources/b-experimental-rigour.md`](sources/b-experimental-rigour.md) | Statistics, estimators, preregistration, validity, contamination, sizing | b.1–b.8 |
| c | [`sources/c-ground-truth.md`](sources/c-ground-truth.md) | Ground truth for smell, static-analysis and architecture tools | c.1–c.9 |
| d | [`sources/d-refactoring-outcome.md`](sources/d-refactoring-outcome.md) | Measuring a refactoring's outcome | d.1–d.8 |
| e | [`sources/e-agent-credentials.md`](sources/e-agent-credentials.md) | Running the agent on a subscription login | e.1–e.7 |
| f | [`sources/f-autonomous-ground-truth.md`](sources/f-autonomous-ground-truth.md) | Ground truth by construction, automated validation, generator bias, SZZ | f.1–f.9 |

Gaps the ledgers record, which this design fills with **our choice**:

- **Trials per task.** No source recommends a number (b.4; track b, "What was not obtainable").
- **Cluster adjustment.** Miller's cluster-adjusted formulas (Appendix C, b.1) were not read. This design departs from them: template means are treated as independent units with T − 1 degrees of freedom (section 5).
- **Mutation-score threshold.** No source gives one for accepting a hidden suite (f.2, f.3).

## 2. What is being evaluated

1. **Detection (H3, deterministic).** Does `coach codesignal` report exactly the violations injected into a fixture, and nothing on its near-misses and clean controls? This is **conformance to the team's own specification, not precision** (section 14).
2. **The self-review hook on feature tasks (confirmatory: H4, with H2 as a co-condition).** The hook in `docs/cli-codesignal.md` "## Agent self-review hook" runs `coach codesignal --worktree` when the agent finishes a turn. Given a feature request with known structural traps, is an agent with the hook more likely to complete the feature cleanly? "Cleanly" means:
   - hidden tests pass;
   - no degenerate change;
   - no **graded issue** introduced.
3. **Refactor on request (secondary, H1).**
4. **Follow-on maintainability (secondary, H5).** Is a coach-arm tree easier for a fresh agent to extend?
5. **History association (exploratory).** On real repositories, is code that Coach flags later implicated in fixes? This is mined automatically with SZZ (f.8, section 13a).

## 3. Hypotheses and variables

### Arms

| Arm | Feature task family (confirmatory) | Refactor task family (secondary) |
| --- | --- | --- |
| `control` | Base prompt; no hook | Base prompt |
| `coach` | Base prompt; the documented Stop hook; `coach` on `PATH` | Base prompt plus the coach sentence; `coach` on `PATH` |
| `rules_in_prompt` | Base prompt plus the rule paragraph; no hook | Base prompt plus the rule paragraph |
| `generic_review` | Base prompt; a yoked Stop hook (section 7) | not run |

Held fixed across arms (b.7):
- the instance tree, including `scripts/coach-self-review.sh` and `.lousy-agents/coach.json`;
- the base prompt;
- the solver, the model and the CLI;
- tool permissions and git-state rules;
- the caps;
- the isolation, the image and the runtime.

What the comparators separate:
- `rules_in_prompt` separates "Coach helps" from "Coach shows the agent the grader".
- `generic_review` gets a generic extra turn exactly when the coach hook blocks on findings, so H4c isolates that turn's content (**our choice**).

The analysis is **intention to treat**: every trial is graded as assigned.

### Dependent variables

| Variable | Type | Task family | Role |
| --- | --- | --- | --- |
| `clean_success` = G1 ∧ G2 ∧ ¬`introduced_defect` | 0/1 | feature | **primary** (H4) |
| `completed` = G1 ∧ G2 | 0/1 | both | co-condition (H2) in feature; secondary in refactor |
| `introduced_defect`: at least one graded issue introduced (section 4) | 0/1 | feature | secondary; always reported beside the primary |
| `introduced_defect_count` | count | feature | secondary; reported beside `completed` |
| `refused` | 0/1 | feature | secondary |
| `exit_cap_or_timeout` | 0/1 | both | reported per arm |
| `hook_block_cause`: `none`, `findings`, `not_clean` or `both` | category | feature (hooked arms) | reported per arm |
| `success` = G1 ∧ G2 ∧ every target predicate | 0/1 | refactor | secondary (H1) |
| `predicate_score` | 0–1 | refactor | secondary |
| hook findings shown, resolved and ignored | lists | feature (`coach`) | secondary |
| `in_loop_confirmed_share`: the share of shown findings that an independent detector also reports | 0–1 | feature (`coach`) | secondary. This is a proxy, not precision |
| `unconfirmed_fix_share`: the share of resolved findings that no independent detector reports | 0–1 | feature (`coach`) | secondary |
| `gaming` on impossible variants | 0/1 | both | secondary |
| `followon_g1` | 0/1 | feature | secondary (H5) |
| G2 failures by check | counts | both | reported by check and arm |
| `coach_extra_findings`: Coach findings with no matching graded issue | count | both | **reported, never graded** |
| metric deltas; `divergences_after`, `absences_after` | numbers | both | descriptive (d.1–d.3) |
| tokens, turns, wall-clock; the CLI's notional cost (raw records only) | numbers | both | reported with every effect (b.5) |

### Hypotheses

**Confirmatory** (feature task family; all **our choice**):
- **H4a, H4b, H4c.** `coach` versus `control`, `rules_in_prompt` and `generic_review`, on `clean_success`. Each is two-sided, with H0: d̄ = 0. Holm correction across the three, at a family-wise α of 0.05.
- **H2a, H2b, H2c.** The non-inferiority of `coach` on `completed`. Each is one-sided with margin −0.05. Holm correction across the three, at a family-wise α of 0.025.
- **Claim rule (intersection-union).** A claim against x needs all of: H4x rejected under Holm, d̄ > 0, and H2x rejected under Holm.
- **Minimum effect:** 15 points.

**Secondary.** Each family is Holm-corrected within itself.
- H1a: `coach` vs `control` on `success` (refactor).
- H1b: `coach` vs `rules_in_prompt` on `success` (refactor).
- H5: `coach` vs `control` on `followon_g1`, two-sided.

**H3** is deterministic (section 8). Everything else is exploratory (b.6).

## 4. Graders and definitions

All graders are code (a.1, a.2, f.7). A later grader counts only once an earlier one has passed (d.5). Every grader and detector ignores `scripts/coach-self-review.sh`. In the grading container, dependencies are restored read-only from the pristine image.

### G1: behaviour preserved

G1 passes only when every test ID listed in the template's manifest reports `pass` on the nonce channel (section 7).
- The IDs cover the hidden suite and the visible tests, and none may report `fail` or `skip`.
- Visible test files match their SHA-256 (a.6 T.5, d.6).
- Feature hidden suites include FAIL_TO_PASS tests (a.4).
- Agent-added test files are removed before grading.

### G2: not degenerate

G2 (d.6, d.7) fails on any of:
- an `api_surface` symbol is missing or unreachable;
- **refactor only:** non-test lines fall outside [0.7, 1.5] × the original (**our choice**; d, proposed grader, gives 0.7);
- a reflexion absence (d.3);
- a new suppression comment, a `TODO` stub, or an `any`/`interface{}` widening on the API surface;
- a literal branch;
- `.lousy-agents/coach.json` was modified;
- a dependency change that the manifest does not whitelist (manifests, lockfiles, `node_modules/`, `vendor/`, a `go.mod` `replace`);
- a static check finds that non-test code uses inherited descriptors, `os.NewFile`, `/proc/self/fd` or `process.binding`, or writes to stdout or exits the process during initialisation.

### Target predicates (refactor)

The refactor oracle is the **unmutated seed program**, so each predicate is the inverse of an operator's injected violation (section 6; d.4 for structure-level subtests). There are 3–8 per template (d, proposed grader), and at least one is evaluated without Coach.

### Independent detectors (the graded set)

| Graded `rule_id` | Detector |
| --- | --- |
| `architecture.layer_violation`, `architecture.layer_bypass` | An import graph from `go list -deps -json` or the TypeScript compiler's module resolution, checked against the layer model. A bypass edge counts as introduced. A missing required edge is a G2 absence |
| `complexity.cognitive_complexity`, `complexity.max_nesting_depth` | An independent implementation. This departs from d.1, which grades against Coach's own implementation, because a shared grader can be optimised directly (d.7) |
| `state.hidden_input_mutation`, `security.toctou_check_then_act`, `coupling.tight_constructor_init` (TS) | AST predicates generated by the same operators that inject the violation (section 6), written from the contract, not from Coach's code (c.8) |

These rules are not graded, because none has an independent detector: `complexity.branch_density`, `structure.constructor_density`, `structure.pointer_return_density`, `coupling.deep_relative_import`, and `structure.react_component_orchestration_density`.

**Detector precision is established automatically.** On the detection fixtures, a detector must reproduce **every** seeded row of its rules and fire on **no** `must_not_fire` row. Until it does, `eval-agent-study` refuses. No human labels any issue.

### Definitions

- **Introduced.** A key is introduced if it is in a detector's output on the final tree but not on `initial_commit_sha`.
  - A key is `(rule_id, path, subject)`, or the `(from, to)` pair for an edge. Paths go through rename detection (**our choice**).
  - A key present in both counts only if it is above the threshold and its metric rose by at least Δ: 3 for cognitive complexity, 1 for nesting depth (**our choice**).
- **Literal branch.** A new conditional on a literal that appears only in test inputs (d.6).
- **Line match.** A finding matches a manifest row when `rule_id` and `path` are equal, `subject` is equal where given, and the lines overlap.
  - A finding's lines are `location.start_row + 1` to `location.end_row + 1` (`docs/cli-codesignal.md`).
  - A file-level finding matches a row with `lines: null`.
  - `fingerprint` is informational (`pkg/codesignal/fingerprint.go:35`).
- **True negatives and FPR.** The TN unit is a `must_not_fire` row. FPR = FP / (FP + TN) (c.5).
- **Discrimination.** A hit counts only if its twin stays silent (c.6).
- **Hook findings.**
  - *Shown*: in any hook message. *Resolved*: absent from the final post-hoc Coach output. *Ignored*: still present.
  - *Confirmed*: an independent detector reports the same key.
- **Gaming**, on impossible variants (d.6):
  - refactor: (the impossible predicate is met ∧ G1 fails) ∨ (Coach findings at seeded sites gone ∧ (G1 ∨ G2 fails));
  - feature: G1 passes without the forbidden edge, or a test or policy file was edited.
- **Refused.** The final tree is byte-identical to the initial commit, and the final message names a `rule_id` or a policy edge. A partial edit is graded normally.

### LLM judge

An LLM judge may produce only a **labelled secondary** measure, never a gate or a claim input (f.5, f.6, f.7). If one is used:
- it must be from a different model family from the agent (a.2, f.5);
- it judges one dimension with a closed rubric (a.1, a.2);
- its settings are pinned (a.3).

## 5. Statistics and sizing

**Unit:** the template (a.7, b.1). Per arm, average instances and trials into one rate per template, then take paired differences d_t. The estimate is d̄ with SE = sd(d_t)/√T. Student's t is used throughout, with T − 1 degrees of freedom (b.5; using it throughout is **our choice**).

Further:
- Per-arm rates are mean ± SE (a.7).
- pass@k (b.2) and pass^k (b.3) are exploratory. a.1 recommends pass^k "for agents where consistency is essential". No claim rests on pass@k (b.5, d.5).
- Sampling settings stay at their defaults (a.7).
- Tokens and turns are reported with every effect (b.5).

### Sizing

Generated templates cost no human hours, so **the full design is the default**.

Formula, Miller's paired formula on template means (a.7, b.1):

T ≈ (z_crit + z_power)² · ω² / δ²

- ω² = Var(d_t), taken from the pilot as the one-sided 80% upper confidence bound, ω̂² · 11/χ²₀.₂₀,₁₁ ≈ 1.57 ω̂² (**our choice**).
- Each test is sized at 90% power, so joint power is at least about 80% by the union bound (**our choice**).
- Planning placeholders: ω² = 0.3 (from b), and ω²_completed = 0.05 (**our choice**).

With three comparators, δ = 15 points, and an H2 margin of −0.05:
- H4 binds at α/3 and needs T ≈ 180.
- **H2 binds at 0.025/3 and needs T ≈ 270.**

The H2 margin is the strongest lever (decision 15.7).

**Totals for the full design (T ≈ 270).** All figures are placeholders from our arithmetic, assuming 5 min a trial and 4 trials in parallel:
- **≈ 10,220 trials**: 6,480 feature trials, 1,080 impossible-variant trials, 2,160 follow-on trials, and 504 in the refactor task family;
- **≈ 10,220 τ tokens**, where τ is the pilot's tokens per trial;
- **≈ 213 run hours**, which is **at least 43 five-hour usage windows**, plus the weekly limits (e.7);
- **no human hours.**

**MDE at a given T:** (t_crit,T−1 + t_power,T−1) · √(ω²/T). Examples:
- `eval-agent-panel` (T = 12): ≈ 49 points.
- Refactor task family (T = 24): ≈ 33 points unadjusted, ≈ 36 at the first Holm step.
- H5 at T = 270: ≈ 9 points.

### Pilot and the automated checkpoint

`eval-agent-pilot` runs the tuning panel with all arms, plus an A/A run (the same `coach` twice). It writes `pilot_estimates` to the protocol, and is never evidence (b.6). The estimates are:
- ω̂² and ω̂²_completed;
- tokens, minutes and **trials per usage window**;
- the A/A flip rate.

**The checkpoint is automated.** `eval-agent-study` computes T from the estimates and compares the trials, tokens and hours with the configured caps. It then acts on whichever case applies:
- **go-full**, if T fits;
- **go-fallback**, if only a preregistered fallback T fits. Its MDE is printed in the protocol and the claim;
- **stop** (exit 6), if neither fits. Agent self-review then stays unjudged by this route (PRD §12: "Until the evaluation carries a claim for agent self-review, that job is not judged").

**Preregistration.** The protocol is committed at `eval/protocol/<protocol_id>.json` (b.6, section 11). T is fixed and there are no interim looks. Deviations are recorded, and a negative result is published.

## 6. Fixtures by construction

**Ground truth comes from construction, not authoring** (f.1).
1. Start from a **clean seed program**.
2. Apply **deterministic, typed mutation operators**. Each operator injects one graded violation and records its exact ground truth: `rule_id`, `path`, `subject`, lines, and the inverse predicate.
3. Accept the result only after automated validation (below).

This removes hand-built corpora, which f.1 reports took "about six months" for fourteen bugs. Injected violations are "synthetic but … realistic" (f.1), and c.6 warns that injected defects are found at different rates from natural ones (section 14).

### Seed programs

*Recommend:* **curated, permissively licensed small programs**, pinned by SHA (Go and TypeScript, 300–1500 non-test lines, with their own test suites), plus deterministic surface variation (**our choice**).
- **Why not generated programs.** LLM-generated programs are an optional later source. If used, the generator must be from a **different model family** from the agent, or at least a separately pinned configuration, and it must never see Coach's output (f.5, f.6).
- **Contamination.** A public seed program may be in training data (b.8). The injected violations, renames and feature requests make each instance novel. The residual risk is in section 14.
- **Licences.** The licence of every seed is recorded, and only permissive licences are accepted (**our choice**).

### Operators

Each operator is typed, deterministic for a given seed, and emits its manifest rows.

| Operator | Injects | Ground truth emitted |
| --- | --- | --- |
| `add_forbidden_edge` | An import from a lower layer into a forbidden one, moving one call through it | an `edge_forbidden` row; the inverse predicate "no edge from A to B" |
| `add_bypass_edge` | A direct call that skips the `required_layer` | a bypass row |
| `raise_cognitive` / `raise_nesting` | Nesting and branches added around existing logic until the contract's threshold is crossed by a set margin, with behaviour preserved | a `fire` row with the target metric; a twin at threshold − 1 |
| `split_check_then_act` | An `os.Stat`/`existsSync` check separated from its act | a `fire` row, using a #205 shape |
| `mutate_hidden_input` | A write through a parameter that the caller observes | a `fire` row |
| `tight_constructor` (TS) | `this.x = new Y()` in a constructor | a `fire` row |
| `feature_trap` | A feature request whose natural implementation needs data behind a forbidden edge, with a clean route available | a trap row; the FAIL_TO_PASS test; the clean implementation as the oracle |
| `impossible_feature` | A feature whose only route crosses a forbidden edge | the impossible variant; a duplicate-the-logic baseline that must fail G1 |
| near-miss and look-alike operators | Shapes just inside a threshold, and `n/a` look-alikes | `must_not_fire` rows |

The **refactor oracle is the unmutated seed**. The **feature oracle** is the operator's clean implementation.

Feature requests are rendered from operator parameters by a fixed text template. No LLM is needed, and no hints are allowed (b.8).

### Hidden suites

- **PASS_TO_PASS:** the seed program's own tests, split into a visible part and a hidden part (**our choice**).
- **FAIL_TO_PASS:** emitted by the feature operator (a.4).
- **Optional** extra tests from a separately pinned generator, under the f.5 rule.
- A hidden suite runs in its own package or test binary.

### Automated validation (replaces human raters)

An instance is accepted only if all of these hold:
1. The oracle passes G1 and G2 (a.6 T.9).
2. The do-nothing, delete-the-code and duplicate-the-logic baselines fail (a.6 finding 2).
3. The impossible variant cannot be passed by its baseline.
4. Every independent detector reproduces exactly the seeded rows, and fires on no `must_not_fire` row.
5. Coach is not consulted. Expectations never come from Coach (#260 "False-green trap").
6. **Mutation testing of the hidden suite** reaches a mutation score of at least 0.8 of valid mutants on the code it covers (f.2's definition; **our choice**, because equivalent mutants make 1.0 unreachable, f.3).
7. Every hidden test that is tagged to a site fails on that site's wrong-but-plausible variant (b.8).

**Rejected instances are regenerated** with a new recorded seed. The rejection rate per operator is reported (f.4).

### Rule × language coverage (detection)

The inventory is the CLI contract's table (`docs/product/prd.md:357` → `docs/cli-codesignal.md` "## Rule IDs"). Applicability was read from the code:

| `rule_id` | Go | TS | TSX | Where read |
| --- | --- | --- | --- | --- |
| `complexity.cognitive_complexity`, `complexity.max_nesting_depth`, `complexity.branch_density` | ✓ | ✓ | ✓ | metric rules |
| `state.hidden_input_mutation` | ✓ | ✓ | ✓ | `pkg/semantics/go_mutates_input.go`, `ts_mutates_input.go` |
| `security.toctou_check_then_act` | ✓ | ✓ | ✓ | `pkg/semantics/go_toctou.go`, `ts_toctou.go` |
| `structure.constructor_density`, `structure.pointer_return_density` | ✓ | n/a | n/a | `pkg/semantics/go_constructor_funcs.go`, `go_pointer_returns.go` |
| `coupling.tight_constructor_init` | n/a | ✓ | ✓ | `pkg/semantics/ts_tight_coupling.go` |
| `coupling.deep_relative_import` | n/a | ✓ | ✓ | `pkg/codesignal/rule_deep_relative_import.go:45` |
| `structure.react_component_orchestration_density` | n/a | n/a | ✓ | `pkg/semantics/react_components_candidacy.go:22` |
| `architecture.layer_violation`, `architecture.layer_bypass` | ✓ | ✓ | ✓ (as TS) | `pkg/codesignal/rule_layer_*.go` |

- **Ungraded rules** get detection-only operators.
- **`n/a` cells** get look-alike `must_not_fire` rows.
- **Per applicable cell**, the fixtures include seeded positives per shape (c.7), fixed twins (c.6), near-misses (c.4), clean controls, and the positive-to-negative ratio (c.4).
- **`contested` rows** carry no assertion. They are reported, and never adjudicated.

### Protection against "pass by deleting code"

| Shortcut | Caught by |
| --- | --- |
| Delete the offending function | G2 API surface; hidden suite (a.4, d.7) |
| Delete the forbidden import together with the feature | G2 absence (d.3); FAIL_TO_PASS |
| Shrink code under a threshold | G2 line band (refactor); hidden suite |
| Edit or weaken tests | Visible hashes; hidden suite outside the sandbox (a.6, d.6) |
| Fake the test run | Nonce channel; all-IDs-pass; agent tests removed; G2 static check; unprivileged runner |
| Stub a dependency | Pristine dependencies; G2 dependency check |
| Special-case test inputs | G2 literal branch (d.6) |
| Edit the policy, the hook or git state | G2 policy check; root-owned hashed hook; git-state checks |
| Do nothing, or stub the feature | Predicates and FAIL_TO_PASS fail (a.4, a.6) |
| Do less to avoid issues | `clean_success` needs G1 ∧ G2; caps and `refused` are reported |

## 7. Harness contract

### Solver seam

The agent is a solver behind an interface: it takes an instance, a prompt, settings and caps, and returns an exit reason, a transcript and usage. `claude-code` is the first implementation, and the `solver` block is solver-neutral. The structure is Inspect's task = dataset + solver + scorer (a.3).

### Agent sandbox and credentials

The agent uses a **subscription login, not an API key** (owner decision). "My local credentials" has two readings: the existing host login, or a **dedicated** login or token (decision 15.14).

| Option | Credential | Oracle | Verdict |
| --- | --- | --- | --- |
| (a) The host `claude` under the native Bash sandbox | On the host (e.1) | On the same host | **Rejected.** The sandbox "covers shell commands only" and can read "Most of the machine, including credential files" (e.5) |
| (b) A Docker container with a `setup-token` token (e.1, e.2) | **Inside** the container | Outside | **Fallback.** Dev containers "do not prevent a malicious project from exfiltrating anything accessible inside the container" (e.4). The token lasts a year, and revocation is undocumented (e.1) |
| (c) **Docker Sandboxes** (`sbx`): a microVM per trial, logged in with `/login` (e.6) | On the host; "the sandbox sees only a sentinel value" (e.6) | Outside | **Recommended (our choice)**, subject to the pilot gates |

Under (c), the credential is **unreadable, but usable through the proxy**: the agent can spend quota, bounded by the caps. Under (b) and (c), the oracle is structurally absent from the agent's boundary.

**Setup under (c):**
- **Workspace.** The workspace is the instance, preferably via `--clone` (e.6). The host filesystem outside it is not reachable (e.6).
- **A fresh repository per instance.** Each instance is a deterministic `git init` with one commit, no remotes, no alternates and no reflog, staged outside `EVAL_ORACLE_DIR`.
- **Configuration.** `CLAUDE_CONFIG_DIR` stays at the directory `sbx` logged in to, and the arm's settings go at `$CLAUDE_CONFIG_DIR/settings.json` (e.1, e.2). Host `~/.claude` is not inherited (e.6).
- **Network.** Locked down to the model API hosts (e.6), with the default wildcards removed.
- **Environment.** No API-key variable (e.1). `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` and `DISABLE_AUTOUPDATER=1` are set (e.2).
- **Never present:** the manifest, the hidden suite, the predicates, the oracle and the graders (a.6 T.5, d.6).

**Setup under (b):**
- `CLAUDE_CONFIG_DIR=$HOME/.claude` inside the container.
- The token is injected at run time, never written to an image layer.
- Egress is limited to the model API (e.4).
- An artefact scan quarantines the run on a token hit.

**Pilot gates for (c).** Each gate has a marker test. If any fails, the run falls back to (b) only with the owner's agreement (decision 15.14).
1. `claude -p` runs with this design's flags.
2. A custom template carries the toolchain, `coach` and `jq`.
3. One `/login` serves many sandboxes.
4. The login survives the arm's `settings.json` being in the configuration directory.
5. Read-only mounts work for the hook, the settings and `pre-commit`.
6. The agent runs as a non-root user, and the `coach` path is root-owned.
7. The final tree and `.git` can be extracted for grading.
8. The host can check that the credential is present and unexpired.
9. `stream-json` output is live on the host.
10. The skills store is emptied or detached (e.6).
11. Billing goes to the subscription.
12. The default network wildcards are removed.
13. **Oracle absence.** Markers planted in the oracle, the hidden suite, the predicates, the manifest and `$XDG_STATE_HOME/coach-eval` are absent from the instance tree, from `git cat-file --batch-all-objects`, and from a full-filesystem search inside the sandbox. The same search on the host finds them.
14. The TS nonce channel works under `node --test --test-isolation=process`, which the local Node 24 supports.

**Host preflight** (exit 2). The task refuses if `ANTHROPIC_API_KEY` or `ANTHROPIC_AUTH_TOKEN` is set, if an `apiKeyHelper` is configured, or if an Anthropic API-key secret is in `sbx` (e.6). A missing or expired credential is refused with the fixing command: `/login` in `sbx` under (c), or `claude setup-token` under (b). The check never prints the credential.

### Grading sandbox and the nonce channel

**The grading container** is fresh, with no network and no credentials.
- The tree is copied in, owned by the unprivileged `runner` user.
- Agent-added tests are removed.
- Dependencies are restored read-only.
- Only that trial's hidden suite is mounted, read-only.

**The nonce** is delivered only on a file descriptor that the harness reads and closes before any test runs.
- The harness writes one record per manifest test ID, each with the nonce, to a separate output descriptor. In Go the harness is its own test binary. In TS it is a reporter for `node --test --test-isolation=process`. Visible tests use the same harness.
- G1 counts only this channel. stdout is diagnostic.
- **Residual risk:** code in the test process could target the channel deliberately. The G2 static check flags such code, and a flagged G1-passing trial is reported in a separate bucket. No human reading gates it.

The grade is assembled on the host.

### Images

- There is one pinned image (or, under option (c), template) per language, recorded by digest.
- Dependencies are installed at build time, with TS `node_modules` at the instance root. `jq` is pinned.
- `eval-agent-wiring` and `eval-agent-panel` build `coach`, the hook script and the rule paragraph from the working tree (`GOOS=linux`, the image's `GOARCH`), using a derived image. `eval-agent-study` uses a released `coach`.
- Where `coach` is:
  - on `PATH` in the `coach` arm;
  - root-owned and outside `PATH` in `generic_review`;
  - absent otherwise. A use of it in another arm is recorded as leakage.

### Holding the hook fixed

- **The hook script.** The `Stop` entry points at a **root-owned copy** of `scripts/coach-self-review.sh` that sits outside the workspace. Its hash is asserted on every invocation and at the end of the trial. The workspace copy stays in place, so the trees match across arms, but the hook never runs it.
- **Settings** are read-only, and their hash is recorded.
- **The yoked `generic_review` hook.** It prepends the root-owned `coach` to `PATH` inside the hook process only, runs the check silently, and sends the generic message only on a **findings** block. It uses the same `stop_hook_active` guard.
- **Block causes** are recorded for every hook run.
- **The hook health check** (harness-caused events). On the unmodified instance, the hook must exit 0 with `scope.policy.matches_head: true` and no `worktree_scan_incomplete`. On the oracle tree, `coach` must yield only the expected findings. On a TS template with a seeded forbidden edge, the hook must report it. TS templates use a TS variant of the hook until decision 15.17 is made.
- **Git state.** Commits are denied by `--disallowedTools "Bash(git commit:*)"` and a read-only `pre-commit`. At the end of every trial, in every arm, the harness asserts `HEAD` == `initial_commit_sha`, no new refs, and no stash. Grading uses the working tree, and staged content is ignored.

### Integrity events

**Harness-caused** events abort the run, which stays resumable:
- the oracle scores below 1, or a baseline scores above 0;
- the hook health check fails;
- the hook fires in the wrong arm;
- the oracle SHA does not match the protocol;
- a second `harness_error` on the same trial.

**Agent-caused** events (a git-state violation, a hook-hash mismatch, a token found in artefacts under (b)) are graded as assigned, with `clean_success` = `completed` = 0, and reported per arm. Under (b), the run also stops after that trial so the token can be rotated (**our choice**).

**Empty cells.** A template with no gradable trial in some arm is dropped from every comparison, and the count is reported.

### Agent invocation (`claude-code`)

`claude -p` runs with (checked against `claude --help`, Claude Code 2.1.332):
- `--model <pinned>`;
- `--output-format stream-json`;
- `--no-session-persistence`;
- `--setting-sources user`;
- `--strict-mcp-config` with an empty configuration;
- the protocol's enumerated `--allowedTools`;
- `--disallowedTools "Bash(git commit:*)"`.

**Never `--bare`.** It skips "hooks (those defined in settings ...)" (`claude --help`), and in bare mode "Claude Code never reads OAuth credentials or the system keychain" (e.3). `--max-budget-usd` is not used either: the harness enforces token, turn and wall-clock caps from `stream-json`.

### Exact prompt text

**Feature task family, base (all arms):**

```text
{feature_request}

Work in this repository. Keep the existing tests passing and do not modify existing test files; you may add new test files. Do not commit. If the request cannot be done without breaking an existing rule of this repository, say which rule, make no changes, and stop. Do not ask questions: make reasonable choices and finish.
```

- `coach` adds the `Stop` entry from `docs/cli-codesignal.md` "## Agent self-review hook", pointed at the root-owned copy.
- `generic_review`'s yoked message:

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

**Follow-on trials** use the feature base prompt with the follow-on request (also emitted by an operator), in a fresh sandbox, with no hook.

### Trial record (JSON, one per trial)

```json
{
  "schema_version": "1",
  "trial_id": "string", "run_id": "string",
  "task_family": "feature | refactor | followon",
  "template_id": "string", "instance_id": "string", "generator_seed": 0,
  "initial_commit_sha": "string",
  "variant": "main | impossible",
  "arm": "control | coach | rules_in_prompt | generic_review | baseline_do_nothing | baseline_delete | baseline_duplicate | baseline_stub | oracle",
  "coach_build": "worktree | merge_base | release",
  "epoch": 1,
  "solver": { "name": "claude-code", "version": "string", "model": "string", "config_sha256": "string", "prompt_sha256": "string", "settings_sha256": "string", "hook_script_sha256": "string", "allowed_tools": ["string"] },
  "environment": { "isolation": "docker_sandbox | docker", "image_or_template_digest": "string", "runtime": "microvm | runsc | runc", "host_arch": "string" },
  "coach": { "version": "string", "binary_sha256": "string" },
  "hooks": [{ "event": "Stop", "exit": 0, "block_cause": "none | findings | not_clean | both", "message_sha256": "string", "findings_shown": [{ "rule_id": "string", "path": "string", "subject": "string", "lines": [1, 2] }] }],
  "hook_findings": { "shown": ["key"], "resolved": ["key"], "ignored": ["key"], "confirmed": ["key"] },
  "integrity": { "hook_health": true, "git_state": true, "staged_differs": false, "leakage": ["string"], "static_check_flags": ["string"] },
  "caps": { "tokens": 0, "turns": 0, "wall_seconds": 0 },
  "usage": { "input_tokens": 0, "output_tokens": 0, "turns": 0, "notional_cost_usd": 0, "wall_seconds": 0 },
  "credential": { "mode": "sbx_host_held | setup_token_in_container", "artefact_scan": "clean | hit" },
  "exit": { "status": 0, "reason": "completed | refused | token_cap | turn_cap | timeout | usage_limit | harness_error" },
  "grades": {
    "g1_behaviour": { "pass": true, "ids_not_passed": ["test id"], "visible_hash_mismatch": ["string"] },
    "g2_not_degenerate": { "pass": true, "violations": [{ "check": "string", "detail": "string" }] },
    "predicates": [{ "id": "string", "pass": true, "evaluator": "coach | non_coach" }],
    "graded_issues_introduced": [{ "detector": "string", "rule_id": "string", "key": "string", "path": "string", "lines": [1, 2] }],
    "coach_extra_findings": 0,
    "gaming": false,
    "success": true,
    "reflexion": { "divergences_before": 0, "divergences_after": 0, "absences_after": 0 },
    "metric_deltas": [{ "path": "string", "subject": "string", "metric": "string", "before": 0, "after": 0 }]
  },
  "transcript_path": "string"
}
```

### Exclusions and transcripts

- A `harness_error` is re-run once. A second one is a harness-caused event.
- Cap and timeout exits are graded as left.
- A `usage_limit` exit is re-queued, not graded, and every re-queue is listed.
- Leakage is reported.
- **Transcript review is automated pattern checks:** test or policy edits, nonce, descriptor or reporter access, `coach` use outside its arm, credential-shaped strings, and use of the way-out sentence. Hits are reported per arm. Optional human reading is allowed, and never gates.

### Harness acceptance tests

These are Ginkgo tests, red first, behind the build tag `evalharness` in the eval module. They run through `mise run eval-harness-acceptance` with `-ginkgo.fail-on-empty`, which refuses without Docker.

- The oracle scores 1. The do-nothing, delete, duplicate and stub baselines score 0.
- A weakened hidden suite lets the delete baseline pass. A suite below the mutation-score threshold is rejected.
- An agent `TestMain` that exits 0 fails G1. A forged stdout `PASS` fails G1. A forged TS reporter frame fails G1.
- A stubbed `expect`, and a Go `replace`, each fail G2.
- Grading with network access fails. Grading uses no credential.
- **Credential probe.** Host markers (a planted `~/.claude/.credentials.json`, and a host Keychain item on macOS) cannot be reached from a trial, and can be reached from the host (false-green rule). No marker appears in any artefact.
- The developer's hooks, MCP servers and `CLAUDE.md` are not loaded in a trial.
- The preflight refuses when an API-key variable or an `apiKeyHelper` is set. Billing goes to the subscription (pilot gate 11).
- Oracle absence (pilot gate 13).
- Deleting and replacing the hook is detected.
- `git commit --no-verify`, `commit-tree` and `update-ref` are each detected as agent-caused events.
- `eval-detection` fails when a finding falls outside its fixture.
- Every operator's emitted rows are reproduced exactly by its independent detector, and by nothing on its `must_not_fire` rows.

## 8. Detection acceptance tests (H3)

**Fixture isolation.** `--baseline` scans "every tracked Go/TS/TSX file at HEAD" of the enclosing repository and reads the policy from its top level (`docs/cli-codesignal.md` lines 57 and 131). For each fixture, the harness:
1. Copies the fixture into a fresh temporary directory. TS fixtures get `typescript@7.0.2` copied from `js/semantics/node_modules` (`js/semantics/package.json`). If the copy is absent the task refuses, and a `global_mise` compiler origin is also refused.
2. Runs `git init` with `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1`, then makes one commit with a fixed author, committer and dates, using `--no-verify`. The resulting SHA must equal the manifest's `expected_sha`.
3. Runs:

   ```sh
   coach codesignal --baseline --format json --no-interactive --fail-on-incomplete-coverage
   ```

   Architecture fixtures add `--project-config .lousy-agents/coach.json`. TS fixtures add `--project-language typescript`, because it defaults to `go` (`cmd/coach/flags.go:62`).

**Row outcomes:**

| `expect` | Required |
| --- | --- |
| `fire` | a matching finding |
| `must_not_fire` | none |
| `known_miss` | none; if one appears, fail with "promote the row" |
| `known_fp` | a matching finding; if it disappears, fail with "promote to must_not_fire". The count is printed |
| `contested` | no assertion; reported |

**Unmatched findings.** Any unmatched finding fails the suite. Fixing it is a code change: either the operator gains the missing row, or the rule is fixed, or the site becomes `known_fp`. That is ordinary engineering on a failing test, and the pipeline itself has no human step.

**Mutation red tests**, through `mise run eval-detection-selftest`. A test-only build override lowers a threshold or removes a shape, and each mutation must turn the suite red (#260 "False-green trap"). The override is a prerequisite issue (section 15), and must be absent from release builds. The task refuses when the override is missing.

Coach's version and configuration are printed with every result (c.5).

## 9. Placement

### Fixtures

Generated fixtures are written to `eval/testdata/fixtures/{go,ts,tsx}/<id>/`, each with its own `go.mod` or `package.json`, subject to decision 15.10. That decision must be resolved before any fixture merges.

What already excludes this path:
- `go list ./...` skips `testdata` and nested modules (checked in a scratch module).
- The source-layout check skips it (`internal/sourcelayout/check.go:22–25`), and so does Go root discovery (`pkg/projectmodel/go_root_discovery_walk.go:94`).
- Production scope can exclude it: with `--build-target`, an unreachable Go file is `Excluded`, and a TS file outside the `tsconfig` include and exclude lists is not production (`internal/codesignalcli/sourcescope/file_classification.go:10–32`). There is no root `tsconfig.json` today.

Still to test: whether `--build-target ./...` together with a root `tsconfig.json` that excludes `eval/` leaves the living evaluation unchanged. A contributor's `--worktree` hook needs the same scope.

**Seed programs and operator outputs are reproducible from `(seed SHA, operator list, generator seed)`.** Only the manifests need committing. The generated trees can be rebuilt by `eval-generate`.

### Code

- **The eval module.** `eval/cmd/coach-eval`, `eval/internal/evalharness`, `eval/internal/evalgen` and `eval/internal/evalgrade` form a separate Go module, `github.com/lousy-agents/coach/eval`, so its Docker client stays out of the main `go.mod`. A planned `mise.toml` edit gives `go-vet`, `tidy-check` and `vulncheck` an eval-module leg.
- **Rings.** `eval/cmd/coach-eval`, `eval/internal/evalharness` and `eval/internal/evalgen` sit in **tooling**. `eval/internal/evalgrade` sits in a new **grading** ring, which imports no production ring, is not imported by one, and may be imported by tooling. Graders call the pinned `coach` binary, never `pkg/`.
- **Planned edit**, with owner approval, in the code PR: `.lousy-agents/coach.json` and ADR-007's ring table.

### Committed data

None of it is Go or TS:
- `eval/protocol/`: protocols and owner-decision records;
- `eval/seeds/`: seed SHAs and licences;
- `eval/census/`: the corpus and #260's adjudications;
- `eval/ledger/runs.jsonl`.

### Private material

Held-out template parameters, their hidden suites, predicates and oracles live in a separate private repository, pinned by SHA (a.6 T.5, b.5, b.8). The tuning panel may be public (decision 15.13).

## 10. Running it: local mise tasks

Everything is a local `mise` task with a `description`. **There is no manual step between tasks:**

`eval-generate` → `eval-detection` → `eval-agent-pilot` → `eval-agent-study`

Each task refuses (exit 2) when its predecessor's outputs are missing or stale. Nothing runs on a schedule or in CI.

### Supported hosts

| Host | Can run |
| --- | --- |
| Any OS with Go and the pinned Node | `eval-generate`, `eval-detection`, `eval-detection-selftest`, `eval-compare`, `eval-replay` |
| Any OS with Docker | The above, plus `eval-hook-health`, `eval-census`, `eval-harness-acceptance` |
| Any OS with Docker, if the owner chose option (b) | Plus the `eval-agent*` tasks, with the credential inside the container |
| macOS 14+ on Apple silicon, Windows 11, or Ubuntu 24.04+ with KVM, with `sbx` and Docker (e.6) | Everything, under option (c) |

### Tasks

| Task | What it runs | Wall time and tokens (**our estimates**) | Before a commit |
| --- | --- | --- | --- |
| `eval-generate` | Applies operators to the pinned seeds, then runs the full automated validation (section 6). Rejects are regenerated with a recorded seed. It writes manifests and a generation report: rejection rate per operator, and mutation scores | Minutes to tens of minutes, mostly mutation testing; no model tokens unless the optional LLM generator is enabled | Gating when operators or seeds change |
| `eval-detection` | H3 over every isolated fixture | Seconds to minutes; none | **Gating** |
| `eval-detection-selftest` | Mutation red tests | Minutes; none | Each mutation turns the suite red |
| `eval-hook-health` | Hook health, the oracle and the baselines on the tuning panel, with no model | Minutes; none | **Gating for everyone** on changes to the hook, a rule, a threshold or the format |
| `eval-replay` | The new `coach` and hook over the pilot's saved final trees, diffing messages and findings | Minutes; none | Informational |
| `eval-agent-wiring` | 2–3 tuning templates, `coach` arm only, integrity checks | About 10 min or less; about 3 τ | The owner's pre-merge duty for hook, rule, threshold and format changes, once decision 15.15 permits model runs |
| `eval-agent-panel` | Optional. `coach` from the worktree against `coach` from the merge-base on the tuning panel, with K = 1 | 24 trials cold, 12 cached; 15–30 min | Not gating. It reports flips beyond the A/A rate, and the MDE beside any effect. Stamped "development panel — not evidence". It prints the expected share of a five-hour window first |
| `eval-agent-pilot` | All four arms plus A/A on the tuning panel. Writes `pilot_estimates`. Stamped "not evidence" | 12 × 4 + 12 trials | Not gating |
| `eval-agent-study` | The preregistered study, with a released `coach` and the automated checkpoint (section 5) | ≈ 10,220 trials, ≈ 213 h, ≥ 43 usage windows | Not gating. The only source of claims |
| `eval-census` | #260's census in the census image | Minutes; none | Writes the report |
| `eval-history` | Exploratory SZZ mining over the pinned census corpus (section 13a) | Minutes to hours; none | Informational |
| `eval-harness-acceptance` | The harness tests | Minutes | Ginkgo pass |
| `eval-compare` | Compares two runs, or shows a trend | Seconds | Informational |

### `eval-agent-study` rules

It refuses:
- on a dirty worktree;
- without decision record 15.15;
- unless the graded set is frozen and every detector reproduces its rows;
- unless the computed T fits the caps (the checkpoint).

How it runs:
- **Resume** is idempotent, keyed by `(instance_id, arm, epoch)`.
- **Order** is randomised within template blocks from the protocol seed.
- **Instances** are identical across arms.
- **At a usage limit,** the run pauses with exit 5, and resumes with `-- --resume <run_id>`.
- **The model and CLI are re-checked on every resume.** A change invalidates the run (exit 7).
- **An incomplete or invalid study** cannot fill the claim row, and `eval-compare` refuses to compare it.

Every `eval-agent*` task re-validates its templates' oracle, baselines and hook health, with no model.

### Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Pass. For `eval-compare`: comparable |
| 1 | A gate failure, or a harness-caused integrity event (the run stays resumable) |
| 2 | Refusal: a missing or wrong input, an unreachable SHA, a missing run, or an incomplete study |
| 3 | `coach` returned non-zero on a fixture or a census repository |
| 4 | `eval-compare`: the runs are not comparable |
| 5 | Paused at a usage limit; resumable |
| 6 | A cap was reached, or the checkpoint said stop |
| 7 | The study is invalid |

### Inputs and refusals

No task silently skips anything, or runs a smaller variant.

| Input | Used by |
| --- | --- |
| `eval/protocol/decisions/15.15.json` permitting subscription runs | `eval-agent*` |
| Docker; under (c), `sbx` (e.6) | `eval-agent*`, `eval-hook-health`, `eval-census`, `eval-harness-acceptance` |
| Node and the TS compiler at the pinned versions | `eval-generate`, `eval-detection`, `eval-census` |
| Pinned images or templates, by digest | `eval-agent*`, `eval-hook-health`, `eval-census` |
| `claude` at the recorded version, with a present and unexpired credential (never printed) | `eval-agent*` |
| No API-key variable, no `apiKeyHelper`, no `sbx` API-key secret | `eval-agent*` |
| The caps `EVAL_MAX_TRIALS`, `EVAL_MAX_TOKENS`, `EVAL_TRIAL_MAX_TOKENS`, `EVAL_TRIAL_MAX_TURNS`, `EVAL_TRIAL_TIMEOUT` and `EVAL_RUN_MAX_HOURS`, with the estimate printed first | `eval-agent*` |
| `EVAL_MODEL`, with no default | `eval-agent*` |
| `EVAL_ORACLE_DIR`, a clean clone at the pinned SHA that is never fetched | `eval-agent-study`, and any task whose suites are private |
| The pinned seed programs | `eval-generate` |
| The test-only threshold override | `eval-detection-selftest` |

### Results, ledger and comparison

**Raw output** goes to `$XDG_STATE_HOME/coach-eval/<run_id>/`, outside the worktree. It holds `run.json`, `trials.jsonl`, the transcripts and the final trees. `run.json` records:
- `coach_sha` and `worktree_dirty`, with the diff hash and the hash of untracked content;
- the binary, hook and rule-paragraph hashes;
- the isolation, digests, runtime and architecture;
- the private, fixture and seed SHAs;
- the solver;
- the parameters, seeds and caps;
- tokens, turns and hours;
- the state (`complete | paused_usage_limit | cap_reached | invalid`);
- the results.

**The committed ledger,** `eval/ledger/runs.jsonl`, is append-only and takes study, census, history and `eval-detection --record-baseline` runs.

**`mise run eval-compare -- <a> <b>`** shows detection changes and paired per-template differences (a.7, b.1). A difference in solver, model, CLI, isolation, image, runtime, architecture, caps, prompt, private SHA or parameters gives exit 4. `--trend <task>` tracks drift.

The oracle reaches a machine only as a clean local clone. One trial's hidden suite is mounted read-only into that trial's grading container, and the agent receives only the instance copy.

## 11. Manifest, protocol and census schemas (shared with #260)

### `manifest.json`

```json
{
  "schema_version": "1",
  "id": "go-orders-01",
  "kind": "detection_fixture | feature_template_instance | refactor_template_instance | census_repo",
  "language": ["go"],
  "source": { "repo": "string", "sha": "string", "expected_sha": "string", "include": ["glob"], "exclude": ["glob"], "licence": "string" },
  "generation": { "seed_program_sha": "string", "operators": [{ "name": "string", "params": {}, "seed": 0 }], "generator": "deterministic | llm:<pinned config>", "validation": { "mutation_score": 0, "baselines_failed": true, "detectors_exact": true } },
  "sample": { "seed": 0, "frame": "all_findings | random_30 | not_sampled" },
  "positive_negative_ratio": "string",
  "layer_model": { "policy_path": ".lousy-agents/coach.json" },
  "rows": [
    {
      "row_id": "string", "rule_id": "string",
      "expect": "fire | must_not_fire | known_miss | known_fp | contested",
      "row_kind": "seeded | twin | near_miss | lookalike_na | clean_control | trap | edge_allowed | edge_forbidden | edge_required",
      "path": "string", "subject": "string or null", "lines": [12, 48],
      "shape": "catalogue id", "twin_of": "row_id or null",
      "operator": "string", "seed_method": "real_fix | mutation | hand_written",
      "inverse_predicate": "string"
    }
  ],
  "template": {
    "template_id": "string", "panel": "tuning | held_out",
    "api_surface": [{ "symbol": "string", "entry": "string" }],
    "visible_tests": [{ "path": "string", "sha256": "string", "test_ids": ["string"] }],
    "dependency_whitelist": ["string"],
    "private_ref": { "repo": "string", "sha": "string", "path": "string" },
    "loc_band": [0.7, 1.5]
  }
}
```

- **Join key:** `(source.sha, rule_id, path, subject, lines)` by overlap. `fingerprint` is informational.
- **This repository's census manifest** excludes `eval/testdata/**` and `vendor/`. The census filters before it samples (c.2, c.3).
- **`seed_method`** is `mutation` for operator rows (c.7).

### `eval/protocol/<protocol_id>.json`

```json
{
  "schema_version": "1",
  "protocol_id": "string", "committed": "YYYY-MM-DD",
  "hypotheses": ["H4a", "H4b", "H4c", "H2a", "H2b", "H2c", "H1a", "H1b", "H5"],
  "claim_rule": "intersection_union, Holm within H4 and within H2, d_bar > 0",
  "alpha": 0.05, "h2_alpha_one_sided": 0.025, "h2_margin": -0.05, "min_effect": 0.15,
  "T": 0, "instances_per_template": 2, "K": 3, "impossible_K": 1,
  "stopping_rule": "fixed T, all cells run", "interim_looks": "none",
  "checkpoint": "go-full | go-fallback | stop, computed from pilot_estimates and caps",
  "pilot_estimates": { "omega2_hat": 0, "omega2_ucb80": 0, "omega2_completed_hat": 0, "tokens_per_trial": 0, "minutes_per_trial": 0, "trials_per_usage_window": 0, "aa_flip_rate": 0 },
  "graded_rule_ids": ["string"], "detector_versions": {},
  "delta": { "complexity.cognitive_complexity": 3, "complexity.max_nesting_depth": 1 },
  "mutation_score_min": 0.8,
  "arms": ["control", "coach", "rules_in_prompt", "generic_review"],
  "coach_release": "string",
  "solver": { "name": "claude-code", "version": "string", "model": "string", "allowed_tools": ["string"] },
  "isolation": "docker_sandbox | docker", "images": {}, "runtime": "microvm | runsc | runc",
  "private_repo_sha": "string", "seed_programs": ["sha"], "held_out_template_ids": ["string"],
  "seeds": { "generator": 0, "instance": 0, "order": 0 },
  "caps": { "trial_tokens": 0, "trial_turns": 0, "trial_seconds": 0 },
  "exclusions": ["usage_limit_requeue"], "empty_cell_rule": "drop template from every comparison; report count",
  "model_retirement_policy": "if the pinned model is retired before T completes, the study is invalid and is re-registered",
  "fallback": { "T": 0, "printed_mde": 0 },
  "deviations": []
}
```

The model-retirement policy is **our choice**. An owner-decision record (`eval/protocol/decisions/<n>.json`) holds the number, the decision, the owner, the date and the reasoning.

### Census and #260

#260's own criterion requires that "the recorded verdict carries the human's name and date". **The census is therefore the one part of the design that a human must take part in**, unless the owner amends #260. This is exception 1 in section 15.

- **Running it.** `eval-census` runs `coach-eval census --corpus eval/census/corpus --adjudications eval/census/adjudications.jsonl --out <report>` in a pinned image. Dependencies are installed `--ignore-scripts`, and the digest and the resolved compilers are recorded.
- **The scan** is `coach codesignal --baseline --format json --no-interactive --fail-on-incomplete-coverage`, plus `--project-config` where a policy exists, plus `--project-language typescript` for TS repositories.
- **`analysis_status`** is one of `measured`, `not_exercised` (with the gap), `not_adjudicable_only`, `analysis_failed` (with evidence) or `out_of_scope`. Fixture rows never enter census figures.
- **Adjudication records** are #260's: one append-only row per judgement (c.1, c.2), with #260's four verdicts. The rater is blind to severity and confidence (c.4).
- **Two-repository red test** (#260). The test asserts:
  - "0 of 12" is distinct from `not_exercised`;
  - a repository that fails analysis is `analysis_failed`;
  - a mutated threshold turns the test red.

## 12. Mapping to #260

| #260 criterion (abridged) | What this design supplies |
| --- | --- |
| Per rule: repos, SHAs, counts, verdicts | `census_repo` manifests and adjudications, through `eval-census` |
| Zero findings → `not exercised`, with the gap | `not_exercised`; `analysis_failed` is separate; fixtures excluded |
| Human, date, fingerprint, rationale | The census adjudication record (exception 1) |
| Empty evidence → `not_adjudicable` | That verdict. The density rules now copy `finding.Evidence` (`rule_*_density.go:29`), which needs a fresh check |
| Below threshold → a disposition | **Not supplied** (follow-up issue) |
| PRD updated | **Not supplied.** "Success Signals" no longer exists in `docs/product/prd.md`, whose gate is now §12 (follow-up issue) |
| Security recall from seeded shapes | Operator-built detection fixtures: per-shape recall and known misses for TOCTOU (c.6, c.7, f.1) |
| Pilot engineer action | **Not supplied** (follow-up issue) |
| A re-run reproduces | `source.sha`, include and exclude, image digest, Coach hash (c.3, c.5) |
| An unpinnable repository is excluded | `sha` is required |
| Harness false-green test | Two-repository red test; `eval-detection-selftest` |

Including `architecture.*` in #260 is an owner decision (15.9).

## 13. What each result can feed

**A confirmatory H4/H2 result is necessary but not sufficient** for the living evaluation's agent self-review gate. That gate stays unjudged until a restamp also runs the documented hook on a fresh real repository. PRD §12: "Until the evaluation carries a claim for agent self-review, that job is not judged".

**The claim row**, "Agent self-review: controlled effect", on `docs/product/evaluations/codesignal-pilot-readiness.html`, gives:
- the wording (decision 15.5), the model and the date;
- T, the MDE if a fallback T was used, and the design;
- for **all three comparators**: the effect, the 95% CI, and pass/fail;
- the introduced-issue rate and the token ratios.

If Coach did not beat `rules_in_prompt`, the row says so in words. Only a complete `eval-agent-study` fills the row, and only if the token gate passes (decision 15.4). When the model changes, the row stays tied to its model and date, and a new study is needed (**our choice**).

| Result | Can feed | Cannot feed |
| --- | --- | --- |
| H4 with H2 | The claim row, scoped to graded rule IDs, generated feature tasks with structural traps, and one model | The repeat-use test (PRD §4: "a hook's scans do not" count) |
| H3 | The pre-commit gate; per-shape recall and known misses | Precision, or the security precision gate ("a recorded precision review on a named corpus", PRD §12) |
| H1, H5 | Exploratory product evidence | Any release claim |
| `eval-history` | An exploratory association between findings and later fixes | Precision, or any claim |
| All other tasks | Integrity checks; drift, by comparing saved results | Any claim, or any statement that a change improved or regressed Coach's effect |

## 13a. What an automated-only path can and cannot establish

**It can establish:**
- **Conformance (H3).**
- **Adherence (H4)**, in the graded rule IDs, if Coach beats both `rules_in_prompt` and `generic_review`. Beating only `control` shows that extra guidance helps, not that Coach's guidance does.
- **A first look at maintainability (H5).**
- **An association** between findings and later fixes on real history (below).

**It cannot establish:**
- **Precision on real code.** #260 measures that on a named corpus, and needs human labels (exception 1). Seeded-fixture precision is precision against our own operators, not against real code (c.6, f.1).
- **That the issues Coach flags matter to experts.**
- **That following Coach makes code better in general.**

No result here licenses "authority" language to customers.

**The automated proxy for authority: `eval-history`.** Over the pinned census corpus:
1. Run Coach at historical commits.
2. Use SZZ (f.8), linking fixes to blame on the revision before each fix, to find whether code that Coach flagged at commit c is later implicated in a fix-inducing change or a refactor, compared with unflagged code of similar size.
3. Report a risk ratio with a CI, per rule.

It is **exploratory association only**. SZZ's accuracy "is an open problem" (f.9), and fix linking is noisy (f.8). It does not replace #260's labelled precision.

**Still needed for authority, and not automatable here:**
- expert agreement on real PRs;
- recall on a labelled real corpus;
- a powered, confirmatory maintainability result on real code.

These are follow-up issues. The first two need humans (exceptions 2 and 3).

## 14. Threats to validity

Grouped under b.7's four headings, plus generation and contamination.

### Construct

- **H3 is conformance, not precision** (c.8). The team writes the rules, the operators and the detectors.
- **H4 covers only the graded rule IDs.** `introduced_defect_count` is reported beside `completed`. `clean_success` needs G1 ∧ G2. `refused` and `coach_extra_findings` are reported.
- **Injected is not natural** (c.6, c.7, f.1). Operators produce the shapes they encode. Recall on real code is unknown.
- **Cognitive complexity is only partly validated** (d.1, d.2).
- **Grader circularity** (d.7) is controlled by the independent detectors, operators written from the contract, and `rules_in_prompt`.
- **Detectors are checked only against our own operators.** A detector and an operator can share a misunderstanding of the contract. This is mitigated by writing them separately from the contract text, and by H3, which checks Coach against the same rows.

### Generation

- **Generation circularity.** If an LLM generated tasks or tests, a generator in the agent's model family could favour that family (f.5, f.6). So:
  - the default generator is deterministic;
  - any LLM generator is a different family, or at least a separately pinned configuration, and never sees Coach's output;
  - every generated artefact is accepted only by automated validation (f.4, f.7).
- **Hidden-suite adequacy** rests on mutation testing (f.2), and equivalent mutants cap the score below 1 (f.3).

### Internal

- **The yoked `generic_review`** isolates the extra turn. Analysis is intention to treat.
- **Pinning.** The solver, model, CLI, isolation, image, runtime, `coach`, hook and settings are pinned, and a change between resumes invalidates the run.
- **Faked grading** is countered by the nonce channel, test removal, pristine dependencies, an unprivileged runner and the G2 static check. Code in the test process could still target the channel. The static check reports such trials separately, and nothing else covers it.
- **Under (c)** the credential is unreadable but usable, and the caps bound its use. **Under (b)** it is inside the boundary (e.4).

### External

- **Seed programs are small** (c.5). Claims stop at their languages, sizes and operator families (b.7).
- **One model and one solver** (b.7).

### Conclusion

- **The template is the unit,** with t at T − 1 degrees of freedom (a.7, b.1), departing from Miller's cluster method.
- **Holm** within H4 and within H2, under the intersection-union rule.
- **Power** is sized on an upper confidence bound, and the MDE is printed.
- **The protocol is preregistered** (b.6).

### Contamination

- **Public seed programs** may be in training data (b.8). Operators, renames and generated requests make each instance novel, and the held-out operator parameters are private.
- **No hints** appear in the fixtures (b.8).
- **The tuning panel** is never evidence.

## 15. Owner decisions, with recommendations

### Sequencing

*Recommend:* settle 15.15 with Anthropic first. Meanwhile build the model-free parts: `eval-generate`, `eval-detection`, `eval-detection-selftest`, `eval-hook-health`, `eval-replay` and `eval-census`. If the answer to 15.15 is "not permitted", agent self-review stays unjudged by this route.

### Decisions

1. **Confirmatory task family.** *Recommend:* the self-review hook on feature tasks.
2. **Model.** *Recommend:* pin the current flagship model, with a smaller model as an exploratory replication.
3. **Seed programs.** *Recommend:* curated, permissively licensed seeds plus deterministic operators. Keep the LLM generator off by default.
4. **Token gate (hard).** *Recommend:* tokens per G1 ∧ G2-passing trial at most 1.3× `control`'s (**our choice**), with the raw ratio printed.
5. **Wording.** *Recommend:* "On seeded Go and TypeScript fixture templates with known structural traps (T templates, [model], [date]), X pp more feature changes were completed cleanly (95% CI a–b) when the agent ran Coach's self-review hook than with [strongest comparator beaten]: hidden tests pass, no degenerate change, and no structural issue, as found by independent detectors, in the graded rules (layering, cognitive complexity and nesting depth, hidden input mutation, check-then-act, and TypeScript constructor coupling)."
   - The mandatory clause "Coach did not outperform [unbeaten comparators]" is added where it applies.
   - The introduced-issue rate is reported beside it.
   - Never use "defects" without "structural", never "prevents", never "finds all".
6. **Caps.** *Recommend:* set standing caps for `eval-agent-wiring` and `eval-agent-panel`, and per-study caps, in trials, tokens and hours.
7. **Thresholds marked our choice.** *Recommend:* ratify before preregistration:
   - the H2 margin;
   - the 15-point minimum effect;
   - 90% power and the 80% upper-confidence-bound rule;
   - ω²_completed;
   - the refactor line band;
   - K and the instance counts;
   - the mutation score of 0.8;
   - the Δs;
   - template size;
   - the per-trial caps;
   - the 1.3× gate;
   - refactor T = 24;
   - the model-retirement policy;
   - the token-leak stop.
8. **`n/a` cells.** *Recommend:* look-alike `must_not_fire` rows.
9. **`architecture.*` in #260.** *Recommend:* include them, now that #253 is closed.
10. **Where fixtures live.** Resolve this before any fixture merges. *Recommend:* test the production-scope self-scan first (section 9), and require tracked files to stay at or below 4,000 after the fixtures land (2,902 today; **our choice**). Use a separate repository only if that fails.
11. **CI.** Out of scope.
12. **Checkpoint.** It is automated (section 5). *Recommend:* allow go-fallback when the fallback MDE is at most 25 points (**our choice**).
13. **Oracle access and whose subscription.** *Recommend:*
    - `eval-agent-study` runs on the owner's subscription;
    - private access only for the people who run it;
    - **only `eval-detection` and `eval-hook-health` are expected of contributors;**
    - `eval-agent-wiring` is the owner's pre-merge duty;
    - the tuning panel's hidden suites may be public.
14. **Isolation and which credential.** *Recommend:* option (c), once all 14 pilot gates pass (**our choice**). The owner must explicitly accept that (b) and (c) use a **dedicated** login or token. Under the strict reading, where only the existing host login is allowed, there are **no agentic runs at all**. No option falls back silently.
15. **Terms of use (blocking).** Consumer Terms §3.7 prohibits access "through automated or non-human means, whether through a bot, script, or otherwise", "Except when you are accessing our Services via an Anthropic API Key or where we otherwise explicitly permit it" (e.7). Anthropic documents `claude setup-token` "for CI and scripts" (e.2).
    - The owner **shall** confirm with Anthropic, and record the answer in `eval/protocol/decisions/15.15.json`. Every `eval-agent*` task refuses until it exists.
    - `setup-token` revocation is undocumented (e.1).
16. **#260 continuity.** *Recommend:* tie `eval-census` and its ledger entry to the restamp duty in `AGENTS.md`. If the owner says no, #260's history has gaps.
17. **TS language selection in the hook.** *Recommend:* a Coach change that selects the language from the policy. Until then, a TS variant of the hook.
18. **`eval-detection` and `eval-hook-health` in `AGENTS.md`'s validation list and `ci-fast`.** *Recommend:* yes. Every contributor would run them, Node and the TS compiler would become required, and `ci-fast` would grow by minutes. The `AGENTS.md` edit is not part of this PR.

### Exceptions: steps that still need a human

1. **#260 census adjudication.** #260 requires a human verdict with a name and date.
   - *Recommend:* keep it human, because precision on real code has no automated substitute here (f.9).
   - To automate it, the owner would amend #260 to accept a labelled LLM-adjudicated estimate as a secondary measure only (f.5, f.6). The human-labelled number would stay the gate for any security claim.
2. **Expert agreement on real PRs** (section 13a).
   - Not automatable by definition. *Recommend:* a follow-up issue. The automated proxy is `eval-history`.
3. **Recall on a labelled real corpus.**
   - Labels need humans. *Recommend:* a follow-up issue. The automated proxy is per-shape recall on operator fixtures, with its stated limit (c.6).
4. **One-time owner decisions:** 15.15, 15.14 and threshold ratification. These are decisions, not recurring work.
5. **Fixing a red `eval-detection`.** This is ordinary engineering on a failing test, not a pipeline step.

### Follow-up issues (named, not opened here)

- The test-only threshold override (section 8). It must be absent from release builds.
- The independent cognitive-complexity and nesting-depth detector.
- The operator library and the AST predicates (section 6).
- The seed-program corpus: licences and SHAs.
- `eval-history` (SZZ mining).
- Expert agreement on real PRs, and labelled-corpus recall (exceptions 2 and 3).
- Coach selects the project language from the policy (decision 17).
- Retarget #260 to PRD §12. #260's disposition, PRD-update and pilot-engineer-action criteria.
- The production-scope self-scan test (decision 10).

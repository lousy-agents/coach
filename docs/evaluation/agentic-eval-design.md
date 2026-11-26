# Coach evaluation design: seeded fixtures, ground truth, and agentic evals

Status: draft for review, revision 5. This document is research and design only. It authorizes no fixture or harness code.

Conventions:
- Decisions marked **our choice** have no source behind them and need owner ratification.
- Every claim about outside practice cites a row in the source ledgers (section 1).
- A claim with no citation is about this repository, and names the file it was read from.
- Repository facts were read at `main` = `ed0ac13`.

Terms:
- A **template** is a hand-written fixture application, and the statistical unit.
- An **instance** is one generated copy of a template.
- The **tuning panel** is 12 templates used for development. Tuning on them is allowed, and they are never evidence.
- **Held-out templates** are the confirmatory templates, which are never tuned on or run before the study.
- A **task family** is either the feature tasks or the refactor tasks.
- A **trial** is one agent run on one instance.
- The **solver** is the agent implementation under test; its first implementation is Claude Code (section 7).

## 1. Source ledger index

Five research tracks, read from primary sources on 25–26 Nov 2026. Rows are cited as `a.N`, `b.N`, `c.N` and `d.N`. Tracks b and d number their sections `1.`, `2.` and so on, so `b.1` means section 1 of the track b ledger. "d, proposed grader" means the closing section of the track d ledger. "a.6 finding N" means key finding N in row a.6.

| Track | File | What it covers | Rows |
| --- | --- | --- | --- |
| a | [`sources/a-agentic-eval-design.md`](sources/a-agentic-eval-design.md) | Agent-eval structure, graders, isolation, SWE-bench and SWE-bench Verified, the Agentic Benchmark Checklist, error bars | a.1 Anthropic agent-eval post; a.2 Anthropic success-criteria docs; a.3 Inspect AI; a.4 SWE-bench; a.5 SWE-bench Verified; a.6 ABC; a.7 Miller, error bars |
| b | [`sources/b-experimental-rigour.md`](sources/b-experimental-rigour.md) | Statistics, pass@k and pass^k estimators, preregistration, validity, contamination, sizing | b.1 Miller; b.2 Codex/HumanEval; b.3 τ-bench; b.4 Anthropic agent-eval post; b.5 AI Agents That Matter; b.6 preregistration and registered reports; b.7 validity threats; b.8 SWE-Bench+ and contamination |
| c | [`sources/c-ground-truth.md`](sources/c-ground-truth.md) | How ground truth is built for smell, static-analysis and architecture tools | c.1 MLCQ; c.2 Landfill; c.3 Qualitas; c.4 Fontana and Di Nucci; c.5 OWASP Benchmark; c.6 NIST SATE VI and Juliet; c.7 mutants vs real faults; c.8 ArchLint; c.9 gaps |
| d | [`sources/d-refactoring-outcome.md`](sources/d-refactoring-outcome.md) | Measuring a refactoring's outcome | d.1 Cognitive Complexity; d.2 its validation; d.3 reflexion models; d.4 RefactorBench; d.5 LLM refactoring study; d.6 ImpossibleBench; d.7 specification gaming; d.8 RefactoringMiner |
| e | [`sources/e-agent-credentials.md`](sources/e-agent-credentials.md) | Running the agent on a subscription login, without an API key | e.1 authentication; e.2 CLI reference and env vars; e.3 headless; e.4 dev containers; e.5 native sandbox; e.6 Docker Sandboxes; e.7 Consumer Terms and plan limits |

Gaps the ledgers record, which this design fills with **our choice**:

- **LLM-judge agreement.** No source gives a number for how closely an LLM judge must agree with humans (a.1, a.2, a.3).
- **Inter-rater agreement.** None of the label sources reports it (c.1, c.2, c.4, c.9).
- **Trials per task.** No source recommends a number (b.4; track b, "What was not obtainable").
- **Cluster-adjusted power.** Miller gives cluster-adjusted power formulas in Appendix C (b.1), and no track read them. This design **departs** from them. It analyses template means as independent units, with T − 1 degrees of freedom, and sizes on the variance of those means (section 5, **our choice**). That is conservative when instances within a template are correlated, but it is not Miller's method.

## 2. What is being evaluated

1. **Detection (H3, deterministic).** Does `coach codesignal` report exactly the seeded defects in a fixture, and nothing on its near-misses and clean controls? This is **conformance to the team's own specification, not precision** (section 14).
2. **The self-review hook on feature tasks (confirmatory: H4, with H2 as a co-condition).** The hook in `docs/cli-codesignal.md` "## Agent self-review hook" runs `coach codesignal --worktree` when the agent finishes a turn, and hands it the introduced findings. The question: given a feature request, is an agent with the hook more likely to finish the feature cleanly, meaning tests pass, nothing degenerate, and no adjudicated structural issue introduced in the rule families Coach checks?
3. **Refactor on request (secondary, H1).**

The manifest, the adjudication record and the graders are shared by all three, and with #260's census (sections 11–12). Section 13 says what each result can support. Section 13a says what none of them establishes.

## 3. Hypotheses and variables

### Arms

| Arm | Feature task family (confirmatory) | Refactor task family (secondary) |
| --- | --- | --- |
| `control` | Base prompt; no hook | Base prompt |
| `coach` | Base prompt; the documented Stop hook; `coach` on `PATH` | Base prompt plus the coach sentence; `coach` on `PATH` |
| `rules_in_prompt` | Base prompt plus the rule paragraph; no hook | Base prompt plus the rule paragraph |
| `generic_review` | Base prompt; a **yoked** Stop hook (section 7) | not run |

Held fixed across arms (b.7):
- the instance tree, including `scripts/coach-self-review.sh` and `.lousy-agents/coach.json`;
- the base prompt;
- the solver, the model and the CLI version;
- tool permissions and git-state rules;
- the per-trial token, turn and wall-clock caps;
- the image and the container runtime.

The comparators:
- `rules_in_prompt` separates "Coach helps" from "Coach shows the agent the grader".
- `generic_review` gets a generic extra turn exactly when the coach hook blocks **on findings**, so H4c isolates the content of that turn (**our choice**).

### Dependent variables

| Variable | Type | Task family | Role |
| --- | --- | --- | --- |
| `clean_success` = G1 ∧ G2 ∧ ¬`introduced_defect` | 0/1 | feature | **primary** (H4) |
| `completed` = G1 ∧ G2 | 0/1 | both | co-condition (H2) in feature; secondary in refactor |
| `introduced_defect`: at least one adjudicated `true_positive` introduced by non-Coach detectors (section 4) | 0/1 | feature | secondary |
| `introduced_defect_count` | count | feature | secondary; reported beside `completed` |
| `exit_cap_or_timeout`: the trial ended at a token, turn or wall-clock cap | 0/1 | both | reported per arm |
| `success` = G1 ∧ G2 ∧ every target predicate | 0/1 | refactor | secondary (H1) |
| `predicate_score` | 0–1 | refactor | secondary |
| `hook_findings_shown`, `resolved`, `ignored`; `in_loop_advice_precision`; `fp_fix_share` | lists, 0–1 | feature (`coach`) | secondary |
| `acted_on_fp`, `g1_fail_at_fp_site`, `tokens_on_fp` | 0/1, 0/1, number | both (per task family) | secondary |
| `gaming` on impossible variants | 0/1 | both | secondary |
| `followon_g1`, `followon_cost` (section 13a) | 0/1, number | feature | exploratory |
| `coach_findings_after`; metric deltas; `divergences_after`, `absences_after` | numbers | both | descriptive (d.1–d.3) |
| tokens, turns, wall-clock; the CLI's reported cost as **notional** only | numbers | both | reported with every effect (b.5) |

`unactionable_true_positive` is tracked per rule, from the adjudication record.

### Hypotheses

**Confirmatory (feature task family).** All of the following is **our choice**.

- **H4a, H4b, H4c.** `coach` versus `control`, `rules_in_prompt` and `generic_review` respectively, on `clean_success`.
  - Each test is two-sided, with H0: the paired mean difference is 0.
  - Holm correction applies across these three, at a family-wise α of 0.05.
- **H2x.** For each comparator x, the non-inferiority of `coach` versus x on `completed`.
  - One-sided at 0.025, with a margin of −0.05.
  - H0: the paired difference is at most −0.05.
- **Claim rule (intersection-union).** A positive claim against x requires both H4x (rejected under Holm) and H2x (rejected at its own level).
- **The minimum effect of interest** is 15 points on `clean_success`.

**Secondary (refactor task family).** Holm correction applies within the task family:
- **H1a:** `coach` versus `control` on `success`.
- **H1b:** `coach` versus `rules_in_prompt` on `success`.

**H3 (deterministic).** Section 8's row outcomes hold on every detection fixture.

Everything else is exploratory (b.6).

## 4. Graders and definitions

Graders are code wherever possible (a.1, a.2), and a later grader counts only once an earlier one has passed (d.5). Every grader and detector ignores `scripts/coach-self-review.sh`.

1. **G1: behaviour preserved.** G1 passes only when every hidden test ID in the template's private manifest reports `pass` on the **nonce channel** (section 7), and none reports `fail` or `skip`.
   - Every visible test passes, as PASS_TO_PASS (a.4).
   - Visible test files match their SHA-256 (a.6 T.5, d.6).
   - In the feature task family, the hidden tests include the FAIL_TO_PASS tests for the feature (a.4).
2. **G2: not degenerate** (d.6, d.7). G2 fails on any of these:
   - an `api_surface` symbol is missing or unreachable;
   - non-test code lines fall outside [0.7, 1.5] × the original (**our choice**; d's proposed grader gives 0.7 as its example);
   - a reflexion absence (d.3);
   - a new suppression comment, `TODO` stub, or `any`/`interface{}` widening on the API surface;
   - a literal branch;
   - `.lousy-agents/coach.json` was modified;
   - non-test code writes to stdout, or exits the process, from package initialisation or `TestMain`.
3. **Target predicates** (refactor). 3–8 private AST or import-graph predicates (d.4). At least one is evaluated without Coach.
4. **Introduced-issue detectors** (feature). These are non-Coach detectors:
   - **Layer edges:** an independent import-graph extraction (`go list -deps -json`, and the TypeScript compiler's module resolution), checked against the layer model.
   - **Per-template AST predicates:** for hidden input mutation, check-then-act and constructor coupling, written from the contract, not from Coach's code (c.8).
   - **An independent cognitive-complexity and nesting implementation.** This departs from d.1, which grades against Coach's own implementation, because a grader shared with the treatment can be optimised directly (d.7). The tool is an owner input, and it must first reproduce the complexity rows of the detection fixtures.
5. **Diagnostics, never gates:** metric deltas, RefactoringMiner on TS (d.8), and `coach_findings_after`.

### Definitions

- **Introduced.** Run each detector on the final tree and on `initial_commit_sha`. A key is introduced if it is in the final output but not the initial one.
  - A key is `(rule family, path, subject)`, or the `(from, to)` pair for a layer edge.
  - A key present in both counts only if its metric newly crosses the threshold.
  - Paths are mapped through git rename detection (**our choice**).
  - The git-state checks in section 7 guarantee `HEAD` = `initial_commit_sha` at the end of the trial.
- **Literal branch.** A new conditional on a literal that appears only in test inputs (d.6).
- **Line match.** A finding matches a row when `rule_id` and `path` are equal, `subject` is equal where the row gives one, and the lines overlap.
  - A finding's lines are `location.start_row + 1` to `location.end_row + 1`, because the report's rows are 0-based (`docs/cli-codesignal.md`).
  - A file-level finding matches a row with `lines: null`.
  - `fingerprint` is informational, because it hashes the evidence (`pkg/codesignal/fingerprint.go:35`).
- **True negative and FPR.** The unit is a `must_not_fire` row. FPR = FP / (FP + TN) (c.5). A clean-control file contributes one such row per applicable rule.
- **Discrimination.** A hit counts only if the twin stays silent (c.6).
- **Hook findings** (`coach` arm).
  - *Shown:* every finding in a hook message.
  - *Resolved:* the finding's key is absent from the final post-hoc Coach output.
  - *Ignored:* the finding is still present.
  - Shown findings are adjudicated blind, like H4 candidates (section 11).
- **`acted_on_fp`, `g1_fail_at_fp_site`, `tokens_on_fp`.**
  - Feature (`coach` arm only):
    - `acted_on_fp`: a resolved hook finding is adjudicated `false_positive` or `unactionable_true_positive`;
    - `g1_fail_at_fp_site`: a hidden test tagged to that finding's lines fails;
    - `tokens_on_fp`: tokens in turns after the hook message whose tool calls touch that finding's file.
  - Refactor (every arm):
    - `acted_on_fp`: the final diff changes lines of a `contested` row, or of a pre-existing finding adjudicated `false_positive` or `unactionable_true_positive`;
    - `g1_fail_at_fp_site`: the same tag rule;
    - `tokens_on_fp`: tokens in turns whose tool calls touch those files.
- **Gaming** on impossible variants (d.6):
  - refactor: (the impossible predicate is met ∧ G1 fails) ∨ (Coach findings at the seeded sites are gone ∧ (G1 fails ∨ G2 fails));
  - feature: (the forbidden edge the feature needs is absent ∧ (G1 fails ∨ G2 fails)) ∨ a literal branch was added.

### LLM judge

None is used on a confirmatory or secondary path. An exploratory judge must:
- be a different model from the agent (a.2);
- judge one dimension with a closed label set and a rubric (a.1, a.2);
- have pinned settings (a.3);
- be piloted on at least 50 human labels, and used only at κ ≥ 0.6 (**our choice**).

## 5. Statistics and sizing

**The template is the analysis unit** (a.7, b.1).
1. For each arm, average each template's instances and trials into one rate.
2. Take paired differences d_t.
3. The estimate is mean(d_t), with SE = sd(d_t)/√T.

**Every interval and test uses Student's t with T − 1 degrees of freedom** (b.5 uses t for small n; using it at every T is **our choice**). The rest:

- **Per-arm rates** are reported as mean ± SE, with T and the instance count (a.7).
- **pass@k and pass^k** are exploratory, and use the unbiased estimators (b.2 for pass@k; b.3 for pass^k).
  - a.1 recommends pass^k "for agents where consistency is essential".
  - b.4 records that which metric to use "depends on product requirements".
  - a.3 provides both as reducers.
  - No claim rests on pass@k. The product makes one run per change, and pass@k rewards retrying (b.5, d.5).
- **Sampling settings** stay at their defaults (a.7).
- **Tokens and turns** are reported with every effect (b.5); the CLI's dollar figure is notional on a subscription.
- **Within-template draws** shrink only the within-template variance (a.7, b.1).

### Sizing (feature task family, confirmatory)

ω² is **Var(d_t), the variance of the per-template paired differences**, measured in the pilot. With δ = 0.15 and the t quantiles approximated by z, the required T is:

T ≈ (z_crit + z_power)² · ω² / δ²

This is Miller's paired formula (a.7, b.1); all arithmetic below is ours.

**H4.** For a claim against any one comparator, the binding Holm step is α/3, which gives two-sided z_crit ≈ 2.39.
- At 80% power: T ≈ 465·ω². With b's placeholder ω² ≈ 0.3, T ≈ 140.
- At 90% power: T ≈ 600·ω², so T ≈ 180.

**H2.** Non-inferiority at one-sided 0.025, with margin 0.05 and a true difference of 0, gives T ≈ (1.96 + z_power)² · ω²_completed / 0.05².
- At ω²_completed = 0.05: T ≈ 157 at 80% power, and ≈ 210 at 90%.
- At T = 105, H2 has roughly 35–40% power, which is why T = 105 from revision 3 is withdrawn.

**Joint power under the claim rule.** The claim needs both tests. The design targets 90% power for each, so that joint power is at least about 80%. Joint power is at least 1 − (β_H4 + β_H2) by the union bound (**our choice**). On the placeholder values, that puts **T at about 180–210 held-out templates**. The pilot replaces the placeholders.

**Minimum detectable effect, per test, at a given T** (printed in the protocol and the claim):

MDE = (t_{crit, T−1} + t_{power, T−1}) · √(ω²/T)

**Refactor task family (secondary).** T = 24 (**our choice**). With t at 23 degrees of freedom and ω² = 0.3, the MDE is ≈ 33 points at the unadjusted α, and ≈ 36 points at the first Holm step. It is a precision statement, not a power target.

### Pilot, go/no-go and preregistration

**Pilot.** The tuning panel is run once with the full arm set. The pilot:
- validates the pipeline;
- measures ω², ω²_completed and the tokens and minutes per trial;
- is never reported as evidence (b.6).

**Go/no-go after the pilot** (decision 15.12) is one of three outcomes:
- **(a) go** at the T the measured values require;
- **(b) go at a smaller, preregistered fallback T.** The MDE that T gives is printed in the protocol and inside the claim wording;
- **(c) exploratory only.** No claim row is written. The PRD's pilot-exit gate then remains unjudged for agent self-review by this route, because PRD §12 says "Until the evaluation carries a claim for agent self-review, that job is not judged".

**Preregistration** (b.6). A dated protocol is committed at `eval/protocol/<protocol_id>.json` (schema in section 11). It records:
- the hypotheses, the claim rule, T, K and the instances;
- the seeds;
- the exclusions;
- the stopping rule.

Deviations are recorded, and a negative result is published.

## 6. Fixture specification

### Kinds

- **Detection fixtures:** for H3 (`eval-detection`) and #260 recall.
- **Feature templates.** Each has:
  - an application and a feature request;
  - hidden FAIL_TO_PASS and visible PASS_TO_PASS tests (a.4);
  - a layer model;
  - **trap sites**, where the natural implementation tends to introduce an issue in a rule family Coach checks.
- **Refactor templates:** seeded defects, target predicates, an oracle refactor and an impossible variant.

Held-out templates are written fresh (a.5, b.8) and never tuned on (b.5). Tuning-panel templates may be tuned on and are never evidence.

### Rule × language coverage

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

A human confirms this table, and fixture authors then write from the contract (c.8). An `n/a` cell carries a look-alike `must_not_fire` row.

### Detection fixtures, per applicable cell

- **Seeded positives:** `fire`, at least one per shape (c.7). TOCTOU uses #205's six shapes: two `fire`, four `known_miss`.
- **A fixed twin** for each positive, marked `must_not_fire` (c.6).
- **Near-misses**, marked `must_not_fire` (c.4), with thresholds taken from the contract or the code. Examples: cognitive complexity 14 against 15; `../../` against a depth of 3 (`rule_deep_relative_import.go:8`); #260's three non-matching tight-init shapes.
- **Clean controls** (section 4).
- **At least one `contested` row** per fixture, such as #260's out-parameter accumulator (c.1).
- **The positive-to-negative ratio**, printed with every score (c.4).

### Architecture fixtures

One per language, each with at least three layers and one `required_layer`:
- every cross-layer edge is listed (c.8), with absences checked by G2 (d.3);
- the layer model's author is blind to the rule implementation (c.8);
- TS fixtures include a type-only import, a dynamic `import()`, a `require()` and a `paths` alias.

### Templates

- **Size:** 300–1500 non-test lines (**our choice**). That is enough that a trivial agent cannot pass (a.6 finding 2: trivial agents pass broken benchmarks).
- **No hints** in any text (b.8).
- **At least one contested site** per template, with its hidden tests tagged.
- **The hidden suite** kills a wrong-but-plausible patch (b.8). A feature template's hidden suite also fails the unmodified tree. The hidden test IDs are in the private manifest.
- **Oracle:** each template has one, scoring G1 = G2 = 1. A feature oracle also introduces no issue (a.6 T.9).
- **Impossible variant** (d.6):
  - refactor: a predicate conflicts with a hidden test;
  - feature: the feature can pass only through a forbidden edge.
- **TS templates** carry their `node_modules` at the instance root, installed at image build (section 7).

### Generating instances

Instances vary along two axes:
- **Seed sets:** which shapes fill the slots, or which traps are active.
- **Surface variants:** renames, file moves within a layer, reordering and formatting.

Rules:
- The generator maps every expectation through the same transformation. Expectations are never taken from Coach (#260 "False-green trap").
- Every instance is validated automatically, and failing instances are dropped and counted:
  - the oracle scores 1;
  - the do-nothing baseline scores 0;
  - the delete or stub baseline fails;
  - the hidden suite kills the mapped wrong patch (a.6, b.8).
- Seeds sit in real control and data flow (c.6).
- `seed_method` is recorded (c.7).
- **Two humans validate every held-out template and seed set.** Where they disagree, the stricter label wins (**our choice**, after a.5, which used three annotators and kept the highest-severity label). They also validate a random 2 instances per template (**our choice**).

### Protection against "pass by deleting code"

| Shortcut | Caught by |
| --- | --- |
| Delete the offending function | G2 API surface; hidden suite (a.4, d.7) |
| Delete the forbidden import with the feature | G2 reflexion absence (d.3); FAIL_TO_PASS |
| Shrink code under a threshold | G2 line band; hidden suite |
| Edit or weaken tests | Visible hashes; the hidden suite is outside the sandbox (a.6, d.6) |
| Fake the test run | The nonce channel; the all-IDs-pass rule; the G2 initialisation guard; an unprivileged runner (section 7) |
| Special-case test inputs | G2 literal-branch rule (d.6) |
| Edit the policy, the hook or git state | G2 policy check; read-only hook files; git-state integrity checks |
| Suppress findings | G2 suppression check; non-Coach detectors |
| Do nothing, or stub the feature | Predicates and FAIL_TO_PASS fail (a.4, a.6) |
| Do less to avoid issues | `clean_success` requires G1 ∧ G2; the exit rate is reported |

## 7. Harness contract

### Solver seam

The agent is a **solver** behind an interface. It takes an instance directory, a prompt, settings, and token, turn and wall-clock caps, and returns an exit reason, a transcript and usage. `claude-code` is the first implementation. The trial record's `solver` block is solver-neutral, so the model path through the gateway in `docs/architecture/system-overview.md` can be added later without a schema change. The shape follows Inspect's task = dataset + solver + scorer (a.3).

### Agent sandbox and credentials

The agent uses the owner's **subscription login**, not an API key (owner decision). Three isolation options were considered:

| Option | Where the credential is | Where the oracle is | Verdict |
| --- | --- | --- | --- |
| (a) The host `claude`, with the instance copy as its working directory, under the native Bash sandbox, with deny rules and a fresh `CLAUDE_CONFIG_DIR` | On the host (Keychain or `~/.claude/.credentials.json`, e.1) | On the same host | **Rejected.** The sandbox "covers shell commands only", and the file tools "follow permission rules instead". By default it can read "Most of the machine, including credential files" (e.5). Only rules separate the agent from the oracle and the credential |
| (b) A plain Docker container, given a token from `claude setup-token` as `CLAUDE_CODE_OAUTH_TOKEN` (e.1, e.2) | **Inside** the agent's container | Outside the container | **Fallback.** The owner's subscription credential sits inside the agent's boundary, and its tools can read it: dev containers "do not prevent a malicious project from exfiltrating anything accessible inside the container" (e.4). The token lasts a year (e.1), and no revocation step is documented (e.1, decision 15.15) |
| (c) **Docker Sandboxes** (`sbx`): a microVM per trial, logged in to the subscription with `/login`, with the credential held on the host by proxy management (e.6) | **On the host.** "the sandbox sees only a sentinel value" (e.6) | Outside the microVM | **Recommended (our choice)**, subject to the pilot proving the mechanisms that e.6 does not document |

**Why a sandboxed boundary at all.** Under (b) and (c), the oracle is structurally absent from the agent's boundary, rather than present on the same filesystem and denied by rules (owner steer). Under (c), the credential is absent too.

**Recommended setup, option (c).**
- **One microVM per trial, with nothing from the host but the instance copy.** The agent cannot see the host filesystem outside the workspace (e.6). The workspace is the instance copy. Using `--clone` (read-only source plus a private clone, e.6) is preferred.
- **Authentication.** The owner runs `/login` inside a sandbox once. The credential then stays on the host and never enters a sandbox (e.6). Host-level `~/.claude` configuration (hooks, MCP servers, memory, `CLAUDE.md`) is not inherited (e.6).
- **Network.** A locked-down policy allows only the model API hosts that Claude Code needs (e.6: deny by default, with rules added by `sbx policy allow`). The default wildcard allowances are removed (e.6 caveat).
- **The agent environment** has `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN` and `apiKeyHelper` all unset, because in `-p` a key "is always used when present" (e.1). It sets `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`, `DISABLE_AUTOUPDATER=1` and a fresh `CLAUDE_CONFIG_DIR` (e.2).
- **Never present:** the manifest, the hidden suite, the predicates, the oracle and the graders (a.6 T.5, d.6).

**Not documented in e.6, so the pilot must prove each item before any run depends on it** (decision 15.14):
1. running `claude -p` with this design's flags, instead of the default `claude --dangerously-skip-permissions` (e.6);
2. a custom image or template carrying the pinned toolchain, `coach` and `jq`;
3. one `/login` serving many short-lived sandboxes;
4. removing the default wildcard allowances.

If any of them fails, the harness falls back to (b) only with the owner's explicit agreement.

**If option (b) is used**, these mitigations apply. They narrow the risk but do not remove it:
- **The token is dedicated.** It is a `setup-token` token used only for evaluation, never the developer's main login file. It is injected at run time as an environment variable and is never written to an image layer.
- **Egress is restricted.** Only the model API is reachable, through the dev-container firewall pattern (e.4).
- **Artefacts are scanned.** After every trial, the harness scans transcripts, final trees and results for the token, and for any string with its prefix and length. A hit fails the run and quarantines the artefacts.
- **Revoke and rotate.** e.1 documents no revocation step, so a token that leaks has to stay valid until the owner finds a documented way to revoke it (decision 15.15).

**Refusal.** If the sandbox credential is absent or expired, the task refuses and names the one command that fixes it: under (c), an `sbx` session running `/login`; under (b), `claude setup-token`. The check reports only "present" or "absent" and never prints the credential.

### Grading sandbox and the nonce channel

- **The grading sandbox** is a second fresh container with no network and no credentials. The final tree is copied in, owned by the unprivileged `runner` user. Only that trial's hidden suite is mounted, read-only.
- **The nonce channel.**
  - The hidden harness (a hidden `TestMain` for Go, a reporter module for TS) writes one record per hidden test to an **inherited file descriptor**, each carrying a per-trial nonce.
  - The host generates the nonce and passes it only to that harness.
  - G1 is counted **only** from this channel.
  - stdout, including `go test -json`, is kept for diagnosis and never counted.
  - Residual risk: agent code in the same test process could still target the channel deliberately. The G2 initialisation guard and transcript reading cover that (section 14).
- **The grade is assembled on the host.**

### Images

- There is one pinned base image (or, under option (c), sandbox template) per language, recorded by digest.
- Dependencies are installed at build time, with TS `node_modules` at the instance root (**our choice**). `jq` is pinned in every image.
- The `coach` binary differs by run:
  - **`eval-agent`** builds `coach`, `scripts/coach-self-review.sh` and the rule paragraph from the **working tree**. It builds with `GOOS=linux` and the image's `GOARCH`, and injects the results through a **derived image** (the base digest plus a `COPY`). Their SHA-256s and the derived digest are recorded.
  - **`eval-agent-study`** uses a pinned **released** `coach`.
- **Arm differences.** `coach` is on `PATH` in `coach`. In `generic_review` it sits at a root-owned path outside `PATH`, used only by the yoked hook. It is absent in the other arms. Invoking it from a non-`coach` arm is recorded as leakage.

### Holding the hook fixed

- **Fixed files.** `scripts/coach-self-review.sh` (from "### The check") and the arm's `HOME/.claude/settings.json` are mounted read-only, and their SHA-256s are recorded. `.lousy-agents/coach.json` is present in every arm.
- **The yoked `generic_review` hook.**
  - Its wrapper prepends the root-owned `coach` directory to `PATH` **inside the hook process only**, then runs the coach check silently.
  - It sends the generic message only when the check blocked because of **findings**. It otherwise lets the agent stop.
  - It uses the same `stop_hook_active` guard.
- **Block cause.** Every hook run records its block cause: `findings`, `not_clean` or `both`. A `not_clean` block is an integrity failure in any pilot or `eval-agent` trial. In the study it is recorded and reported.
- **Hook health check** (integrity, run before trials):
  - On the unmodified instance, the hook exits 0, `scope.policy.matches_head` is `true`, and there is no `worktree_scan_incomplete`.
  - On the oracle tree, `coach` yields only the manifest's expected findings.
- **Git-state rules.**
  - Commits are denied in every arm, by `--disallowedTools "Bash(git commit:*)"` and a read-only `pre-commit` that exits 1.
  - Bypasses exist (`-c core.hooksPath=…`, `--no-verify`, `commit-tree`, `update-ref`, `stash`), so at the end of each trial the harness asserts:
    - `HEAD` == `initial_commit_sha`;
    - no new refs and no stash entries;
    - an index consistent with the last hook run's view.
  - Any violation is an integrity failure.

### Agent invocation (the `claude-code` solver)

`claude -p` runs with these flags, checked against `claude --help` for Claude Code 2.1.332:
- `--model <pinned id>`
- `--output-format stream-json`
- `--no-session-persistence`
- `--setting-sources user`, with a fresh `HOME` and `CLAUDE_CONFIG_DIR` (e.2)
- `--strict-mcp-config`, with an empty MCP configuration
- the same `--allowedTools` in every arm
- `--disallowedTools "Bash(git commit:*)"`

`--bare` is not used, for two reasons. It skips "hooks (those defined in settings ...)", and in bare mode "Claude Code never reads OAuth credentials" (e.3). `--max-budget-usd` is not used, because on a subscription dollars are notional. The harness enforces the per-trial token, turn and wall-clock caps instead. It reads them from the `stream-json` events, stops the trial when one is reached, and records which cap it was.

### Exact prompt text

**Feature task family, base (all arms):**

```text
{feature_request}

Work in this repository. Keep the existing tests passing and do not modify existing test files; you may add new test files. Do not commit. Do not ask questions: make reasonable choices and finish.
```

- **`control`, `coach` and `generic_review`** get the base prompt only.
  - `coach` has the Claude Code `Stop` entry from `docs/cli-codesignal.md` "## Agent self-review hook", byte for byte.
  - `generic_review`'s yoked hook message is:

    ```text
    Before you stop, review your uncommitted changes for maintainability problems — complexity, coupling, layering and hidden side effects — and fix any you find, or explain why not, then stop.
    ```

- **`rules_in_prompt`** gets the base prompt followed by:

  ```text
  Before you finish, check your change against these structural rules and fix any violation you introduce:
  {one line per rule: rule_id — what it flags — threshold, generated from docs/cli-codesignal.md "## Rule IDs" and its threshold table at the coach version under test, plus the layer policy's forbidden and required edges}
  ```

**Refactor task family, base (all arms):**

```text
This repository has maintainability problems. Improve its internal structure without changing its behaviour. Keep the existing tests passing and do not modify existing test files. Do not commit. Do not ask questions: make reasonable choices and finish.
```

- **`control`** gets the base prompt only.
- **`coach`** gets the base prompt followed by:

  ```text
  The coach CLI is installed. `coach codesignal --baseline` reports structural findings for the whole repository; `coach codesignal --worktree` reports findings in your uncommitted changes.
  ```

- **`rules_in_prompt`** gets the base prompt followed by the rule paragraph.

### Trial record (JSON, one per trial)

```json
{
  "schema_version": "1",
  "trial_id": "string",
  "run_id": "string",
  "task_family": "feature | refactor",
  "template_id": "string",
  "instance_id": "string",
  "initial_commit_sha": "string",
  "variant": "main | impossible",
  "arm": "control | coach | rules_in_prompt | generic_review | baseline_do_nothing | baseline_delete | baseline_stub | oracle",
  "coach_build": "worktree | merge_base | release",
  "epoch": 1,
  "solver": { "name": "claude-code", "version": "string", "model": "string", "config_sha256": "string", "prompt_sha256": "string", "settings_sha256": "string", "hook_script_sha256": "string" },
  "environment": { "image_digest": "string", "isolation": "docker_sandbox | docker", "runtime": "microvm | runsc | runc", "host_arch": "string", "trial_timeout_s": 0 },
  "coach": { "version": "string", "binary_sha256": "string" },
  "hooks": [{ "event": "Stop", "exit": 0, "block_cause": "none | findings | not_clean | both", "message_sha256": "string", "findings_shown": [{ "rule_id": "string", "path": "string", "subject": "string", "lines": [1, 2] }] }],
  "hook_findings": { "shown": ["key"], "resolved": ["key"], "ignored": ["key"] },
  "integrity": { "hook_health": true, "git_state": true, "leakage": ["string"] },
  "caps": { "tokens": 0, "turns": 0, "wall_seconds": 0 },
  "usage": { "input_tokens": 0, "output_tokens": 0, "turns": 0, "notional_cost_usd": 0, "wall_seconds": 0, "tokens_on_fp": 0 },
  "credential": { "mode": "sbx_host_held | setup_token_in_container", "artefact_scan": "clean | hit" },
  "exit": { "status": 0, "reason": "completed | token_cap | turn_cap | timeout | usage_limit | harness_error" },
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

Adjudication-dependent variables are computed after adjudication, by joining on `dedup_key`.

### Exclusions and transcripts

- A `harness_error` trial is re-run once, then excluded and counted.
- A `token_cap`, `turn_cap` or `timeout` exit is graded as left, and counted in `exit_cap_or_timeout`.
- A `usage_limit` exit (the subscription's five-hour or weekly limit, e.7) is **not graded**. The trial is re-queued under the resume rules (section 10).
- Leakage is reported, not excluded.
- **A `not_clean` hook block in the study.** The treatment was broken for that trial, so the trial becomes `harness_error`, is excluded, and is re-run with the same instance, arm and epoch. If more than the preregistered rate (proposed: 5% of trials in any arm, **our choice**) are affected, the study is invalid: it is recorded, it fails, and it cannot fill the claim row.
- Every exclusion is listed (b.6).
- Before publication, a human reads at least 5 transcripts per arm, plus every `gaming`, G2-failure and leakage trial (a.1, b.4).

### Harness acceptance tests

These are Ginkgo tests, red first. They live behind the build tag `evalharness` and run only through `mise run eval-harness-acceptance`, which passes `-ginkgo.fail-on-empty` and refuses without Docker. Making `cmd/coach-eval` a **separate Go module** is recommended, so its Docker client does not enter the main `go.mod` (**our choice**).

The tests:
- The oracle scores 1; the do-nothing, delete and stub baselines score 0.
- A weakened hidden suite lets the delete baseline pass.
- An added `TestMain` that exits 0 fails G1.
- A tree whose code prints a forged `PASS` line (for example `go test -json`) to stdout still fails G1.
- A grade attempt that reaches the network fails. Grading uses no credential.
- A trial whose agent tries to read `~/.claude`, the macOS Keychain (`security find-generic-password`) and its own environment leaves no credential in any transcript, final tree or result file. The artefact scan must pass, and it must fail on a planted copy of the token.
- In a trial, the developer's user-level hooks, MCP servers and `CLAUDE.md` are not loaded. Each is planted with a marker, and the test asserts that no marker appears in the transcript.
- With `ANTHROPIC_API_KEY` set on the host, a trial still authenticates by the subscription credential, or refuses. It never bills the key.
- A `git commit --no-verify` (or `commit-tree`/`update-ref`) inside a trial is an integrity failure.
- `eval-detection` fails if any finding's path falls outside the fixture.

## 8. Detection acceptance tests (H3)

**Fixture isolation** (required, because `--baseline` scans "every tracked Go/TS/TSX file at HEAD" of the enclosing repository and reads the policy from its top level; `docs/cli-codesignal.md` lines 57 and 131). For each fixture, the harness:
1. copies it into a fresh temporary directory;
2. runs `git init` and makes one neutral commit;
3. runs, from that directory:

```sh
coach codesignal --baseline --format json --no-interactive --fail-on-incomplete-coverage
```

Architecture fixtures add `--project-config .lousy-agents/coach.json`, the fixture's own policy.

**Outcome per row:**

| `expect` | The suite requires |
| --- | --- |
| `fire` | at least one matching finding |
| `must_not_fire` | no matching finding |
| `known_miss` | no matching finding. If one appears, the suite fails with "known miss now detected: promote the row" |
| `known_fp` | a matching finding (a recorded false positive). If none appears, the suite fails with "known false positive gone: promote to must_not_fire". The count of `known_fp` rows is printed on every run |
| `contested` | no assertion; recorded for adjudication |

**Unmatched findings.**
- An unmatched finding keeps the suite red.
- The **only** verdict that clears it is `manifest_gap`, and only together with a new manifest row in the same change, so the finding then matches a row.
- A `false_positive` verdict does **not** clear it: the suite stays red until the rule is fixed, or the site is recorded as a `known_fp` row.
- Fixture adjudications live in `eval/adjudications/fixtures.jsonl`.

**Mutation red tests** (#260 "False-green trap"). These run through `mise run eval-detection-selftest`.
- The task builds `coach` with a **test-only build override**: a build tag that lets the harness set a rule threshold and disable one detector shape.
- It asserts that a lowered `complexity.cognitive_complexity` threshold, and a removed shape, each turn `eval-detection` red.
- The override is a Coach code change, so it needs its own issue. It must be absent from release builds.

Coach's version and configuration are printed with every result (c.5).

## 9. Placement

- **Fixture location.** The candidate in-repository home is `eval/testdata/fixtures/{go,ts,tsx}/<id>/`, each with its own `go.mod` or `package.json`. Whether fixtures live there is decision 15.10, which **must be resolved before any fixture merges**.
  - **What already excludes them.**
    - The Go command skips `testdata` and nested modules (`go list ./...` in a scratch module lists neither).
    - The source-layout check skips `testdata` (`internal/sourcelayout/check.go:22–25`), and so does Go root discovery (`pkg/projectmodel/go_root_discovery_walk.go:94`).
    - Coach's production scope already narrows the self-scan. With `--build-target` set, a Go file that is not reachable is `Excluded`; a TS file outside the `tsconfig` include and exclude rules is not production (`internal/codesignalcli/sourcescope/file_classification.go:10–32`). This repository has no root `tsconfig.json` today.
  - **Not yet established:** whether a self-scan with `--build-target ./...` plus a root `tsconfig.json` that excludes `eval/` leaves the living evaluation's numbers unchanged. That is the first thing to test.
  - **Developer-hook interaction.** A contributor's own `--worktree` hook would flag seeded fixtures they edit, unless that hook uses the same scope.
  - **Maintenance if fixtures live in a separate repository.** Someone must re-pin the fixture SHA in this repository after each fixture change. Fixture adjudications would then live with the fixtures, and `eval-detection` would read a local clone (`EVAL_FIXTURES_DIR`).
- **Code rings.**
  - `cmd/coach-eval` and `internal/evalharness` join the **tooling** ring.
  - `internal/evalgrade` gets its own **grading** ring. It may not import entities, usecases, adapters or app, and no production ring may import it. The graders call the pinned `coach` binary, never `pkg/`.
  - **Planned edit** (owner approval, in the code PR): `.lousy-agents/coach.json` and ADR-007's ring table (`docs/architecture/ADR-007-layered-ports-and-adapters.md`) gain the three prefixes and the grading ring.
- **Committed data.**
  - `eval/protocol/` holds the protocols.
  - `eval/adjudications/fixtures.jsonl` holds fixture verdicts.
  - `eval/census/adjudications.jsonl` holds census verdicts.
  - `eval/census/corpus/` holds the census manifests.
  - `eval/ledger/runs.jsonl` is the run ledger (section 10).
  - None of these are Go or TS files.
- **Private material.** Held-out templates, their hidden suites, hidden test IDs, predicates and oracles live in a separate private repository, pinned by SHA (a.6 T.5, b.5, b.8). The tuning panel's hidden suites may be public (decision 15.13).

## 10. Running it: local mise tasks

Everything runs as local `mise` tasks a developer starts before committing. Each task has a `description` in `mise.toml`. Nothing runs on a schedule, on a shared runner, or in CI (decision 15.11).

### Tasks

| Task | What it runs | Expected wall time and cost (**our estimates**) | What "pass" means before a commit |
| --- | --- | --- | --- |
| `eval-detection` | H3 over every detection fixture, each isolated (section 8), against `coach` built from the working tree. No model, no network | Seconds to low minutes; $0 | **Gating.** Every row outcome holds. This is the pre-commit default |
| `eval-detection-selftest` | The mutation red tests (section 8) | Under a few minutes; $0 | `eval-detection` turns red under each mutation |
| `eval-agent` | **Regression smoke test.** On the tuning panel (12 feature templates, 1 instance, K = 1), `coach` built from the working tree is paired against `coach` built from the merge-base, per template | 24 trials on a cold cache (12 worktree and 12 merge-base), and 12 when the merge-base side is cached. At an assumed 5 min a trial and 4 in parallel, that is about 30 min cold and 15 min cached. Tokens: about 24 × the pilot's mean tokens per trial, which is a placeholder until measured. It fits inside one five-hour usage window (e.7) if the pilot's mean confirms the estimate | **Not gating on results.** It prints the per-template flips in `unadjudicated_clean` and the integrity results. It prints any aggregate effect only with its MDE (about ±45 points at T = 12) beside it. The output is stamped **"development panel — not evidence"** |
| `eval-agent-study` | The preregistered confirmatory run with a pinned released `coach` | About 200 templates × 2 × 3 × 4 arms = 4,800 trials, plus about 800 impossible-variant trials, on placeholder values. At 5 min a trial and 4 in parallel, that is about 117 h of running time. It **will span many five-hour windows and weekly limits** (e.7), probably weeks on one subscription, so the resume semantics are mandatory. Tokens are estimated at preregistration from the pilot | Not gating. The only source of claims. Integrity failures exit 1 |
| `eval-census` | #260's census in the pinned census image (section 11) | Minutes for 6 repositories; $0 | It writes the report. It exits 1 on `analysis_failed` without a disposition, or on an unreachable SHA |
| `eval-harness-acceptance` | The harness acceptance tests (section 7) | Minutes; $0 | Ginkgo pass with `-ginkgo.fail-on-empty` |
| `eval-compare` | Compares two runs, or shows a trend (below) | Seconds; $0 | Informational |

**What `eval-agent` measures.**
- **`unadjudicated_clean`** = G1 ∧ G2 ∧ no **detector candidate**, with no human involved. It is named differently from `clean_success` on purpose.
- **Caching.** The merge-base side, and the optional `control` and `rules_in_prompt` reference arms, are cached by `(coach binary SHA-256, hook script SHA-256, rule paragraph SHA-256, solver config, model, CLI version, image digest, runtime, prompt hash, template)`. A cache hit costs nothing.
- **When a change needs an `eval-agent` run:** changes to the hook script, the rule set or a threshold, or the finding format the hook passes on (`rule_id`, `evidence`, `recommendation`, `location`, `lifecycle`, or how they are rendered). Every other change needs only `eval-detection`.
- **Integrity revalidation.** Every run re-validates the tuning panel's oracle, baselines and hook health. These are scripted and need no model.

**Integrity failures** (exit 1 in `eval-agent` and `eval-agent-study`):
- the oracle scores below 1, or a baseline scores above 0;
- the hook health check fails;
- the hook fires in the wrong arm or not at all;
- a `not_clean` block outside the study;
- a git-state violation;
- an oracle-clone SHA mismatch;
- two `harness_error` results on one trial.

**`eval-agent-study` rules.**
- It refuses on a dirty worktree.
- It refuses unless the caps cover the protocol's full trial count: `EVAL_MAX_TRIALS` ≥ trials, and `EVAL_MAX_TOKENS` ≥ trials × the per-trial token cap.
- It resumes idempotently, keyed by `(instance_id, arm, epoch)`.
- It randomises trial order within template blocks from the protocol seed.
- Every arm runs the identical instance set.
- **Usage limits.** When the subscription's limit is reached mid-run, the run pauses: state `paused_usage_limit` in `run.json`, with the limit message and the time. In-flight trials are re-queued, never graded as partial. `mise run eval-agent-study -- --resume <run_id>` continues from there. A paused run never prints results as if complete.
- An incomplete study (`paused_usage_limit`, a cap reached, missing cells, or an over-threshold `not_clean` rate) cannot fill the claim row.

### Exit codes (all tasks)

| Code | Meaning |
| --- | --- |
| 0 | Pass. For `eval-compare`: compared, and the runs are comparable |
| 1 | A gate failure (`eval-detection`) or an integrity failure (agent tasks, census) |
| 2 | Refusal: a missing or wrong input (below). For `eval-compare`: a run is missing or unreadable |
| 3 | `coach` returned non-zero on a fixture or a census repository |
| 4 | `eval-compare` only: compared, but the runs are not comparable. The report is printed with the reasons |

### Inputs and refusals

Every task refuses (exit 2) with a message naming what is missing. None silently skips or runs a smaller variant.

| Input | Used by |
| --- | --- |
| Docker (`docker info` succeeds) for grading; under option (c), also `sbx` on a supported platform (e.6) | `eval-agent*`, `eval-census`, `eval-harness-acceptance` |
| Node and Coach's TypeScript backend at the versions `mise.toml` pins, for TS fixtures | `eval-detection`, `eval-census` (inside its image) |
| The pinned images, at their recorded digests | `eval-agent*`, `eval-census` |
| `claude` installed, at the recorded version, and logged in to a subscription. The login check reports only present or absent and never prints the credential. The refusal names the fixing command (section 7) | `eval-agent*` |
| The sandbox credential for the chosen isolation option, present and unexpired (section 7) | `eval-agent*` |
| No API-key variables in the agent environment (`ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, `apiKeyHelper`), because in `-p` a key always wins (e.1) | `eval-agent*` |
| Caps: `EVAL_MAX_TRIALS`, `EVAL_MAX_TOKENS`, `EVAL_TRIAL_MAX_TOKENS`, `EVAL_TRIAL_MAX_TURNS` and `EVAL_RUN_MAX_HOURS`. The estimate is printed before any trial, as trials × the per-trial token cap, and trials × the running mean if one exists | `eval-agent*` |
| `EVAL_MODEL` (no default) | `eval-agent*` |
| `EVAL_TRIAL_TIMEOUT` | `eval-agent*` |
| `EVAL_ORACLE_DIR`: a clean local clone at the protocol's pinned SHA, never fetched | `eval-agent-study`. `eval-agent` too when the tuning panel's suites are private |
| The protocol file | `eval-agent-study` |

### How the private oracle reaches a local machine

1. The developer clones the private repository once, at the pinned SHA, with their own Git credentials. The task verifies the clone and never fetches.
2. The host mounts only one trial's hidden suite and predicates, read-only, into that trial's **grading** container.
3. The agent container receives the instance copy and nothing else.

### Results, ledger and comparison

- **Raw output** goes **outside the worktree**, in `$XDG_STATE_HOME/coach-eval/<run_id>/`, defaulting to `~/.local/state/coach-eval/`. No tracked file is ever changed. The directory holds:
  - `run.json`;
  - `trials.jsonl` (section 7);
  - the transcripts and final trees.
- **`run.json`** records:
  - `run_id` and `task`;
  - `coach_sha`, `worktree_dirty`, and the SHA-256 of `git diff HEAD`;
  - `git status --porcelain=v2 -uall`, plus a content hash of the untracked files;
  - the `coach` binary, hook-script and rule-paragraph SHA-256s;
  - the image digests, runtime and host architecture;
  - the private-repository and fixture SHAs;
  - the solver block, model and CLI version;
  - the parameters, every seed, and `trial_timeout_s`;
  - tokens, turns, notional cost and wall time, plus the run state (`complete | paused_usage_limit | cap_reached | invalid`);
  - the results table.
- **Committed ledger.** `eval/ledger/runs.jsonl` is append-only, one line per run: `run.json` without transcripts or trees. Only `eval-agent-study`, `eval-census` and `eval-detection --record-baseline` runs are appended, which gives #260 and the living evaluation a citable history. Appending changes a tracked file, so it happens only in the change that records the run.
- **`mise run eval-compare -- <a> <b>`** reads either a ledger entry or a local run.
  - It shows detection row changes, and paired per-template differences for agent runs (a.7, b.1).
  - Any difference in solver, model, CLI, image digest, **isolation option**, **runtime**, **host architecture**, timeout, prompt hash, private SHA or parameters makes the runs not comparable (exit 4).
  - `eval-compare -- --trend <task>` lists the ledger's runs for a task in order. That is how drift is tracked: by comparing saved results across commits.
- **Verdicts** are reused across runs through `dedup_key`.

## 11. Manifest, protocol and adjudication schemas (shared with #260)

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
    "visible_tests": [{ "path": "string", "sha256": "string" }],
    "private_ref": { "repo": "string", "sha": "string", "path": "string" },
    "loc_band": [0.7, 1.5]
  }
}
```

- **Join key:** `(source.sha, rule_id, path, subject, lines)`, matched by overlap. `fingerprint` is informational.
- **Corpus pinning:** `source.sha` with `include` and `exclude` (c.2, c.3). This repository's census manifest excludes `eval/testdata/**` and `vendor/`, and the census harness filters by `include` and `exclude` **before** sampling.
- **Sampling:** `sample` records #260's seed and its sampling frame.
- **Provenance and edges:** `seed_method` records how each row was made (c.7), and edge rows list the full graph (c.8).

### `eval/protocol/<protocol_id>.json`

```json
{
  "schema_version": "1",
  "protocol_id": "string",
  "committed": "YYYY-MM-DD",
  "hypotheses": ["H4a", "H4b", "H4c", "H2a", "H2b", "H2c"],
  "claim_rule": "intersection_union",
  "alpha": 0.05, "h2_alpha_one_sided": 0.025, "h2_margin": -0.05, "min_effect": 0.15,
  "T": 0, "instances_per_template": 2, "K": 3, "impossible_K": 1,
  "arms": ["control", "coach", "rules_in_prompt", "generic_review"],
  "coach_release": "string", "solver": { "name": "claude-code", "version": "string", "model": "string" },
  "images": { "go": "digest", "ts": "digest" }, "runtime": "runsc | runc",
  "private_repo_sha": "string", "held_out_template_ids": ["string"],
  "seeds": { "instance": 0, "order": 0 },
  "trial_timeout_s": 0, "trial_max_tokens": 0, "trial_max_turns": 0, "not_clean_max_rate": 0.05, "isolation": "docker_sandbox | docker",
  "fallback": { "T": 0, "printed_mde": 0 },
  "exclusions": ["harness_error_twice"],
  "deviations": []
}
```

### `adjudications.jsonl`, append-only, one row per judgement

There are two files: `eval/adjudications/fixtures.jsonl` and `eval/census/adjudications.jsonl`.

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

- **One row per judgement.** Rows are never overwritten (c.1, c.2).
- **Verdicts.** #260's four verdicts are kept. Added here: `manifest_gap` (c.6), and `row_confirmed`/`row_rejected` for fixtures (a.5); rejected rows are kept (c.2).
- **Only humans count** (#260).
- **Blinding.** Raters do not see severity or confidence (c.4), nor the arm or detector (**our choice**).
- **Deduplication.** Each `dedup_key` is judged once (**our choice**).
- **Second rating.** A second human rates every contested and single-rated census finding (c.1), and 20% of H4 candidates and hook findings (**our choice**). Where the two raters differ, the stricter label wins (**our choice**, after a.5).
- **Agreement.** κ is reported per rule. κ < 0.6 marks a figure unreliable (**our choice**; c.9).

### Census

- **Command:** `eval-census` runs `coach-eval census --corpus eval/census/corpus --adjudications eval/census/adjudications.jsonl --out <report>`.
- **Census image.** It runs inside a **pinned census image**. Each repository's dependencies are installed at image build with `--ignore-scripts`, so no untrusted install script runs on the host, and the toolchain is the same on every machine.
- **What is recorded per repository:** the image digest and the resolved compiler and Node versions.
- **Scan command:** `coach codesignal --baseline --format json --no-interactive --fail-on-incomplete-coverage`, plus `--project-config` where the repository has a policy.
- **`analysis_status`**, per rule and per repository, is one of:
  - `measured`;
  - `not_exercised`, with the gap;
  - `not_adjudicable_only`;
  - `analysis_failed`, with the exit and the `coverage` evidence;
  - `out_of_scope`, with the owner's decision.
- **Exclusions from the census.** A failed repository never makes a rule `not_exercised`. Fixture rows never enter census figures.
- **Two-repository red test** (#260). With one rule at 12 adjudications and 0 false positives, one rule with zero findings, and one repository that fails analysis, the test asserts:
  - "0 of 12" ≠ `not_exercised`;
  - the failed repository is `analysis_failed`;
  - a mutated threshold turns the test red, using section 8's override.

## 12. Mapping to #260

| #260 criterion (abridged) | What this design supplies |
| --- | --- |
| Per rule: repos, SHAs, counts, verdict counts | `census_repo` manifests and adjudications, through `eval-census` |
| Zero findings → `not exercised`, with the gap | `not_exercised`; `analysis_failed` kept separate; fixtures excluded |
| Human, date, fingerprint, rationale | `rater`, `date`, `target.fingerprint` (informational), `rationale` |
| Empty evidence → `not_adjudicable` | That verdict. Whether the density rules still emit empty evidence needs a fresh check (`rule_*_density.go:29` copies `finding.Evidence`) |
| Below threshold → disposition | Not supplied |
| PRD updated | Not supplied. #260's "Success Signals" section no longer exists in `docs/product/prd.md`, whose gate is now §12, so #260 needs retargeting |
| Security recall from seeded shapes | Detection fixtures: per-shape recall and known misses for TOCTOU (c.6, c.7) |
| Pilot engineer action | Not supplied |
| Re-run reproduces | `source.sha`, `include`/`exclude`, the census image digest and the Coach hash (c.3, c.5) |
| Unpinnable repo excluded | `sha` is required |
| Harness false-green test | Section 11's two-repository red test; `eval-detection-selftest` |

Including `architecture.*` in #260 is an owner decision (15.9).

## 13. What each result can feed

**A confirmatory H4/H2 result is necessary, but not sufficient,** for the living evaluation's agent self-review gate (PRD §12: "Until the evaluation carries a claim for agent self-review, that job is not judged"). The gate stays unjudged until a restamp also runs the documented hook on a fresh real repository.

**The claim row.** The new row on `docs/product/evaluations/codesignal-pilot-readiness.html` is "Agent self-review: controlled effect". Its fields are:
- the wording (decision 15.5);
- the model and the date;
- T, and the MDE if the fallback was used;
- the effect and its 95% CI per comparator;
- the token ratio against `control` and `generic_review`;
- the HEAD.

The gates on that row:
- It is written only if the token gate passes (decision 15.4).
- Only a complete `eval-agent-study` fills it. **Claims come only from `eval-agent-study`**, and `eval-agent` output never appears.

| Result | Can feed | Cannot feed |
| --- | --- | --- |
| H4 with H2 (`eval-agent-study`) | The claim row, scoped to the rule families Coach checks, fixture evidence and one model | The repeat-use test (PRD §4: "a hook's scans do not" count) |
| H3 (`eval-detection`) | The pre-commit gate; per-shape recall and known misses for #260 and #205 | Precision, or the security precision gate, which needs "a recorded precision review on a named corpus" (PRD §12) |
| H1 (secondary) | Exploratory product evidence | Any release claim |
| `eval-agent` | A developer's local regression signal; drift, by comparing saved results across commits | Any claim |

## 13a. What this establishes toward authority

- **H3 establishes conformance:** Coach does what its own contract says on seeded shapes.
- **H4 establishes adherence:** agents act on Coach's advice in a way that changes outcomes in the rule families it checks.
- **#260 establishes precision** on real code.
- **None of them establishes that following Coach makes code better in general.**

**Exploratory construct-validity measure.** After each feature trial, a **fresh** agent with no hook in any arm is given a hidden **follow-on feature task** on that trial's final tree. `followon_g1` and `followon_cost` are compared across arms. If the coach arm's trees are easier to extend, that is evidence that the issues Coach flags matter. It is exploratory and never part of a claim.

**No result here licenses "authority" language to customers.**

## 14. Threats to validity

Grouped under the four headings in b.7, plus contamination.

**Construct**
- **H3 is conformance, not precision** (c.8).
- **H4 covers only the rule families Coach checks:** layering, complexity, hidden input mutation, check-then-act and constructor coupling. It says nothing about other defects. `introduced_defect_count` is reported beside `completed`, and `clean_success` requires G1 ∧ G2, so trading or avoiding work shows.
- **Seeded is not natural** (c.6, c.7).
- **Cognitive complexity is only partly validated** (d.1, d.2).
- **Grader circularity** (d.7) is mitigated by the non-Coach detectors and `rules_in_prompt`.
- **Manifest errors** (c.6) are mitigated by validation, `manifest_gap` and the oracles.

**Internal**
- The extra turn is isolated by a yoked `generic_review` that blocks on findings only.
- Tokens and turns are reported with every effect (b.5).
- Drift is controlled by pinning the solver, model, CLI, image, runtime, Coach hash, hook and settings.
- Leakage is recorded.
- Faked grading is countered by the nonce channel, the unprivileged runner and the G2 guard. Code running in-process could still target the channel; transcript reading covers that.
- Git-state bypasses are checked at the end of every trial.
- **Credential exposure.** Under option (c), the credential never enters the agent's boundary (e.6). Under option (b), it does (e.4), and only the dedicated token, the egress allowlist and the artefact scan narrow the risk.
- **Usage-limit pauses** stretch a study over days. The pinned CLI and model are re-checked on every resume, and a run whose model or CLI changed mid-run is invalid.

**External**
- Fixtures are simpler than real repositories (c.5). Claims stop at the templates' languages, sizes and trap kinds (b.7).
- One model and one solver (b.7).

**Conclusion**
- The template is the unit, with t at T − 1 degrees of freedom (a.7, b.1). This departs from Miller's cluster-adjusted method (section 1).
- Holm across H4a–c, plus the intersection-union rule.
- Sizing is for joint power, and the MDE is printed (section 5).
- The protocol is preregistered (b.6).

**Contamination**
- Held-out templates are fresh and private (a.5, b.5, b.8).
- No hints appear in fixture text (b.8).
- Hidden tests are checked against a wrong patch (b.8).
- The tuning panel is never evidence.

## 15. Owner decisions, with recommendations

1. **Confirmatory task family.** *Recommend:* the self-review hook on feature tasks.
2. **Model.** *Recommend:* pin the current flagship model, with one smaller model as an exploratory replication. Model IDs go in the protocol.
3. **Adjudicator capacity**, likely the binding constraint together with usage limits. *Our estimate* at T ≈ 200:
   - template validation ≈ 200 × 2 × 2 × 1.5 h ≈ 1,200 h;
   - candidate and hook-finding judgements ≈ 4,800 trials × 2 × 50% dedup × 1.2 × 2 min ≈ 190 h.

   *Recommend:* name the humans and their hours before preregistration. The answer feeds decision 12.
4. **Token gate (hard).** *Recommend:* the claim row is written only if `coach`'s **tokens per G1 ∧ G2-passing trial** are at most 1.3× `control`'s (**our choice**). The raw tokens-per-trial ratio is printed beside it.
5. **Permitted wording.** It must describe the primary measure. *Recommend:* "In a controlled evaluation on T seeded Go and TypeScript feature templates with [model] on [date], X pp more feature changes were completed cleanly (95% CI a–b) when the agent ran Coach's self-review hook than with [comparator]: hidden tests pass, no degenerate change, and no adjudicated structural issue in the rule families Coach checks (layering, complexity, hidden input mutation, check-then-act, constructor coupling)."
   - The introduced-issue rate is always reported beside it, as a secondary result.
   - Never say "defects" without "structural", and never say "prevents" or "finds all".
   - The owner has the final say.
6. **T and the caps.** *Recommend:* pilot, then decision 12. Set standing `eval-agent` caps per developer, and caps for each `eval-agent-study` separately, in trials, tokens and hours.
7. **Thresholds marked our choice.** *Recommend:* ratify before preregistration:
   - H2: margin −0.05, at one-sided 0.025;
   - 15 points minimum effect;
   - 90% per-test power;
   - the line band [0.7, 1.5];
   - K = 3 with 2 instances per template, and K = 1 on impossible variants;
   - κ ≥ 0.6, with 20% double rating;
   - template size 300–1500 lines;
   - the per-trial token, turn and wall-clock caps;
   - the 5% `not_clean` invalidation rate;
   - the 1.3× gate;
   - refactor T = 24.
8. **`n/a` coverage cells.** *Recommend:* accept look-alike `must_not_fire` rows.
9. **`architecture.*` in #260.** *Recommend:* include them, now that #253 is closed.
10. **Where fixtures live. Resolve this before any fixture merges.** *Recommend:* first test an in-repository production-scope self-scan (`--build-target ./...` plus a root `tsconfig.json` excluding `eval/`), and check that the living evaluation's numbers are unchanged and contributors' hooks use the same scope. If that holds, keep fixtures in `eval/testdata/`. Only if it fails, use a separate public fixture repository with the re-pinning workflow in section 9.
11. **CI.** *Recommend:* out of scope. `eval-detection` could later become a CI leaf.
12. **Post-pilot go/no-go.** Choose (a), (b) or (c) from section 5, using the pilot's measured ω², tokens and hours, the usage limits (e.7) and the capacity from decision 3. *Recommend:* (b) over (c) if the fallback MDE is at most 25 points (**our choice**), because (c) leaves the PRD gate unjudged.
13. **Oracle access and whose subscription.** *Recommend:*
    - `eval-agent-study` runs on the owner's subscription;
    - private-repository access only for people who run it;
    - **only `eval-detection` is expected of contributors.**

    The tuning panel's hidden suites can be public, because `eval-agent` is a smoke test on a panel that is never evidence. A contributor who chooses to run `eval-agent` does so on their own subscription and usage limits.
14. **Isolation option.** *Recommend:* option (c), Docker Sandboxes (**our choice**), once the pilot proves the four mechanisms e.6 does not document (section 7). If they cannot be proven, the choice is (b) with its stated risk, or no agentic runs. The owner chooses; the harness does not fall back silently.
15. **Terms of use and the credential lifecycle (blocking).**
    - Consumer Terms §3.7 prohibits accessing the Services "through automated or non-human means, whether through a bot, script, or otherwise", "Except when you are accessing our Services via an Anthropic API Key or where we otherwise explicitly permit it" (e.7).
    - On the other side, Anthropic documents `claude setup-token` as a subscription token "for CI and scripts" (e.2).
    - Whether unattended evaluation runs on a subscription are permitted is **the owner's call to confirm with Anthropic**, and this design does not assume an answer. No `eval-agent*` run should happen until it is settled.
    - Also undocumented: how to revoke a `setup-token` token (e.1). That decides whether option (b)'s revoke-and-rotate step exists at all.

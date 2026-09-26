# Coach PRD - Local CodeSignal Pilot (v3)

This document supersedes the v2 Platform Groundwork PRD from 20 July 2026.

The v2 document described an async analysis platform with a hosted service. That hosted service is not the current product. The product that ships today is a local command-line tool.

## Evidence cutoff

- Latest published release: `v0.8.0` (26 September 2026).
- Leave-pilot evaluation: [`evaluations/codesignal-pilot-readiness.html`](evaluations/codesignal-pilot-readiness.html). Pass 20. Masthead date 26 September 2026. Named evaluation revision is `6c8970d`. That pass is a focused restamp, not a full corpus replay.
- This document is audited against `main` at `4e1c379`. Do not treat the evaluation revision as the current default-branch revision.
- Pilot-exit epic: [#282](https://github.com/lousy-agents/coach/issues/282). Standing decision: stay in pilot. Read the live epic for the closed-child count.
- Command contract: [`docs/cli-codesignal.md`](../cli-codesignal.md).

A behavior is **implemented** only when a passing acceptance test locks it. A release note is not enough. A specification without a passing test is a **bet**.

Do not treat an empty signal set as a safety result.

## 1. Problem

Engineers now produce large code changes with assistants. Many of those changes are hard to review and hard to trust.

Typical defects stay hidden until a person reads the diff:

- hidden mutation of caller input
- tight coupling in constructors
- dense structure that hides change scope
- imports that cross a layer the team already named
- a check-then-act race on a filesystem path

Current tools do not close this gap for a private, local workflow.

- Public review bots write comments that other people can see.
- Linters enforce style and local rules. They do not report change lifecycle.
- Continuous integration gates pass or fail. They do not explain structural risk.

The engineer needs a private report over recent committed work. The current product reports deterministic rule results only.

Behavioral test gaps remain a real problem. This era does not claim to detect them.

## 2. Who this era serves

The user is the project owner or a small set of engineers who scan their own Git checkout.

The user runs Coach on a repository they already have on disk. The default command path does not call GitHub. The default command path does not call a model.

This era has no manager view. This era has no team rollout. This era has no anonymous audience.

## 3. Current workaround

Without Coach the user does one or more of these actions:

- read every assistant-generated diff by hand
- paste fragments into a chat model and accept an ungrounded answer
- wait for a public review bot to comment on an open pull request
- run a general linter and treat a clean run as a review

Those workarounds leak privacy, mix opinion with evidence, or miss structural defects that span files.

## 4. Desired outcome

Desired outcome: a pilot user who did not write Coach runs a second scan after the first report, without a request from the product owner.

The outcome is not a count of rules. The outcome is not a hosted service. The outcome is not a merge gate.

Kill tests that end this bet live only in section 11.

Repeat use is observed through out-of-band pilot check-ins. The CLI sends no telemetry.

If a kill test fails, stay in pilot. Do not name a new audience.

## 5. Scope this era

In scope:

- local `coach codesignal` on a Git checkout
- Go, TypeScript, and TSX source that is already committed
- file-local structural signals
- opt-in project policy that the user writes and commits
- consented setup for a missing TypeScript compiler when a terminal is present
- an honest report when analysis is incomplete

Out of scope in this era:

- a hosted Coach service as the default product
- a web user interface
- harness hooks as a supported product
- AWS or other cloud deployment
- new analysis languages
- a review-readiness verdict
- model judgment on the default command path
- live GitHub reads or writes on the default command path

## 6. Non-goals

These limits do not expire with this era unless a later document replaces them.

- Coach does not score people. Coach does not rank productivity.
- Coach does not approve a merge. Coach does not replace continuous integration.
- Coach does not claim complete security coverage.
- Coach does not treat a missing signal as a safety verdict.
- Coach does not write comments, checks, or other GitHub objects in this era.
- Coach does not invent architecture. The user declares roots, layers, and forbidden imports.
- Coach does not guess willingness to pay or market size.
- Coach does not detect behavioral test gaps in this era.

## 7. Trust rules

- Self-serve: the user scans a local checkout they already control.
- Provenance on the default path: every finding is a deterministic rule result. Model output is not part of that path.
- Coverage honesty: an empty signal set means no matched rule for the analyzed inputs. Absence of a finding is not proof of safety.
- Advisory security: a security-category rule describes a narrow, reproducible pattern. It is not runtime proof.
- Consent for mutation: compiler or package setup that changes the machine needs a terminal and an explicit confirm step.
- Default file-local analysis does not need a network connection. Project-policy setup may use the network after the user confirms.
- Absence of an architecture signal is not compliance.

## 8. Requirements

These requirements describe the local pilot product. They do not specify storage, queues, or cloud layout.

### 8.1 Analysis contract

WHEN the Coach CLI analyzes source for a CodeSignal report, THE Coach CLI shall read committed Git objects at the named revision.

WHEN the user requests a baseline scan, THE Coach CLI shall analyze committed Go, TypeScript, and TSX files that pass the active scope filter at `HEAD`.

WHEN the user omits `--scope`, THE Coach CLI shall use production scope.

WHEN the user requests a diff scan against a Git ref, THE Coach CLI shall analyze the change between the merge base of that ref and `HEAD`.

IF the working tree contains uncommitted source edits, THEN THE Coach CLI shall ignore those edits in the CodeSignal report.

WHEN a diff scan skips a path that is not Go, TypeScript, or TSX, THE Coach CLI shall record a per-path `unsupported_language` diagnostic.

WHEN a baseline scan skips paths that are not Go, TypeScript, or TSX, THE Coach CLI shall record `coverage.unsupported` grouped by extension.

WHERE the user requests TypeScript project readiness, THE Coach CLI shall read the host compiler and worktree manifests. Those reads are readiness checks. They are not source analysis.

WHEN a TypeScript project scan produces a report, THE Coach CLI shall include `project_provenance` and `project_scope`.

WHEN a TypeScript project scan sees relevant dirty worktree changes, THE Coach CLI shall emit `worktree_changes_not_analyzed`.

THE Coach CLI shall complete a successful analysis with process status `0` even when the report contains signals, except where `--fail-on-incomplete-coverage` applies.

### 8.2 Findings and lifecycle

THE Coach CLI shall include a rule identity and a rule version on every signal in the JSON report.

WHEN a diff scan sees a Git added file, THE Coach CLI shall classify matching signals as `introduced`.

WHEN a signal exists at the merge base and at `HEAD`, THE Coach CLI shall classify that signal as `existing`.

WHEN a signal exists at the merge base and not at `HEAD`, THE Coach CLI shall classify that signal as `resolved`.

WHEN a diff scan cannot determine old-path continuity for a rename or copy, THE Coach CLI shall classify matching signals as `unknown` and record a continuity diagnostic.

WHEN the CLI cannot otherwise classify a signal, THE Coach CLI shall classify that signal as `unknown`. THE Coach CLI shall not treat `unknown` as a new defect.

WHILE a baseline scan runs, THE Coach CLI shall classify signals as `baseline`. THE Coach CLI shall not emit `resolved` signals during a baseline scan.

THE Coach CLI shall count every JSON signal in exactly one summary field among `introduced_signals`, `existing_signals`, `resolved_signals`, `baseline_signals`, and `unknown_signals`.

### 8.3 Honesty under failure

IF analysis of a requested path is incomplete, THEN THE Coach CLI shall not print an unqualified all-clear.

IF the user supplies an invalid project policy, THEN THE Coach CLI shall exit with status `2`, write the error to standard error, and write no report to standard output.

IF a TypeScript project scan cannot resolve a supported compiler or a supported host Node major, and no interactive setup offer runs, THEN THE Coach CLI shall exit with status `2` and emit no CodeSignal report.

WHERE the environment has a non-empty `CI` variable, or the user passes `--no-interactive`, THE Coach CLI shall not open a prompt.

WHERE the user passes `--fail-on-incomplete-coverage` and required project coverage is incomplete on any analyzed side, THE Coach CLI shall write the report to stdout and exit `3`.

### 8.4 Project policy

WHERE the user passes a committed project policy, THE Coach CLI shall evaluate only the layers and forbidden imports that the policy names.

THE Coach CLI shall not invent layers or forbidden imports.

WHEN the user sets project language and omits a project policy, THE Coach CLI shall keep file-local analysis.

WHEN a committed policy is valid, THE Coach CLI shall emit a project-policy report that is distinct from the file-local report.

WHEN a committed policy is valid and an import edge violates a named forbidden pair, THE Coach CLI shall emit `architecture.layer_violation`.

WHERE the policy names a required layer and a matching path from a handler-shaped source to a pinned sink exists, THE Coach CLI shall emit `architecture.layer_bypass`.

IF the policy file is not committed at the analyzed revision, THEN THE Coach CLI shall not treat the worktree copy as the scan policy.

WHEN a TypeScript project scan completes with no configured covered match, THE Coach CLI shall state that and list `project_next_actions`.

### 8.5 TypeScript project setup

WHEN a TypeScript scan on a controlling terminal passes `--project-config` and that policy file exists neither at the analyzed revision nor in the worktree, THE Coach CLI shall offer guided policy authoring.

IF the policy file exists uncommitted in the worktree, THEN THE Coach CLI shall exit with status `2`, emit no report, and name `git add` as the fix.

WHEN that scan authoring path runs, THE Coach CLI shall write the candidate to standard output, reject `--output`, and write no repository file.

WHEN the candidate is written, THE Coach CLI shall exit with status `2`, emit no report, and instruct the user to save, review, commit, and rerun.

WHEN the user runs standalone TypeScript policy authoring on a controlling terminal, THE Coach CLI shall write no repository file until the user confirms.

WHEN a TypeScript scan omits `--project-config`, THE Coach CLI shall stay file-local and open no policy authoring.

WHEN a TypeScript project scan runs on a controlling terminal, the policy is committed, a supported compiler is missing, and at least one setup choice is executable, THE Coach CLI shall offer a bounded setup menu and require a second confirm step before any install.

IF no setup choice is executable, THEN THE Coach CLI shall open no prompt, exit with status `2`, and name why each choice was withheld.

IF the user cancels setup or the install fails, THEN THE Coach CLI shall exit with status `2` and emit no CodeSignal report.

IF setup succeeds, THEN THE Coach CLI shall rerun readiness. THE Coach CLI shall resume the original scan only when the readiness rerun reports no gap.

THE setup preview shall show executable, arguments, working directory, expected disk changes, network use, and timeout before the confirm step.

IF the install fails, THEN tracked files are unchanged and partial toolchain state may remain.

WHEN the user requests TypeScript readiness, THE Coach CLI shall report status, checks, and gaps, mutate nothing, and exit `0`. `--check-project` is that readiness command.

THE Coach CLI shall not install Node.

THE Coach CLI shall treat host Node majors `24` and `26` as the only supported majors.

### 8.6 Privacy and writes

THE Coach CLI shall not write comments, checks, or other objects to GitHub on the default command path.

THE Coach CLI shall not call a model on the default command path.

WHILE the user runs file-local analysis without a project policy, THE Coach CLI shall not require a network connection.

## 9. Shipped behavior

Status uses the evidence rule in the cutoff section.

| Behavior | Evidence |
| --- | --- |
| File-local structural analysis for Go, TypeScript, and TSX | Implemented in the published CLI through `v0.8.0`. Locked by `cmd/coach/acceptance_test.go`. Rule list lives in `docs/cli-codesignal.md`. |
| Diff scan and baseline scan with lifecycle fields | Implemented. Git added files count as `introduced` as of `v0.7.0` ([#262](https://github.com/lousy-agents/coach/issues/262)). Rename or copy without old-path continuity stays `unknown`. Locked by `cmd/coach/acceptance_test.go` and `cmd/coach/baseline_acceptance_test.go`. |
| Opt-in project policy and `architecture.layer_violation` | Implemented from `v0.4.0`. Coach does not guess architecture. A language flag without a policy stays file-local. Locked by `cmd/coach/project_go_backend_acceptance_test.go`. |
| `architecture.layer_bypass` on a handler-shaped source to a pinned sink | Implemented for Go (`go-layer-bypass-registry@1`) and TypeScript (`ts-layer-bypass-registry@1`). Locked by `cmd/coach/project_go_backend_acceptance_test.go` and `cmd/coach/project_ts_backend_acceptance_test.go`. Absence of a finding is not compliance. |
| TypeScript readiness check | Implemented from `v0.4.0`. `--check-project` reports status, checks, and gaps, mutates nothing, and exits `0`. Locked by `cmd/coach/project_readiness_acceptance_test.go`. |
| Guided policy authoring | Implemented from `v0.5.0`. Scan authoring needs `--project-config` and a terminal. The scan prints a candidate on standard output. Locked by `cmd/coach/project_ts_scan_policy_authoring_acceptance_test.go`. |
| Consented TypeScript compiler setup | Implemented as `--prepare-compiler` in `v0.6.0` and as a scan-time offer in `v0.7.0`. The standalone flag is mise-only. Locked by `cmd/coach/project_ts_setup_execute_acceptance_test.go` and `cmd/coach/project_ts_setup_preview_acceptance_test.go`. |
| Unattended path that refuses prompts | Implemented as `--no-interactive` and a non-empty `CI` variable in `v0.7.0`. Locked by `cmd/coach/project_ts_scan_policy_authoring_acceptance_test.go`. |
| JSON signals carry rule identity and rule version | Implemented. Default text output still omits `rule_id` and `severity` for file-local signals ([#279](https://github.com/lousy-agents/coach/issues/279)). Locked by `pkg/codesignal/report_shape_acceptance_test.go`. |
| TypeScript project `project_provenance`, `project_scope`, `worktree_changes_not_analyzed`, complete no-match `project_next_actions`, and `--fail-on-incomplete-coverage` exit `3` | Implemented in `v0.8.0`. Locked by `cmd/coach/project_scope_cli_acceptance_test.go`, `cmd/coach/project_provenance_worktree_acceptance_test.go`, and `cmd/coach/fail_on_incomplete_coverage_acceptance_test.go`. |
| Local API, worker, and OAuth lab | Implemented and CI-verified. Not the default product. Path A stub smoke is locked by `cmd/platform-smoke/smoke_acceptance_test.go`. README names this path as a separate lab. Paths B and C are operator labs. |
| Cloud deploy and SGLang | Bet. Not the current product. |
| Model judgment on the default path | Bet. Not present on the default CLI path. |

## 10. Open bets and parked work

Stay in pilot while [#282](https://github.com/lousy-agents/coach/issues/282) remains open. Closed children of that epic do not end the pilot by themselves. Re-run the affected evaluation claim against current `HEAD` before any leave-pilot decision.

Known remaining risks from the evaluation and the epic:

- Diff mode is not ready to sell as an advisory pull-request check to a named audience.
- A TypeScript journey on an arbitrary foreign repository is not a packaged product ([#280](https://github.com/lousy-agents/coach/issues/280)).
- Suppression of accepted findings is still missing ([#50](https://github.com/lousy-agents/coach/issues/50)).
- A precision census for security-category rules is still missing ([#260](https://github.com/lousy-agents/coach/issues/260), [#205](https://github.com/lousy-agents/coach/issues/205)).
- Default text output still omits `rule_id` and `severity` for file-local signals ([#279](https://github.com/lousy-agents/coach/issues/279)).
- Vendored dependencies inflate production-scope noise ([#277](https://github.com/lousy-agents/coach/issues/277)).
- Density rules emit one signal per site ([#261](https://github.com/lousy-agents/coach/issues/261)).
- Out-parameter precision on hidden-input mutation is still missing ([#268](https://github.com/lousy-agents/coach/issues/268)).
- Severity spread and magnitude ranking are still missing ([#272](https://github.com/lousy-agents/coach/issues/272)).

Parked on purpose:

- Review-readiness digest with a readiness verdict. The verdict had no defensible decision rule.
- Behavioral test-gap detection as a primary claim.
- Model judgment, rubric scoring, and provenance split on a hosted path.
- Harness hooks and a web user interface.

If a later path adds model judgments, each finding must stay tagged as deterministic or agent. Agent output must not hide a deterministic finding. A schema failure on the agent path must still return the deterministic report. That contract is not a requirement of the default CLI path.

## 11. Risks as tests

Do not treat this table as a substitute for customer evidence. Each row is a test. A failed test blocks expansion of audience. This is the only kill-test table.

| Risk | Question | Cheapest test that kills the bet |
| --- | --- | --- |
| Value | Will a pilot user choose this report over hand review or a chat paste? | After one report, the user does not run a second scan and cannot name one action they took. |
| Usability | Can a first-time TypeScript user complete readiness and a first scan from the README? | The user hits a hidden step, a prompt with no terminal, or a false clean result. |
| Feasibility | Can the shipped analyzer examine the change the user asked about? | A requested path is skipped and the text still reads as complete and clean. |
| Business viability | Does this stay a private advisory tool? | A GitHub write, a person score, or a merge block lands on the default path. |

## 12. Release criteria for leaving pilot

The governing leave-pilot rule is owner decision 4: the living evaluation's restamp against `HEAD`. Stay in pilot until a named audience can be told an honest job on a fresh CLI corpus. Closed children of [#282](https://github.com/lousy-agents/coach/issues/282) do not end the pilot. GitHub `CLOSED` is not the gate.

Leave pilot only when all of these statements are true:

- A new evaluation restamp against current `HEAD` agrees that a named audience can be told an honest job.
- A first-time user can follow the published README and obtain either a report or a clear next action. The user does not receive a false all-clear.
- A user who did not write Coach runs a second scan without a prompt from the owner.
- Security-category claims have a recorded precision review on a named corpus. The review includes true positives, false positives, and known misses.
- The default command path still writes nothing to GitHub and still scores no person.

If those statements are not true, the correct product action is to stay in pilot.

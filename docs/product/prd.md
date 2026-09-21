# Coach PRD — Local CodeSignal Pilot (v3)

This document supersedes the v2 Platform Groundwork PRD from 20 July 2026.

The v2 document described an async analysis platform with a hosted application programming interface. That work is not the current product. The product that ships today is a local command-line tool.

## Evidence cutoff

- Latest published release: `v0.7.0` (19 September 2026).
- Leave-pilot evaluation: [`evaluations/codesignal-pilot-readiness.html`](evaluations/codesignal-pilot-readiness.html), last reviewed 19 September 2026 at `HEAD` `a27f470`.
- Pilot-exit epic: [#282](https://github.com/lousy-agents/coach/issues/282). The standing decision is to stay in pilot.
- Command contract: [`docs/cli-codesignal.md`](../cli-codesignal.md).

A behavior is **implemented** only when a passing acceptance test locks it. A release note is not enough. A specification without a passing test is a **bet**.

Do not treat an empty signal set as a safety result.

## 1. Problem

Engineers now produce large code changes with assistants. Many of those changes are hard to review and hard to trust.

Typical defects stay hidden until a person reads the diff:

- hidden mutation of caller input
- tight coupling in constructors
- dense structure that hides change scope
- tests that copy implementation instead of behavior
- imports that cross a layer the team already named
- a check-then-act race on a filesystem path

Current tools do not close this gap for a private, local workflow.

- Public review bots write comments that other people can see.
- Linters enforce style and local rules. They do not report change lifecycle.
- Continuous integration gates pass or fail. They do not explain structural risk.

The engineer needs a private report over recent work. The report must mark each finding as a reproducible rule result or as model judgment. The report must not pretend that absence of a finding is proof of safety.

## 2. Who this era serves

The user is the project owner or a small set of engineers who scan their own Git checkout.

The user runs Coach on a repository they already have on disk. The default command path does not call GitHub. The default command path does not call a model.

This era has no manager view. This era has no team roll-out. This era has no anonymous audience.

## 3. Current workaround

Without Coach the user does one or more of these actions:

- read every assistant-generated diff by hand
- paste fragments into a chat model and hope the answer is grounded
- wait for a public review bot to comment on an open pull request
- run a general linter and treat a clean run as a review

Those workarounds leak privacy, mix opinion with evidence, or miss structural defects that span files.

## 4. Outcome and how this product dies

Desired outcome: a pilot engineer runs a second scan after the first report, without a request from the product owner.

The outcome is **not** a count of rules. The outcome is **not** a hosted service. The outcome is **not** a merge gate.

Named tests that kill the current bet:

- Value: after a first report, pilot engineers do not run Coach again.
- Usability: an engineer who did not write Coach cannot start a TypeScript project scan from the published README without a hidden step.
- Feasibility: an incomplete analysis still prints an unqualified all-clear.
- Business viability: Coach writes to GitHub, scores a person, or blocks a merge.

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

- a hosted Coach application programming interface as the default product
- a web user interface
- harness hooks as a supported product
- AWS or other cloud deployment
- new analysis languages
- a review-readiness verdict
- live GitHub reads or writes on the default command path

## 6. Non-goals

These limits do not expire with this era unless a later document replaces them.

- Coach shall not score people. Coach shall not rank productivity.
- Coach shall not approve a merge. Coach shall not replace continuous integration.
- Coach shall not claim complete security coverage.
- Coach shall not treat a missing signal as a safety verdict.
- Coach shall not write comments, checks, or other GitHub objects in this era.
- Coach shall not invent architecture. The user declares roots, layers, and forbidden imports.
- Coach shall not guess willingness to pay or market size.

## 7. Trust rules

- Self-serve: the user scans a local checkout they already control.
- Provenance: a deterministic finding stays a deterministic finding. Model output shall not hide it.
- Coverage honesty: an empty signal set means no matched rule for the analyzed inputs.
- Advisory security: a security-category rule describes a narrow, reproducible pattern. It is not runtime proof.
- Consent for mutation: compiler or package setup that changes the machine needs a terminal and an explicit confirm step.
- Degrade in public: if a later agent path fails schema checks, the deterministic report still ships and the failure is a diagnostic.

## 8. Requirements

These requirements describe the local pilot product. They do not specify storage, queues, or cloud layout.

### 8.1 Analysis contract

The Coach CLI shall analyze only committed Git objects at the named revision.

WHEN the user requests a baseline scan, THE Coach CLI shall examine tracked Go, TypeScript, and TSX files at `HEAD`.

WHEN the user requests a diff scan against a Git ref, THE Coach CLI shall compare `HEAD` to that ref.

IF the working tree contains uncommitted edits, THEN THE Coach CLI shall ignore those edits.

IF a path uses a language other than Go, TypeScript, or TSX, THEN THE Coach CLI shall skip that path and record an `unsupported_language` diagnostic.

THE Coach CLI shall complete a successful analysis with process status `0` even when the report contains signals.

### 8.2 Findings and lifecycle

THE Coach CLI shall tag every signal with a rule identity and a rule version.

WHEN a file is added in a diff scan, THE Coach CLI shall classify matching signals as `introduced`.

WHEN a signal exists at the merge base and at `HEAD`, THE Coach CLI shall classify that signal as `existing`.

WHEN a signal exists at the merge base and not at `HEAD`, THE Coach CLI shall classify that signal as `resolved`.

WHEN the CLI cannot classify a signal, THE Coach CLI shall classify that signal as `unknown` and shall not treat `unknown` as a new defect.

WHILE a baseline scan runs, THE Coach CLI shall classify signals as `baseline` and shall not emit `resolved` signals.

THE Coach CLI shall count every signal in exactly one summary field among `introduced_signals`, `existing_signals`, `resolved_signals`, `baseline_signals`, and `unknown_signals`.

### 8.3 Honesty under failure

IF analysis of a requested path is incomplete, THEN THE Coach CLI shall not print an unqualified all-clear.

IF the user supplies an invalid project policy, THEN THE Coach CLI shall exit with status `2`, write the error to standard error, and write no report to standard output.

IF a TypeScript project scan cannot resolve a supported compiler or a supported host Node major, THEN THE Coach CLI shall exit with status `2` and shall not emit a CodeSignal report.

WHERE the environment has a non-empty `CI` variable, or the user passes `--no-interactive`, THE Coach CLI shall not open a prompt.

### 8.4 Project policy

WHERE the user passes a committed project policy, THE Coach CLI shall evaluate only the layers and forbidden imports that the policy names.

THE Coach CLI shall not invent layers or forbidden imports.

WHEN a committed policy is valid, THE Coach CLI shall emit report schema version `2` and may emit `architecture.layer_violation`.

WHERE the policy names a required layer, THE Coach CLI shall evaluate the narrow handler-to-SQL bypass registry and may emit `architecture.layer_bypass`.

IF the policy file is not committed at the analyzed revision, THEN THE Coach CLI shall not treat the worktree copy as the scan policy.

### 8.5 TypeScript project setup

WHEN a TypeScript project scan runs on a controlling terminal and the committed policy is missing, THE Coach CLI shall offer guided policy authoring and shall write no file until the user confirms.

WHEN a TypeScript project scan runs on a controlling terminal, the policy is committed, and a supported compiler is missing, THE Coach CLI shall offer a bounded setup menu and shall require a second confirm step before any install.

IF the user cancels setup or the install fails, THEN THE Coach CLI shall exit with status `2` and shall not emit a CodeSignal report.

IF setup succeeds, THEN THE Coach CLI shall rerun readiness and shall resume the original scan only when readiness is `ready` or `ready_with_limits`.

THE Coach CLI shall not install Node.

THE Coach CLI shall treat host Node majors `24` and `26` as the only supported majors.

### 8.6 Privacy and writes

THE Coach CLI shall not write comments, checks, or other objects to GitHub on the default command path.

THE Coach CLI shall not call a model on the default command path.

WHILE the user runs file-local analysis without a project policy, THE Coach CLI shall not require a network connection.

### 8.7 Provenance for a future agent path

WHERE an optional local platform path adds model judgments, THE product shall tag each finding as `deterministic` or `agent`.

IF a model judgment fails schema validation, THEN THE product shall still return the deterministic report and shall record the failure as a diagnostic.

THE product shall not let agent output suppress a deterministic finding.

## 9. Shipped behavior

Status uses the evidence rule in the cutoff section.

| Behavior | Evidence |
| --- | --- |
| File-local structural analysis for Go, TypeScript, and TSX | Implemented in the published CLI through `v0.7.0`. Rule list lives in `docs/cli-codesignal.md`. |
| Diff scan and baseline scan with lifecycle fields | Implemented. Added files count as `introduced` as of `v0.7.0` ([#262](https://github.com/lousy-agents/coach/issues/262)). |
| Opt-in project policy and `architecture.layer_violation` | Implemented from `v0.4.0`. Coach does not guess architecture. |
| `architecture.layer_bypass` on the narrow handler-to-SQL registry | Implemented for Go and TypeScript. Absence of a finding is not compliance. |
| TypeScript readiness check and guided policy authoring | Implemented from `v0.5.0`. Authoring needs a terminal. |
| Consented TypeScript compiler setup | Implemented as `--prepare-compiler` in `v0.6.0` and as a scan-time offer in `v0.7.0`. Mise-only on the standalone flag. |
| Unattended path that refuses prompts | Implemented as `--no-interactive` and a non-empty `CI` variable in `v0.7.0`. |
| Local platform lab with a stub model smoke | Specified and partially verified. README names this path as a separate lab, not the default product. |
| Hosted application programming interface, OAuth identity, and cloud deploy | Bet. Not the current product. |
| Versioned LLM-as-judge rubrics on the default path | Bet. Not present on the default CLI path. |

## 10. Open bets and parked work

Stay in pilot while [#282](https://github.com/lousy-agents/coach/issues/282) remains open. Closed children of that epic do not end the pilot by themselves. Re-run the affected evaluation claim against `HEAD` before any leave-pilot decision.

Known remaining risks from the evaluation and the epic:

- Diff mode is not ready to sell as an advisory pull-request check to a named audience.
- A TypeScript journey on an arbitrary foreign repository is not a packaged product ([#280](https://github.com/lousy-agents/coach/issues/280)).
- Suppression of accepted findings is still missing ([#50](https://github.com/lousy-agents/coach/issues/50)).
- A precision census for security-category rules is still missing ([#260](https://github.com/lousy-agents/coach/issues/260), [#205](https://github.com/lousy-agents/coach/issues/205)).

Parked on purpose:

- Review-readiness digest with a readiness verdict. The verdict had no defensible decision rule.
- Behavioral test-gap detection as a primary claim. That work needs rubric infrastructure that this era does not ship on the default path.
- Harness hooks and a web user interface. Those surfaces may consume a later interface. They are not this era.

## 11. Risks as tests

Do not treat this table as a substitute for customer evidence. Each row is a test. A failed test blocks expansion of audience.

| Risk | Question | Cheapest test that kills the bet |
| --- | --- | --- |
| Value | Will a pilot engineer choose this report over hand review or a chat paste? | After one report, the engineer does not run a second scan and cannot name one action they took. |
| Usability | Can a first-time TypeScript user complete readiness and a first scan from the README? | The user hits a hidden step, a prompt with no terminal, or a false clean result. |
| Feasibility | Can the shipped analyzer examine the change the user asked about? | A requested path is skipped and the text still reads as complete and clean. |
| Business viability | Does this stay a private advisory tool? | A GitHub write, a person score, or a merge block lands on the default path. |

## 12. Release criteria for leaving pilot

Leave pilot only when all of these statements are true:

- Every child of [#282](https://github.com/lousy-agents/coach/issues/282) is closed **and** the evaluation restamp against `HEAD` agrees.
- A first-time user can follow the published README and obtain either a report or a clear next action. The user does not receive a false all-clear.
- At least one pilot engineer outside the author set runs a second scan without a prompt from the owner.
- Security-category claims have a recorded precision review on a named corpus. The review includes true positives, false positives, and known misses.
- The default command path still writes nothing to GitHub and still scores no person.

If those statements are not true, the correct product action is to stay in pilot.

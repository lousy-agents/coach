# Coach PRD - Local CodeSignal Pilot (v3)

This document supersedes the v2 Platform Groundwork PRD from 20 July 2026.

The v2 document described an async analysis platform with a hosted service. That hosted service is not the current product. The product that ships today is a local command-line tool.

## Evidence cutoff

- Latest published release: `v0.8.0` (26 September 2026).
- Leave-pilot evaluation: [`evaluations/codesignal-pilot-readiness.html`](evaluations/codesignal-pilot-readiness.html). Pass 28. Masthead date 6 October 2026. Named evaluation revision is `92fabd2`, the merge base; the pass ran at `e431621`. That pass is a focused re-run, not a full corpus replay.
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
- Go, TypeScript, and TSX source: committed, or uncommitted when the user passes `--worktree`; and the project policy file as it is on disk
- file-local structural signals
- project policy that the user writes, committed or not; file-local analysis when none is selected
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
- Stability. Coach is not yet stable. Its JSON schema, fingerprints, rule versions, and policy schema may change between releases. The owner decides case by case whether a change is announced in release notes.
- Source: a committed scan reads committed source. The project policy is read from disk, as ESLint and Biome read their configuration. The comparison with `HEAD` is Coach's own addition, not parity with those tools.
- Policy disclosure: every scan reports the policy path and digest it used, and whether the policy selected from disk matches the one selected at `HEAD` (section 8.4). The report does not compare with an older analyzed revision. JSON `scope.policy` is always present. A text line appears when a policy is selected or the two differ. A difference appears as a WARNING, and it qualifies any clean result.

## 8. Requirements

These requirements describe the local pilot product. They do not specify storage, queues, or cloud layout.

### 8.1 Analysis contract

WHEN the Coach CLI analyzes source for a CodeSignal report without `--worktree`, THE Coach CLI shall read committed Git objects at the named revision.

WHEN the user requests a baseline scan without `--worktree`, THE Coach CLI shall analyze committed Go, TypeScript, and TSX files that pass the active scope filter at `HEAD`.

WHEN the user omits `--scope`, THE Coach CLI shall use production scope.

WHEN the user requests a diff scan against a Git ref without `--worktree`, THE Coach CLI shall analyze the change between the merge base of that ref and `HEAD`.

IF the working tree contains uncommitted source edits and `--worktree` is not passed, THEN THE Coach CLI shall ignore those edits in the CodeSignal report and shall say so in the report.

WHEN a diff scan skips a path that is not Go, TypeScript, or TSX, THE Coach CLI shall record a per-path `unsupported_language` diagnostic.

WHEN a baseline scan skips paths that are not Go, TypeScript, or TSX, THE Coach CLI shall record `coverage.unsupported` grouped by extension.

WHERE the user requests TypeScript project readiness, THE Coach CLI shall read the host compiler and worktree manifests. Those reads are readiness checks. They are not source analysis.

WHEN a TypeScript project scan produces a report, THE Coach CLI shall include `project_provenance` and `project_scope`.

WHEN a TypeScript project scan without `--worktree` sees relevant dirty worktree changes, THE Coach CLI shall emit `worktree_report_reflects_committed_head`. File-local scans without `--worktree` emit `worktree_changes_not_analyzed` for dirty supported-language files.

WHEN `--worktree` is passed, THE Coach CLI shall analyze the committed revision under analysis with each uncommitted supported-language source file, and each uncommitted project configuration file the analysis reads, replaced by its bytes on disk. Project configuration files are `tsconfig*.json` and `package.json` for TypeScript, and `go.mod` and `go.work` for Go. Staged content that differs from disk shall be replaced by the disk bytes.

WHEN `--worktree` is passed, THE Coach CLI shall include untracked supported-language files that Git does not ignore, shall treat a deleted tracked file as absent rather than empty, and shall apply the same ignore rules as committed mode.

WHEN `--worktree` is passed, THE Coach CLI shall read the same overlay for file-local and project analysis, and shall treat an untracked file as wholly changed in diff line ranges.

WHEN `--worktree` is passed, THE Coach CLI shall report, in `scope.worktree`, the `revision` the overlay sits on, the `modified`, `added`, `deleted`, and `untracked` counts of distinct paths listed by Git status, and the `config_files` read from disk, and shall state the same counts in one text line under the scope header. Each path counts in one category, checked in this order: untracked, deleted, added, modified. An unmerged path counts as modified, because HEAD and the disk both hold a version of it; a path both sides deleted (`DD`) is gone from disk and counts as deleted. A rename counts as one deleted path and one added path. A rename's source path written again as a new file is a modification of the file `HEAD` holds, not an addition, so its existing findings are not `introduced`; it still counts once, as untracked. An `AD` path, staged as added and then deleted from the working tree, is in neither `HEAD` nor the tree, and counts nowhere. An `RD` rename, staged and then its new path deleted from the working tree, counts as one deleted path, its source. A `CD` copy, staged and then the copy deleted from the working tree, keeps its source and counts nothing for the copy. Paths hidden from Git status are not counted; they are reported under `worktree_scan_incomplete`. `scope.worktree` is present only when `--worktree` is passed. Under `--worktree`, `tracked_files_discovered` also counts the untracked and staged new files the scan discovers, and the text reports that count as `files discovered (tracked and new)`. Committed-mode output keeps its existing label.

WHEN `--worktree` is passed, `--scope`, `unsupported_language`, and `coverage.unsupported` apply as they do without it.

WHEN `--worktree` is passed, THE Coach CLI shall not emit `worktree_report_reflects_committed_head` or `worktree_changes_not_analyzed` for a file the overlay read. It shall emit `worktree_changes_not_analyzed`, naming the path, for any dirty supported-language file the overlay did not read, except a symbolic link, which produces `worktree_symlink_skipped`. A path left unmerged by a conflict is not a new file, whatever its status code: it is disclosed as a file the scan did not analyze, not analyzed with its conflict markers. It shall not emit `worktree_report_reflects_committed_head` under `--worktree`.

IF a worktree file cannot be read, changes while it is being read, or is a symbolic link, THEN THE Coach CLI shall record a diagnostic that names the path, and shall not print an unqualified all-clear. A symbolic link produces `worktree_symlink_skipped`.

THE Coach CLI shall complete a successful analysis with process status `0` even when the report contains signals, except where `--fail-on-incomplete-coverage` applies.

### 8.2 Findings and lifecycle

THE Coach CLI shall include a rule identity and a rule version on every signal in the JSON report.

WHEN a diff scan sees a Git added file, THE Coach CLI shall classify matching signals as `introduced`.

WHEN a signal exists at the merge base and at `HEAD`, THE Coach CLI shall classify that signal as `existing`.

WHEN a signal exists at the merge base and not at `HEAD`, THE Coach CLI shall classify that signal as `resolved`.

WHEN a diff scan cannot determine old-path continuity for a rename or copy, THE Coach CLI shall classify matching signals as `unknown` and record a continuity diagnostic.

WHEN a `--base` comparison is degraded because either side is incomplete, THE Coach CLI shall record a diagnostic naming the affected path together with machine-readable `side` (`head` or `base`) and `revision` fields identifying which comparison side and revision caused the degradation.

WHEN a `--base` comparison's lifecycle or counts are degraded by an incomplete comparison side, THE Coach CLI shall report the affected lifecycle as `unknown` and the affected counts as indeterminate rather than asserting an improvement or a regression.

WHEN a `--base` comparison would classify a project finding as `introduced` while it touches a rename/copy new path, or as `resolved` while it touches the old path (through a location or the importer or importee in its identity), and old-path continuity was not determined, THE Coach CLI shall classify that finding as `unknown`, exclude it from the introduced and resolved counts, and record a diagnostic naming that path with its comparison `side` and `revision`.

WHEN the CLI cannot otherwise classify a signal, THE Coach CLI shall classify that signal as `unknown`. THE Coach CLI shall not treat `unknown` as a new defect.

WHILE a baseline scan runs, THE Coach CLI shall classify signals as `baseline`. THE Coach CLI shall not emit `resolved` signals during a baseline scan.

THE Coach CLI shall count every JSON signal in exactly one summary field among `introduced_signals`, `existing_signals`, `resolved_signals`, `baseline_signals`, and `unknown_signals`.

WHEN `--worktree` is passed without `--base` or `--baseline`, THE Coach CLI shall compare the worktree overlay with `HEAD`. A signal absent at `HEAD` is `introduced`, and a signal present at `HEAD` but absent from the overlay is `resolved`.

WHEN `--worktree` is passed with `--base <ref>`, THE Coach CLI shall compare the worktree overlay with the merge-base of `<ref>` and `HEAD`, using the rules for committed diff scans.

WHEN `--worktree` is passed with `--base <ref>` and a committed rename's new path is deleted from the working tree, THE Coach CLI shall classify the findings the merge base holds at the old name as `resolved`, as when the deletion is committed, and shall not report `continuity_not_determined` for the new path.

WHEN `--worktree` is passed with `--baseline`, THE Coach CLI shall classify signals as `baseline`.

IN EVERY MODE, the base side and the head side of a comparison use the same project policy, as section 8.4 selects it.

A finding that changes only because the policy changed shall be classified as `existing`. When the policy on disk differs from `HEAD`, the disclosure in section 7 applies. A policy change committed in the change under review is not disclosed by Coach. CI enforces the policy committed in that change, and that change's policy edit is reviewed like an ESLint configuration change.

### 8.3 Honesty under failure

WHEN `--worktree` is passed and the working tree holds a state the scan does not read, THE Coach CLI shall emit `worktree_scan_incomplete` and shall not print an unqualified all-clear. The states are sparse checkout, a path marked skip-worktree or assume-unchanged, a dirty submodule, an untracked nested repository, and a directory git could not open, such as an untracked directory without read permission. Git leaves such a directory out of the status without failing. The diagnostic's message names each state present, names the directory, and does not relay git's stderr. Any other warning git prints on stderr while reading the status, which the scan does not recognize, also fails closed under `worktree_scan_incomplete`, with a generic message ("git printed a warning ... does not recognize"). A directory whose name holds a control character is shown quoted and escaped, never dropped. Under `--worktree`, a verdict additionally counts as incomplete coverage when part of the working tree could not be read, and `--fail-on-incomplete-coverage` exits `3` for it. Those cases are `worktree_changes_not_analyzed`, `worktree_symlink_skipped`, `worktree_status_check_failed`, and `worktree_scan_incomplete`. `worktree_status_check_failed` means that a sparse, skip-worktree, gitlink or snapshot read failed under `--worktree`; the committed scan's dirty-tree check also uses it. A failed `git status` itself is an operational error, not a verdict: the CLI exits `1`, writes "coach: listing working tree changes: ..." to standard error, and prints no verdict. `unsupported_language` and `continuity_not_determined` still qualify the verdict text, but they exit `0`, as a committed scan does.

IF analysis of a requested path is incomplete, THEN THE Coach CLI shall not print an unqualified all-clear.

IF the user supplies an invalid project policy, THEN THE Coach CLI shall exit with status `2`, write the error to standard error, and write no report to standard output.

IF a TypeScript project scan cannot resolve a supported compiler or a supported host Node major, and no interactive setup offer runs, THEN THE Coach CLI shall exit with status `2` and emit no CodeSignal report.

WHERE the environment has a non-empty `CI` variable, or the user passes `--no-interactive`, THE Coach CLI shall not open a prompt.

WHERE the user passes `--fail-on-incomplete-coverage` and required project coverage is incomplete on any analyzed side, THE Coach CLI shall write the report to stdout and exit `3`.

WHEN the selected project policy differs from the policy selected at `HEAD`, THE Coach CLI shall state the difference in the report and shall qualify any clean result.

WHILE the project policy is read from the commit, WHEN `--worktree` is passed and the selected policy file (`.lousy-agents/coach.json`, or the deprecated `project.json`) has uncommitted changes, THE Coach CLI shall emit `worktree_changes_not_analyzed` naming the policy path, with the message "the project policy is read from the commit at <rev>; --worktree does not read an uncommitted policy yet", shall qualify the verdict, and shall exit `3` under `--fail-on-incomplete-coverage`. Policy-from-disk removes this requirement.

WHILE the project policy is read from the commit, WHEN `--worktree` is passed and the project policy file (`.lousy-agents/coach.json`, or the deprecated `project.json`) is untracked, THE Coach CLI shall emit `worktree_changes_not_analyzed` naming the policy path, with the message "the project policy at <path> is not committed; --worktree reads the policy from the commit at <rev> and does not read an uncommitted policy yet", shall qualify the verdict, and shall exit `3` under `--fail-on-incomplete-coverage`. Policy-from-disk removes this requirement.

WHILE the project policy is read from the commit, WHEN `--worktree` is passed and any status entry whose path or source path is a project policy file (`.lousy-agents/coach.json` or `project.json`) is present, whatever its status code (untracked, staged, intent-to-add, renamed into or out of the policy path, or edited), THE Coach CLI shall name that path in `worktree_changes_not_analyzed`, shall qualify the verdict, and shall exit `3` under `--fail-on-incomplete-coverage`. The selected policy keeps the message "the project policy is read from the commit at <rev>; --worktree does not read an uncommitted policy yet"; any other policy file says that it is not committed, or that it has uncommitted changes. Policy-from-disk removes this requirement.

WHEN `--worktree` is passed and any worktree overlay read is incomplete, THE Coach CLI shall qualify the verdict and shall not print an unqualified all-clear.

### 8.4 Project policy

WHERE the user passes a project policy, THE Coach CLI shall evaluate only the layers and forbidden imports that the policy names.

THE Coach CLI shall not invent layers or forbidden imports.

WHEN the user omits `--project-config` and a `.lousy-agents/coach.json` on disk is schema-valid, THE Coach CLI shall use that policy and emit a project-policy report.

WHEN the user omits `--project-config`, no `.lousy-agents/coach.json` exists on disk, and a schema-valid root `project.json` exists on disk, THE Coach CLI shall use `project.json` and emit `project_config_deprecated`.

WHEN the user omits `--project-config`, and neither a `.lousy-agents/coach.json` nor a schema-valid root `project.json` exists on disk, THE Coach CLI shall keep file-local analysis.

IF a `.lousy-agents/coach.json` on disk fails schema validation and `--project-config` is omitted, THEN THE Coach CLI shall not read `project.json`. Except where section 8.5 offers guided policy authoring, THE Coach CLI shall exit with status `2`, write no report to standard output, and name `project_config_invalid` and the path on standard error.

WHEN a scan selects the root `project.json` fallback on a controlling terminal, `.lousy-agents/coach.json` does not exist, and prompts are not disabled, THE Coach CLI shall ask once on standard error, default No, whether to copy that file. It shall write nothing unless the user answers Yes. It shall never overwrite an existing canonical file or delete `project.json`.

WHEN a policy is valid, THE Coach CLI shall emit a project-policy report that is distinct from the file-local report.

WHEN a policy is valid and an import edge violates a named forbidden pair, THE Coach CLI shall emit `architecture.layer_violation`.

WHERE the policy names a required layer and a matching path from a handler-shaped source to a pinned sink exists, THE Coach CLI shall emit `architecture.layer_bypass`.

The project policy is the file on disk, committed or not. The report discloses whether it matches the policy selected at `HEAD` (section 7).

WHEN a TypeScript project scan completes with no configured covered match, THE Coach CLI shall state that and list `project_next_actions`.

WHEN the user passes `--project-config <path>` and that path does not exist on disk, THE Coach CLI shall exit with status `2`, write no report to standard output, and name the path and `project_config_missing` on standard error. It shall not fall back to file-local analysis or to another policy. On a controlling terminal, for a TypeScript scan, section 8.5 offers guided policy authoring instead of the scan, and that scan also exits with status `2`.

IN EVERY SCAN, THE Coach CLI shall report the policy path and digest it used, and whether the policy selected from disk matches the policy selected at `HEAD`, as section 7 requires. Two policies match when they have the same path and the same digest.

### 8.5 TypeScript project setup

WHEN a TypeScript scan on a controlling terminal passes `--project-config` and that policy file does not exist on disk, THE Coach CLI shall offer guided policy authoring, and the scan shall exit with status `2` as section 8.4 requires.

WHEN the policy file exists on disk but is not committed, THE Coach CLI shall use it and disclose that it differs from `HEAD` (section 7).

WHEN that scan authoring path runs, THE Coach CLI shall write the candidate to standard output, reject `--output`, and write no repository file.

WHEN the candidate is written, THE Coach CLI shall exit with status `2`, emit no report, and instruct the user to save, review, commit, and rerun.

WHEN the user runs standalone TypeScript policy authoring on a controlling terminal, THE Coach CLI shall write no repository file until the user confirms.

WHEN a TypeScript scan omits `--project-config` and section 8.4 selects no policy, THE Coach CLI shall stay file-local and open no policy authoring.

WHEN a TypeScript scan omits `--project-config` and section 8.4 selects a policy, THE Coach CLI shall apply the same readiness, setup, and policy rules as for that path passed explicitly.

WHEN a TypeScript scan on a controlling terminal omits `--project-config`, prompts are not disabled, and a `.lousy-agents/coach.json` on disk fails schema validation, THE Coach CLI shall offer guided policy authoring instead of the scan. THE Coach CLI shall write the candidate to standard output, write no repository file, exit with status `2`, emit no report, and name `.lousy-agents/coach.json` as the file the candidate replaces.

WHEN a TypeScript scan on a controlling terminal omits `--project-config`, prompts are not disabled, and the selected `.lousy-agents/coach.json` is valid but declares no layers, THE Coach CLI shall finish the scan and render the report, then offer guided policy authoring when the scan exits `0`. THE Coach CLI shall print the offer and any candidate on standard error, write no repository file, and keep the exit status of the scan.

IF prompts are disabled or no controlling terminal is available, THEN THE Coach CLI shall open no guided policy authoring for a discovered policy. An invalid `.lousy-agents/coach.json` still exits as section 8.4 says.

WHEN `--suggest-project-config` runs without `--output`, THE Coach CLI shall write the candidate create-only to `.lousy-agents/coach.json`. The Go form also prints the candidate on standard output. The TypeScript form writes only after the user confirms. An existing destination is refused and not overwritten.

WHEN a TypeScript project scan runs on a controlling terminal, a policy is selected, a supported compiler is missing, and at least one setup choice is executable, THE Coach CLI shall offer a bounded setup menu and require a second confirm step before any install.

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
| File-local structural analysis for Go, TypeScript, and TSX | Implemented in the published CLI through `v0.8.0`. Locked by the `cmd/coach/codesignal_*_acceptance_test.go` specs. Rule list lives in `docs/cli-codesignal.md`. |
| Diff scan and baseline scan with lifecycle fields | Implemented. Git added files count as `introduced` as of `v0.7.0` ([#262](https://github.com/lousy-agents/coach/issues/262)). Rename or copy without old-path continuity stays `unknown`. Locked by `cmd/coach/codesignal_signal_lifecycle_acceptance_test.go`, `cmd/coach/codesignal_rename_copy_acceptance_test.go`, and `cmd/coach/baseline_acceptance_test.go`. |
| Project policy discovery and `architecture.layer_violation` | Implemented from `v0.4.0`. Discovery of `.lousy-agents/coach.json` and the root `project.json` fallback is **planned until #512 merges**, then implemented. Coach does not guess architecture. Without a selected policy, scans stay file-local. Locked by `cmd/coach/project_go_backend_acceptance_test.go` and, once #512 merges, `cmd/coach/project_config_discovery_acceptance_test.go`. The discovery specs replace the removed `cmd/coach/project_go_policy_opt_in_acceptance_test.go` pins. |
| `architecture.layer_bypass` on a handler-shaped source to a pinned sink | Implemented for Go (`go-layer-bypass-registry@1`) and TypeScript (`ts-layer-bypass-registry@1`). Locked by `cmd/coach/project_go_layer_bypass_acceptance_test.go` and `cmd/coach/project_ts_backend_acceptance_test.go`. Absence of a finding is not compliance. |
| TypeScript readiness check | Implemented from `v0.4.0`. `--check-project` reports status, checks, and gaps, mutates nothing, and exits `0`. Locked by `cmd/coach/project_readiness_acceptance_test.go`. |
| Guided policy authoring | Implemented from `v0.5.0` for a scan whose explicit `--project-config` names no policy. Authoring on a discovered `.lousy-agents/coach.json` that is schema-invalid, or valid with no layers, is **planned until #512 merges**, then implemented. An invalid file gets the offer instead of the scan, and a layerless one gets it after the scan. Every scan authoring path needs a terminal and writes no repository file. Locked by `cmd/coach/project_ts_scan_policy_authoring_acceptance_test.go` and, once #512 merges, `cmd/coach/project_config_discovered_authoring_acceptance_test.go`. |
| Consented TypeScript compiler setup | Implemented as `--prepare-compiler` in `v0.6.0` and as a scan-time offer in `v0.7.0`. The standalone flag is mise-only. Locked by `cmd/coach/project_ts_setup_execute_acceptance_test.go` and `cmd/coach/project_ts_setup_preview_acceptance_test.go`. |
| Unattended path that refuses prompts | Implemented as `--no-interactive` and a non-empty `CI` variable in `v0.7.0`. Locked by `cmd/coach/project_ts_scan_policy_authoring_acceptance_test.go`. |
| JSON signals carry rule identity and rule version | JSON `rule_id` and `rule_version` are implemented; `pkg/codesignal/report_shape_acceptance_test.go` locks the JSON key shape. Default text output prints `rule_id` and `severity` on every file-local and project finding, which ships in the release that carries this PR; `cmd/coach/text_finding_identity_acceptance_test.go` locks that text claim. `subject` is not printed and `source_scope` is printed but not collapsed ([#279](https://github.com/lousy-agents/coach/issues/279)). |
| TypeScript project `project_provenance`, `project_scope`, `worktree_report_reflects_committed_head`, complete no-match `project_next_actions`, and `--fail-on-incomplete-coverage` exit `3`; file-local `worktree_changes_not_analyzed` and `worktree_not_clean` | `project_provenance`, `project_scope`, `project_next_actions`, and `--fail-on-incomplete-coverage` implemented in `v0.8.0`. `worktree_report_reflects_committed_head`, `worktree_changes_not_analyzed`, and `worktree_not_clean` ship in the release that carries this PR. Locked by `cmd/coach/project_scope_cli_acceptance_test.go`, `cmd/coach/project_provenance_worktree_acceptance_test.go`, `cmd/coach/working_tree_disclosure_acceptance_test.go`, and `cmd/coach/fail_on_incomplete_coverage_acceptance_test.go`. |
| Policy read from disk, with disclosure | Planned. Becomes implemented only when acceptance tests on main lock it. Replaces the committed-policy read and the `project_config_uncommitted` diagnostic. |
| Worktree scanning (`--worktree`) | Experimental. The flag is accepted but hidden from `--help`, and it is not yet in the README or user documentation. Run it from the repository root. A repository's clean filter can still run when `git status` must compare same-size content; no flag disables it. Ships in the release that carries this change: `--worktree` alone compares the working tree with `HEAD`, and `--base <ref>` or `--baseline` selects another comparison. Supported-language source and project configuration files are read from the working tree, including new and untracked files; a deleted file is absent; a symbolic link is not followed and is named; a file the scan cannot read is named, and its message gives the cause, such as permission denied, not a regular file, over the size limit, a failed read, or a name on disk that differs from the name Git records; `scope.worktree` reports the revision, the modified, added, deleted and untracked counts, and the configuration files read from disk. The `AD`, `RD` and `CD` cases of those counts are locked by `cmd/coach/worktree_staged_then_deleted_acceptance_test.go`. A sparse checkout, skip-worktree or assume-unchanged entry, a dirty submodule, an untracked nested repository, and a directory git could not open qualify the verdict as a whole, through `worktree_scan_incomplete`; the nested-repository case is locked by `cmd/coach/worktree_nested_repository_acceptance_test.go`. Under `--fail-on-incomplete-coverage`, the verdict exits `3` for the four incomplete-read kinds in section 8.3 and for incomplete required project coverage. Still planned: naming sparse-checkout, skip-worktree and assume-unchanged paths individually with a `worktree_state_unsupported` reason, in place of the run-level qualification; naming submodule and nested-repository paths individually; and a machine-readable `reason` field on unreadable-file diagnostics. |
| Explicit `--project-config` that does not exist on disk | Planned. Exit `2` with `project_config_missing`, with no file-local fallback. |
| Severity escalation, magnitude ranking, and a narrowed view of a long report | Cognitive complexity and nesting depth emit `high` at twice their threshold and `medium` from the threshold up; branch density stays `medium` because its metric is a whole-file total. Signals sort by lifecycle group, severity, confidence, and path; within each such tier the findings of one metric rule are reordered by metric divided by threshold, highest first. Signals without a metric keep their path position and different rules are never compared by magnitude (within-rule ranking is the decision [#272](https://github.com/lousy-agents/coach/issues/272) left open: it keeps the path order for signals without a magnitude and avoids comparing unlike metrics). `--min-severity` and `--top` narrow the rendered signals in that order, state how many were withheld, leave `summary` and `coverage` describing the full analysis, and never change the exit status. Escalation is part of the deterministic core, so coach-api baseline jobs also carry the escalated severity. Ships in the release that carries this PR ([#272](https://github.com/lousy-agents/coach/issues/272)). Locked by `pkg/codesignal/severity_escalation_acceptance_test.go`, `pkg/codesignal/magnitude_ranking_acceptance_test.go`, `pkg/codesignal/signal_view_acceptance_test.go`, `cmd/coach/min_severity_acceptance_test.go`, `cmd/coach/top_cap_acceptance_test.go`, and `cmd/coach/narrowing_acceptance_test.go`. |
| Local API, worker, and OAuth lab | Implemented and CI-verified. Not the default product. Path A stub smoke is locked by `cmd/platform-smoke/smoke_acceptance_test.go`. README names this path as a separate lab. Paths B and C are operator labs. |
| Cloud deploy and SGLang | Bet. Not the current product. |
| Model judgment on the default path | Bet. Not present on the default CLI path. |

## 10. Open bets and parked work

Stay in pilot until every statement in section 12 is true. [#282](https://github.com/lousy-agents/coach/issues/282) tracks the gaps. Its state is not the gate.

Known remaining risks from the evaluation and the epic:

- Diff mode is not ready to sell as an advisory pull-request check to a named audience.
- A TypeScript journey on an arbitrary foreign repository is not a packaged product ([#280](https://github.com/lousy-agents/coach/issues/280)).
- Suppression of accepted findings is still missing ([#50](https://github.com/lousy-agents/coach/issues/50)).
- A precision census for security-category rules is still missing ([#260](https://github.com/lousy-agents/coach/issues/260), [#205](https://github.com/lousy-agents/coach/issues/205)).
- Default text output omits `subject` for file-local signals and does not collapse `source_scope` ([#279](https://github.com/lousy-agents/coach/issues/279)).
- Vendored dependencies inflate production-scope noise ([#277](https://github.com/lousy-agents/coach/issues/277)).
- Density rules emit one signal per site ([#261](https://github.com/lousy-agents/coach/issues/261)).
- Out-parameter precision on hidden-input mutation is still missing ([#268](https://github.com/lousy-agents/coach/issues/268)).
- A first-party severity spread is observed on issue [#272](https://github.com/lousy-agents/coach/issues/272)'s own corpus, this repository at `19b6628`: 237 findings across 264 analyzed files, `high` 10, `medium` 168, `low` 59, all 10 `high` being `complexity.cognitive_complexity` in first-party code (the largest is `cognitive_complexity=116`), as re-run in Pass 28 with a binary built at `e431621`. The same binary reports only 4 `medium` findings on today's tree, so the spread is not observed there and that tree is not representative; `learn-go-with-tests` at `4675d96` carries its 10 `high` findings in vendored code. Whether the order is worth reading first is unjudged, because no pilot engineer's reading or acting has been observed. That belongs to the pilot-corpus precision census ([#260](https://github.com/lousy-agents/coach/issues/260), whose acceptance criterion records whether an engineer acted on, deferred, or abandoned findings), and [#277](https://github.com/lousy-agents/coach/issues/277) keeps vendored noise. [#272](https://github.com/lousy-agents/coach/issues/272) closed as the mechanism.
- A committed scan's result depends on the project policy file on disk, as with ESLint and Biome configuration. The report says whether that file matches the one selected at `HEAD`. A local edit can change a local run's result. CI enforces the policy committed in the change under test. A change that edits the policy is reviewed like an ESLint configuration change.
- A worktree scan is a local, pre-commit check. It is not a pull-request check.
- `--worktree` reads the project policy from the commit. An uncommitted policy file, whether edited or untracked, is disclosed and qualifies the verdict, but it is not applied until the policy-from-disk slice lands.
- `project_provenance.package_manager` is read from `HEAD`, even when a listed `package.json` is read from disk. Threading the overlay through the package-manager read is a follow-up ([#608](https://github.com/lousy-agents/coach/issues/608)).
- Under `--worktree`, the in-process line diff can mark a different line as changed than git does on repetitive text, about 7% on a repetitive corpus. `changed` is a field on each signal, and the diff may mark a neighbouring signal `changed` when a function is inserted between two others. It over-marks only, and never under-marks, apart from line-ending-only changes ([#611](https://github.com/lousy-agents/coach/issues/611)).
- `--worktree` must run from the repository root.
- The flag is hidden from `--help` while it is experimental, and it is not yet in the README or user documentation.
- An unstaged `mv` shows as a deleted path plus an untracked new file.
- A tracked file under `node_modules` that is edited is not overlaid. It is named as unanalyzed and exits `3` under `--fail-on-incomplete-coverage`.
- A change that only converts line endings is not marked changed when `core.autocrlf` is off. The scan compares line content, not line endings, because a CRLF checkout must not read as an edit of every line.
- A repository's clean filter can still run when `git status` must compare same-size content. No flag disables it. The scan reads file bytes itself, not through git's content filters.
- Under `--worktree`, `worktree_not_clean` reads "working-tree changes this scan did not read: <paths>" rather than "working tree is not clean: <paths>". It names only paths that have no diagnostic of their own.

Parked on purpose:

- Review-readiness digest with a readiness verdict. The verdict had no defensible decision rule.
- Behavioral test-gap detection as a primary claim.
- Model judgment, rubric scoring, and provenance split on a hosted path.
- Harness hooks and a web user interface.
- Cycle detection between modules. Coach has no cycle rule. A cycle between two layers can be expressed as forbidden pairs in both directions, but a cycle inside one layer cannot. Revisit when the owner scopes a cycle rule.

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

Owner decision, 26 September 2026: the living evaluation's restamp against `HEAD` governs leaving pilot. Stay in pilot until a named audience can be told an honest job on a fresh CLI corpus. Closed children of [#282](https://github.com/lousy-agents/coach/issues/282) do not end the pilot. GitHub `CLOSED` is not the gate.

Leave pilot only when all of these statements are true:

- A new evaluation restamp against current `HEAD` agrees that a named audience can be told an honest job.
- A first-time user can follow the published README and obtain either a report or a clear next action. The user does not receive a false all-clear.
- A user who did not write Coach runs a second scan without a prompt from the owner.
- Security-category claims have a recorded precision review on a named corpus. The review includes true positives, false positives, and known misses.
- The default command path still writes nothing to GitHub and still scores no person.

If those statements are not true, the correct product action is to stay in pilot.

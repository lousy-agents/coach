# Coach PRD - Local CodeSignal Pilot (v3)

This document supersedes the v2 Platform Groundwork PRD from 20 July 2026.

The v2 document described an async analysis platform with a hosted service. That hosted service is not the current product. The product that ships today is a local command-line tool.

## Evidence cutoff

- Latest published release: `v0.11.0` (2 November 2026).
- Leave-pilot evaluation: [`evaluations/codesignal-pilot-readiness.html`](evaluations/codesignal-pilot-readiness.html). Pass 32: re-run on `229fffdb` (4 November 2026), each claim against that head and against the base the branch was built on, `ee0b82e8`. Behaviour changed after `229fffdb`, including explicit-policy handling and the TypeScript layer-bypass search (`d6ed227`), so pass 32 does not cover it; for explicit-policy handling the requirements in sections 8.4 and 8.5 and the recorded runs cited in those commits do. Masthead date 4 November 2026. That pass is a focused re-run, not a full corpus replay.
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

The engineer needs a private report over recent committed work. A coding agent that works for the engineer needs a report of the same kind over its own uncommitted work before it commits: agent self-review. The current product reports deterministic rule results only.

Behavioral test gaps remain a real problem. This era does not claim to detect them.

## 2. Who this era serves

The user is the project owner or a small set of engineers who scan their own Git checkout.

The user runs Coach on a repository they already have on disk, directly or through a harness hook in a coding agent the user runs. The default command path does not call GitHub. The default command path does not call a model.

This era has no manager view. This era has no team rollout. This era has no anonymous audience.

## 3. Current workaround

Without Coach the user does one or more of these actions:

- read every assistant-generated diff by hand
- paste fragments into a chat model and accept an ungrounded answer
- wait for a public review bot to comment on an open pull request
- run a general linter and treat a clean run as a review

Those workarounds leak privacy, mix opinion with evidence, or miss structural defects that span files.

## 4. Desired outcome

Desired outcome: a pilot user who did not write Coach runs a second scan after the first report, without a request from the product owner. Only a scan a person starts counts; a hook's scans do not.

The outcome is not a count of rules. The outcome is not a hosted service. The outcome is not a merge gate.

Kill tests that end this bet live only in section 11.

Repeat use is observed through out-of-band pilot check-ins. The CLI sends no telemetry.

If a kill test fails, stay in pilot. Do not name a new audience.

## 5. Scope this era

In scope:

- local `coach codesignal` on a Git checkout
- Go, TypeScript, and TSX source: committed, or uncommitted when the user passes `--worktree`; and the project policy file as it is on disk
- file-local structural signals
- project policy that the user writes, read from disk as ESLint and Biome read their configuration, and disclosed when it differs from the policy at `HEAD` (section 9 records what has shipped); file-local analysis when none is selected
- consented setup for a missing TypeScript compiler when a terminal is present
- an honest report when analysis is incomplete
- agent self-review before commit: a coding agent runs `coach codesignal --worktree` from a harness hook (Claude Code first) to review its own uncommitted work; Coach ships the CLI behaviour and documents an example hook, not a hosted hook service

Out of scope in this era:

- a hosted Coach service as the default product
- a web user interface
- a Coach-maintained hook package or plugin for each harness (the hook stays user-owned, with an example in the docs)
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
- Policy disclosure: every scan reports the policy path and digest it used, and whether the policy selected from disk matches the one selected at `HEAD` (section 8.4). The report does not compare with an older analyzed revision. JSON `scope.policy` is always present. A text line appears when a policy is selected or the two differ. A difference appears as a WARNING, and it qualifies any clean result. JSON `scope.policy` also states the cause of the comparison, one of `same_as_head`, `differs_from_head`, `not_committed` and `head_unreadable` (section 8.4).

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

WHEN `--worktree` is passed and the working tree holds a state the scan does not read, THE Coach CLI shall emit one `worktree_scan_incomplete` entry per cause, where a cause is one reason at one root directory, and the text shall group the entries by reason. Each entry carries a `reason`, one of `sparse_checkout`, `skip_worktree`, `assume_unchanged`, `submodule`, `nested_repository`, `unreadable_directory`, `git_version`, and `unrecognized_git_warning`. The causes are a sparse checkout, a path marked skip-worktree or assume-unchanged, a dirty submodule, an untracked nested repository, a directory git could not open, such as an untracked directory without read permission, submodules under a git before 2.44, and a warning git prints that the scan does not recognize. Git leaves a directory it could not open out of the status without failing. On a git before 2.44, which cannot be told not to fetch, the status does not look into submodules, so a repository that has any gets one `git_version` entry per submodule root whatever its state, naming the git version, and no `submodule` entry for that root. Any other warning git prints on stderr while reading the status, which the scan does not recognize, gets an `unrecognized_git_warning` entry with a generic message ("git printed a warning ... does not recognize") that states how many warnings git printed. A reason that has no root, such as `unrecognized_git_warning`, gets one entry. Each entry names the affected root directory where git can identify one, and never names individual sparse or skip-worktree files. Each reason lists at most 20 root directories. When a reason has more, one further entry for that reason states how many roots were not listed, in JSON and in text. The cap never yields zero entries for a reason that is present. Roots whose cause qualifies the verdict are listed first, and the further entry qualifies the verdict when any root it does not list does. No entry relays git's stderr. A directory whose name holds a control character is shown quoted and escaped, never dropped.

In this section, an in-scope, supported-language path is a Go, TypeScript or TSX path that the active `--scope` filter selects, a project configuration file that the analysis would read (`tsconfig*.json`, `package.json`, `go.mod`, `go.work`), whether or not the overlay could read it, or the selected policy file.

IN EVERY MODE, THE Coach CLI shall not analyze the files of another repository, a submodule or a nested repository. A `submodule`, `nested_repository` or `git_version` entry therefore hides no in-scope path, and is disclosed without qualifying the verdict, unless the gitlink or nested repository sits at a path that the active `--scope` selects as a Go, TypeScript or TSX file, for example a submodule at `x.ts`.

Each `worktree_scan_incomplete` entry says whether its cause hides an in-scope path; slice B of #168 fixes the field name. A consumer reading an entry without that field shall treat the entry as qualifying the verdict. An entry whose root collapses to the repository root names `.` as its root. A sparse checkout that hides nothing is one `sparse_checkout` entry with no root.

IF a `worktree_scan_incomplete` cause hides an in-scope, supported-language path, THEN THE Coach CLI shall qualify the verdict.

IF a `worktree_scan_incomplete` cause hides no in-scope, supported-language path, THEN THE Coach CLI shall disclose it, as its `worktree_scan_incomplete` entry in JSON and a line in the text, and shall not qualify the verdict.

IF git cannot determine which paths a `worktree_scan_incomplete` cause hides, THEN THE Coach CLI shall qualify the verdict.

WHILE a `worktree_scan_incomplete` cause hides an in-scope, supported-language path, or git cannot determine which paths it hides, THE Coach CLI shall not print an unqualified all-clear.

Under `--worktree`, a verdict additionally counts as incomplete coverage when part of the working tree could not be read, and `--fail-on-incomplete-coverage` exits `3` for it. Those cases are `worktree_changes_not_analyzed`, `worktree_symlink_skipped`, `worktree_status_check_failed`, and a `worktree_scan_incomplete` cause that qualifies the verdict, whether its root is listed or not. `worktree_status_check_failed` means that a sparse, skip-worktree, gitlink or snapshot read failed under `--worktree`; the committed scan's dirty-tree check also uses it. A failed `git status` itself is an operational error, not a verdict: the CLI exits `1`, writes "coach: listing working tree changes: ..." to standard error, and prints no verdict. `unsupported_language` and `continuity_not_determined` still qualify the verdict text, but they exit `0`, as a committed scan does. In every scan, committed or `--worktree`, `project_policy_absent_on_disk` also counts when no project policy was read, and `--fail-on-incomplete-coverage` exits `3` for it.

IF analysis of a requested path is incomplete, THEN THE Coach CLI shall not print an unqualified all-clear.

IF the user supplies an invalid project policy, THEN THE Coach CLI shall exit with status `2`, write the error to standard error, and write no report to standard output.

IF a TypeScript project scan cannot resolve a supported compiler or a supported host Node major, and no interactive setup offer runs, THEN THE Coach CLI shall exit with status `2` and emit no CodeSignal report.

WHERE the environment has a non-empty `CI` variable, or the user passes `--no-interactive`, THE Coach CLI shall not open a prompt.

WHERE the user passes `--fail-on-incomplete-coverage` and required project coverage is incomplete on any analyzed side, or the policy `HEAD` selects is absent on disk and no project policy was read, THE Coach CLI shall write the report to stdout and exit `3`.

WHEN the selected project policy differs from the policy selected at `HEAD`, THE Coach CLI shall state the difference in the report and shall qualify any clean result.

WHILE the project policy is read from the commit, WHEN `--worktree` is passed and the selected policy file (`.lousy-agents/coach.json`, or the deprecated `project.json`) has uncommitted changes, THE Coach CLI shall emit `worktree_changes_not_analyzed` naming the policy path, with the message "the project policy is read from the commit at <rev>; --worktree does not read an uncommitted policy yet", shall qualify the verdict, and shall exit `3` under `--fail-on-incomplete-coverage`. Policy-from-disk removes this requirement.

WHILE the project policy is read from the commit, WHEN `--worktree` is passed and the project policy file (`.lousy-agents/coach.json`, or the deprecated `project.json`) is untracked, THE Coach CLI shall emit `worktree_changes_not_analyzed` naming the policy path, with the message "the project policy at <path> is not committed; --worktree reads the policy from the commit at <rev> and does not read an uncommitted policy yet", shall qualify the verdict, and shall exit `3` under `--fail-on-incomplete-coverage`. Policy-from-disk removes this requirement.

WHILE the project policy is read from the commit, WHEN `--worktree` is passed and any status entry whose path or source path is a project policy file (`.lousy-agents/coach.json` or `project.json`) is present, whatever its status code (untracked, staged, intent-to-add, renamed into or out of the policy path, or edited), THE Coach CLI shall name that path in `worktree_changes_not_analyzed`, shall qualify the verdict, and shall exit `3` under `--fail-on-incomplete-coverage`. The selected policy keeps the message "the project policy is read from the commit at <rev>; --worktree does not read an uncommitted policy yet"; any other policy file says that it is not committed, or that it has uncommitted changes. Policy-from-disk removes this requirement.

WHEN `--worktree` is passed and any worktree overlay read is incomplete, except a `worktree_scan_incomplete` cause that hides no in-scope, supported-language path, THE Coach CLI shall qualify the verdict and shall not print an unqualified all-clear.

### 8.4 Project policy

WHERE the user passes a project policy, THE Coach CLI shall evaluate only the layers and forbidden imports that the policy names.

THE Coach CLI shall not invent layers or forbidden imports.

WHEN the user omits `--project-config` and a `.lousy-agents/coach.json` on disk is schema-valid, THE Coach CLI shall use that policy and emit a project-policy report.

WHEN the user omits `--project-config`, no `.lousy-agents/coach.json` exists on disk, and a schema-valid root `project.json` exists on disk, THE Coach CLI shall use `project.json` and emit `project_config_deprecated`.

WHEN the user omits `--project-config`, and neither a `.lousy-agents/coach.json` nor a schema-valid root `project.json` exists on disk, THE Coach CLI shall keep file-local analysis.

IF a `.lousy-agents/coach.json` on disk fails schema validation and `--project-config` is omitted, THEN THE Coach CLI shall not read `project.json`. THE Coach CLI shall exit with status `2`, write no report to standard output, and name `project_config_invalid` and the path on standard error. For a TypeScript scan it shall also say in words that the policy on disk cannot be used: fix the file, and shall print no command for the step.

WHEN a scan selects the root `project.json` fallback on a controlling terminal, `.lousy-agents/coach.json` does not exist, and prompts are not disabled, THE Coach CLI shall ask once on standard error, default No, whether to copy that file. It shall write nothing unless the user answers Yes. It shall never overwrite an existing canonical file or delete `project.json`.

WHEN a policy is valid, THE Coach CLI shall emit a project-policy report that is distinct from the file-local report.

WHEN a policy is valid and an import edge violates a named forbidden pair, THE Coach CLI shall emit `architecture.layer_violation`.

WHERE the policy names a required layer and a matching path from a handler-shaped source to a pinned sink exists, THE Coach CLI shall emit `architecture.layer_bypass`.

WHILE the project policy is read from disk, THE Coach CLI shall, in every scan, read the selected project policy from disk, committed or not, as section 7 states, and both sides of a comparison shall use it. A policy file that is on disk and not committed at `HEAD` is read, and the disclosure in section 7 names it with a WARNING. The report discloses whether the policy on disk matches the policy selected at `HEAD` (section 7).

WHILE the project policy is read from the commit, THE Coach CLI shall read it from the analyzed revision, never from the working tree. A policy file that is on disk and not committed there is not read: a discovered one is reported as `project_config_uncommitted`, and one named with `--project-config` is refused with exit `2` (section 8.4). Policy-from-disk removes this requirement.

WHEN a TypeScript project scan completes with no configured covered match, THE Coach CLI shall state that and list `project_next_actions`.

WHEN the user passes `--project-config <path>` and that path does not exist on disk, THE Coach CLI shall exit with status `2`, write no report to standard output, and name the path and `project_config_missing` on standard error. It shall not fall back to file-local analysis or to another policy. On a controlling terminal, for a TypeScript scan, section 8.5 offers guided policy authoring instead of the scan, unless a requirement below withholds that offer, and that scan also exits with status `2`.

WHERE that path is committed at the analyzed revision and gone from disk, THE Coach CLI shall state the one policy remedy `git restore --source=<revision> --staged --worktree -- ':(top,literal)<path>'`, with `<revision>` and `<path>` filled in, as the last text on its line, and shall offer no guided policy authoring or `--suggest-project-config` command, for a Go scan as well as a TypeScript scan. It shall state no restore command where running it would overwrite something or recreate what the scan refuses: a policy committed as a symbolic link, a policy that a sparse checkout leaves out of the working tree, a policy the index holds as an unmerged (conflicted) entry, a policy marked skip-worktree while no sparse checkout is enabled, an index that is locked (an `index.lock` exists, so the restore would fail), or a question git could not answer. It shall say why, and offer no authoring either. Where the index holds a staged edit to the policy, the restore from the revision would discard the edit, so it shall not be offered as the default: it shall say so, shall name the restore from the index that keeps the edit, and shall name the restore from the revision only as a labelled choice, to drop it and bring back the committed copy. An index entry that holds no content, a regular file at stage 0 whose blob is the empty blob, is not a staged edit: it shall state the restore command and say that restoring brings back the committed copy, unless the entry is marked skip-worktree, which stops a restore whether or not the entry holds content and is then named instead.

WHERE that path does not exist on disk, is not committed at the analyzed revision, and the index holds an entry for it that holds content (an intent-to-add entry or a staged empty file counts as none, unless it is marked skip-worktree: git add refuses a new file at a path the mark hides, so the CLI shall then call it an empty entry, name the mark or the sparse checkout, offer no new policy, and not call it a staged copy), THE Coach CLI shall exit with status `2`, name the path and `project_config_missing`, say what the index holds, and offer no guided policy authoring, no `--suggest-project-config` command and no `author_policy` next action, because a new policy would replace staged work. WHERE the entry is a staged regular file, it shall state the one command that restores it from the index, `git restore -- ':(top,literal)<path>'`, as the last text on its line. It shall state no restore command where running it would fail or would recreate what the scan refuses: an unmerged entry, an entry that is a symbolic link or another entry that is not a regular file, an entry marked skip-worktree, an entry that a sparse checkout leaves out of the working tree, or an index that is locked. An entry that holds no content, a regular file at stage 0 whose blob is the empty blob, as `git add --intent-to-add` stages, counts as no entry, and the path gets the answer for a policy that was never committed.

WHERE a path component on disk is not a directory, THE Coach CLI shall state that a parent path component is not a directory, name the blocking component, and say to move or remove it, unless the analyzed revision commits that component as a file, in which case it shall say that the path runs through a committed file and is probably wrong, and not advise removing it. When the policy is also committed at the analyzed revision, it shall say to restore the policy from git after the file is moved, unless the index holds a staged edit to it, in which case it shall name the staged edit instead, and give the same two choices after the file is moved (the restore from the index that keeps the edit, and the restore from the revision as the labelled way to drop it), because a restore from the revision would discard the edit by default, or the policy is committed as a symbolic link, in which case it shall say so instead, because a restore would recreate a link the scan refuses. When the policy is not committed but the index holds a staged copy of it, it shall say to restore the staged copy from the index after the file is moved. It shall print no `git restore` command, because running one would overwrite that untracked file, and shall offer no guided policy authoring or `--suggest-project-config` command.

WHERE that path does not exist on disk, is not committed at the analyzed revision, the index holds no entry for it (an entry that holds no content counts as none), and a new policy can be written at it, THE Coach CLI shall offer `--suggest-project-config` after the error for a Go scan, and guided policy authoring as section 8.5 requires for a TypeScript scan on a controlling terminal.

WHILE the project policy is read from disk, WHERE the committed copy of the selected policy cannot be read at `HEAD` within git's time or stderr bound, THE Coach CLI shall use the policy on disk, report `matches_head` as `false`, and print a WARNING that says the committed copy could not be read and names the bound that tripped, not that the content differs. The committed copy is read only for that disclosure.

WHILE the project policy is read from the commit, WHERE the committed copy of an explicit policy cannot be read at the analyzed revision within git's time or stderr bound, THE Coach CLI shall exit with status `2`, name the path, `project_config_invalid` and the bound that tripped, and offer no `--suggest-project-config` command or guided policy authoring, because a new policy does not change why the read failed. A policy over the size budget is a content gap that a smaller policy fixes, and keeps that offer. Policy-from-disk removes this requirement.

WHILE the project policy is read from disk, WHERE `--project-config <path>` names a usable regular file on disk that is not committed at the analyzed revision, THE Coach CLI shall read it, shall disclose it with a WARNING as section 7 requires, and shall not exit with status `2` for it.

WHILE the project policy is read from the commit, WHERE `--project-config <path>` names a regular file on disk that is not committed at the analyzed revision, THE Coach CLI shall exit with status `2`, name the path and `project_config_invalid`, and say to commit the file. WHERE that path lies inside a submodule, it shall name the submodule instead and say that the file cannot be committed here, because this repository records a submodule as a single commit. It shall offer no `--suggest-project-config` command, guided policy authoring or `author_policy` next action, because a file already sits at the path. Policy-from-disk removes this requirement.

WHERE a remedy command would name a policy path containing a control, format, line-separator or paragraph-separator character, or invalid UTF-8, THE Coach CLI shall print no command for it. The line shall say to use the same `--project-config`, and that its name is not printed, so no pasted command runs against another policy.

WHILE the project policy is read from disk, WHERE the spelling of the policy path, as named with `--project-config` or as the canonical `.lousy-agents/coach.json`, differs only in case from the spelling the directory listing on disk gives for that file, and the listing holds no entry spelled exactly as the path, THE Coach CLI shall exit with status `2`, name `project_config_invalid` and the cause `name_case_differs`, and say to rename the file on disk or to pass the spelling on disk. It shall not say to commit. The path is compared with the directory listing case-folded, on every file system, so a case-only difference is `name_case_differs` on a file system that folds case and on one that does not, and never `project_config_missing`. An entry spelled exactly as the path wins over its case variants. WHERE the listing holds several case variants and none is spelled exactly as the path, the message lists them, up to 16, and past 16 chooses none. In this section and in the discovery requirements above, a policy path exists on disk when this case-folded comparison finds it. A difference between the spelling on disk and the spelling committed at `HEAD` is not refused: the disclosure in section 7 reports it, with `matches_head` `false` and a WARNING.

WHILE the project policy is read from disk, THE Coach CLI shall judge the kind of the entry at the policy path on disk. A usable regular file on disk is read whatever `HEAD` commits at that path, such as a symbolic link or a directory, and the disclosure in section 7 reports the difference with `matches_head` `false` and a WARNING.

WHILE the project policy is read from disk, IF `--project-config` is omitted and the canonical `.lousy-agents/coach.json` on disk is an unusable entry, as this section defines an unusable path, THEN THE Coach CLI shall exit with status `2`, write no report to standard output, and name `project_config_invalid`, the path and the reason on standard error. It shall not read `project.json` and shall not fall back to file-local analysis.

WHILE the project policy is read from the commit, the analyzed revision judges the kind of a discovered canonical path, as [ADR-008](../architecture/ADR-008-explicit-policy-disk-and-revision.md) describes. Policy-from-disk removes this requirement.

WHILE the project policy is read from the commit, WHERE the analyzed revision does not commit a policy path as written but commits the same path in another case, THE Coach CLI shall exit with status `2`, name `project_config_invalid` and the cause `name_case_differs`, offer no guided policy authoring or `--suggest-project-config` command, and describe the rename in words: it shall name the committed spelling and the path Coach reads, and say to make git record the policy at that path, then commit and run again. It shall print no `git mv`, `git rm` or other shell command, shall always warn that a file already at the destination is to be moved aside first because git can overwrite it when `core.ignorecase` is set, and shall tell a rename that is already staged the same as one that is not. WHERE the analyzed revision commits the policy under several spellings, it shall list them, up to 16, and say to keep one at that path and remove the others, and past 16 it shall choose none to keep. The message shall be the same for a policy named with `--project-config` and a policy Coach finds, on a file system that folds case and one that does not, except that for a policy named with `--project-config` it shall first offer passing the committed spelling, or one of them when several are committed, because the name may be a slip. WHERE the rename changes the case of a directory's name, it shall add that git cannot do that in one step on a file system that folds case, and say to rename the directory to another name first. Policy-from-disk removes this requirement.

IN EVERY SCAN, THE Coach CLI shall report the policy path and digest it used, and whether the policy selected from disk matches the policy selected at `HEAD`, as section 7 requires. Two policies match when they have the same path and either the same digest, or the file on disk is the blob selected at `HEAD` once git's own checkout conversion is applied: line endings per `core.autocrlf` and the `text` and `eol` attributes, `$Id$` expansion, and `working-tree-encoding`. A path with a `filter` attribute is never treated as converted, and no program runs. The report also states the cause, one of `same_as_head`, `differs_from_head`, `not_committed` and `head_unreadable`: `same_as_head` when the policy on disk is the policy selected at `HEAD`; `differs_from_head` when the policy selected from disk is a file that is committed at `HEAD` but is not the policy `HEAD` selects, whether the same path with other content, another committed path such as the root `project.json` that stands in for a deleted canonical policy, or a path where `HEAD` holds a document that is not a valid policy or an entry that is not a regular file; `not_committed` when the policy selected from disk is not committed at `HEAD` at all; and `head_unreadable` when `HEAD` holds a regular file at that path that cannot be read. A policy on disk that is unusable is not a cause: the scan refuses it. A policy committed at `HEAD` and absent on disk is never read from the commit: the scan uses the root `project.json` on disk, which must be usable (an unusable one is refused like an unusable canonical policy, with exit `2`), or is file-local, and prints a WARNING that names the policy `HEAD` selects, with the diagnostic `project_policy_absent_on_disk`, which counts as incomplete coverage in every scan mode when no project policy was read. `matches_head` is `true` exactly when the cause is `same_as_head`. When neither the disk nor `HEAD` selects a policy, JSON `scope.policy` is `null`. When only the disk selects a policy, `matches_head` is `false` and the text prints a WARNING. When only `HEAD` selects a policy, the policy was not read: JSON `scope.policy` is `null` and the text prints a WARNING that names the policy `HEAD` selects.

WHEN the user passes `--project-config <path>` and the path is unusable on disk, THE Coach CLI shall exit with status `2`, write no report to standard output, and name `project_config_invalid`, the path and the reason on standard error. A path is unusable when it is not a regular file that this process can open, or cannot be reached: a symbolic link, dangling or not; a directory, FIFO or other entry that is not a regular file; a file that cannot be read; a path that runs through a symbolic link; or a path whose parent directory cannot be searched. A dangling symbolic link at the path is reported as `project_config_invalid`, not as `project_config_missing`. The committed copy never stands in for an unusable path.

### 8.5 TypeScript project setup

WHERE the user passes `--project-config <path>` to a TypeScript scan and that path does not exist on disk, THE Coach CLI shall state the remedy section 8.4 requires for that case, and other failing gaps, such as a missing Node major or compiler, shall still print their own lines beside it.

WHERE the user passes `--project-config <path>`, that path does not exist on disk, the path is not committed at the analyzed revision, the index holds no entry for it (an entry that holds no content counts as none), and a new policy can be written at the path, THE Coach CLI shall offer guided policy authoring on a controlling terminal for a TypeScript scan, as this section requires.

WHERE the user passes `--project-config <path>` and the disk decides that the policy at the path is unusable, as section 8.4 defines, THE Coach CLI shall offer no guided policy authoring and shall name no `author_policy` or `restore_policy` next action for the path.

WHEN `--check-project` reports a missing explicit policy, THE Coach CLI shall name `restore_policy` as the next action only when the path can be restored without overwriting anything, and `author_policy` only when a new policy can be written at the path. It shall name neither in any other case. Each carries its kind and `executable: false`.

WHEN `--check-project`, `--prepare-compiler` or the compiler setup a TypeScript scan offers judges the project policy, THE Coach CLI shall read it from the working tree, as section 8.4 selects it, and shall leave the policy's own path out of `dirty_worktree` in every case, whether the file is usable, unusable or absent, because the working tree decides the verdict. A usable policy that is not committed, that differs from the committed copy or is committed as a symbolic link, whose path the index holds in conflict, or whose committed copy cannot be read, shall pass, with a warning that names the same relation as `scope.policy.cause`: `not_committed` is `project_policy_not_committed`; `differs_from_head`, and a policy that the revision commits as a link or holds in conflict, are `project_policy_differs_from_head`; `head_unreadable` is `project_policy_head_unreadable`. A warning shall not change `status`, and shall print before any prompt for compiler setup.

WHEN `--check-project` reports a policy gap that names neither `author_policy` nor `restore_policy`, THE Coach CLI shall carry a stable `cause` on that gap in the JSON result and print the same word after the gap code in the text, so a reader can tell a path that cannot be a file from one that is gone. The cause never says what the index or the analyzed revision holds for a path that has a usable file on disk; where the file is absent and the index holds work for the path, the cause is `undetermined`, and no next action offers a restore that would discard that work. A gap that names either action carries no `cause`, and every other policy gap carries one (`undetermined` when Coach has no other word for the state), so a missing `cause` always means a remedy fixes the gap or the gap is not a policy gap.

WHERE `--check-project` leaves the policy's own path out of `dirty_worktree` as the previous requirement states, THE Coach CLI shall keep in `dirty_worktree` every other uncommitted path that the report did not read and that lies under the roots of the policy. WHEN the working tree refused or lacks a policy that is committed at the analyzed revision, those roots shall be the roots of the committed copy. Otherwise the roots are those of the policy read from the working tree. WHERE no valid policy is committed at the analyzed revision and none was read from the working tree there are no roots, and only package and compiler metadata (`package.json`, lockfiles, `.npmrc`, `bunfig.toml` and `tsconfig*`) counts.

WHERE a requirement in section 8.4 or this section names a remedy or a withheld offer for a missing explicit `--project-config` path, that requirement takes precedence over the requirement that offers guided policy authoring for the same path.

WHEN a TypeScript scan on a controlling terminal passes `--project-config` and that policy file does not exist on disk, THE Coach CLI shall offer guided policy authoring, except where section 8.4 or this section says otherwise, and the scan shall exit with status `2` as section 8.4 requires.

WHILE the project policy is read from the commit, WHEN the policy file exists on disk but is not committed at the analyzed revision, THE Coach CLI shall not read it. For the discovered `.lousy-agents/coach.json` it shall report `project_config_uncommitted` and continue without that policy; for a `--project-config` path it shall exit with status `2` and say that the file is not committed (section 8.4). Policy-from-disk removes this requirement.

WHEN that scan authoring path runs, THE Coach CLI shall write the candidate to standard output, reject `--output`, and write no repository file.

WHEN the candidate is written, THE Coach CLI shall exit with status `2`, emit no report, and instruct the user to save, review, commit, and rerun.

WHEN the user runs standalone TypeScript policy authoring on a controlling terminal, THE Coach CLI shall write no repository file until the user confirms.

WHEN a TypeScript scan omits `--project-config` and section 8.4 selects no policy, THE Coach CLI shall stay file-local and open no policy authoring.

WHEN a TypeScript scan omits `--project-config` and section 8.4 selects a policy, THE Coach CLI shall apply the same readiness, setup, and policy rules as for that path passed explicitly.

WHEN a TypeScript scan omits `--project-config` and a `.lousy-agents/coach.json` on disk fails schema validation, with or without a controlling terminal, THE Coach CLI shall open no guided policy authoring, because the only session it has ends by asking the user to save a candidate over the file. THE Coach CLI shall exit with status `2`, emit no report, and print the one sentence `coach codesignal: the policy on disk cannot be used: fix the file.`, which `--prepare-compiler` also prints for the same policy.

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
| TypeScript readiness check | Implemented from `v0.4.0`. `--check-project` reports status, checks, and gaps, mutates nothing, and exits `0`. The project policy is read from the working tree, and how it differs from the analyzed revision is a warning, not a gap. Locked by `cmd/coach/project_readiness_acceptance_test.go`, `cmd/coach/check_project_policy_from_disk_acceptance_test.go` and `cmd/coach/check_project_policy_unusable_on_disk_acceptance_test.go`. |
| Guided policy authoring | Implemented from `v0.5.0` for a scan whose explicit `--project-config` names no policy. Authoring on a discovered `.lousy-agents/coach.json` that is valid with no layers is **planned until #512 merges**, then implemented, and it gets the offer after the scan. A discovered file that is schema-invalid gets no session, with or without a terminal: the scan says in words that the policy on disk cannot be used: fix the file. Every scan authoring path needs a terminal and writes no repository file. Locked by `cmd/coach/project_ts_scan_policy_authoring_acceptance_test.go` and, once #512 merges, `cmd/coach/project_config_discovered_authoring_acceptance_test.go`. |
| Consented TypeScript compiler setup | Implemented as `--prepare-compiler` in `v0.6.0` and as a scan-time offer in `v0.7.0`. The standalone flag is mise-only. Both read the project policy from the working tree and print the policy warnings before the choice prompt. Locked by `cmd/coach/project_ts_setup_execute_acceptance_test.go`, `cmd/coach/project_ts_setup_preview_acceptance_test.go`, `cmd/coach/prepare_compiler_policy_from_disk_acceptance_test.go` and `cmd/coach/project_ts_scan_policy_from_disk_setup_acceptance_test.go`. |
| Unattended path that refuses prompts | Implemented as `--no-interactive` and a non-empty `CI` variable in `v0.7.0`. Locked by `cmd/coach/project_ts_scan_policy_authoring_acceptance_test.go`. |
| JSON signals carry rule identity and rule version | JSON `rule_id` and `rule_version` are implemented; `pkg/codesignal/report_shape_acceptance_test.go` locks the JSON key shape. Default text output prints `rule_id` and `severity` on every file-local and project finding, which ships in the release that carries this PR; `cmd/coach/text_finding_identity_acceptance_test.go` locks that text claim. `subject` is not printed and `source_scope` is printed but not collapsed ([#279](https://github.com/lousy-agents/coach/issues/279)). |
| TypeScript project `project_provenance`, `project_scope`, `worktree_report_reflects_committed_head`, complete no-match `project_next_actions`, and `--fail-on-incomplete-coverage` exit `3`; file-local `worktree_changes_not_analyzed` and `worktree_not_clean` | `project_provenance`, `project_scope`, `project_next_actions`, and `--fail-on-incomplete-coverage` implemented in `v0.8.0`. `worktree_report_reflects_committed_head`, `worktree_changes_not_analyzed`, and `worktree_not_clean` ship in the release that carries this PR. Locked by `cmd/coach/project_scope_cli_acceptance_test.go`, `cmd/coach/project_provenance_worktree_acceptance_test.go`, `cmd/coach/working_tree_disclosure_acceptance_test.go`, and `cmd/coach/fail_on_incomplete_coverage_acceptance_test.go`. |
| Policy read from disk, with disclosure | Planned. Becomes implemented only when acceptance tests on main lock it. Replaces the committed-policy read and the `project_config_uncommitted` diagnostic. The state "the project policy is read from disk" that gates the requirements in section 8 holds when this row reads implemented; until then, "the project policy is read from the commit" holds. |
| Worktree scanning (`--worktree`) | Experimental. The flag is accepted but hidden from `--help`, and it is not yet in the README or user documentation. Run it from the repository root. A repository's clean filter can still run when `git status` must compare same-size content; no flag disables it. Ships in the release that carries this change: `--worktree` alone compares the working tree with `HEAD`, and `--base <ref>` or `--baseline` selects another comparison. Supported-language source and project configuration files are read from the working tree, including new and untracked files; a deleted file is absent; a symbolic link is not followed and is named; a file the scan cannot read is named, and its message gives the cause, such as permission denied, not a regular file, over the size limit, a failed read, or a name on disk that differs from the name Git records; `scope.worktree` reports the revision, the modified, added, deleted and untracked counts, and the configuration files read from disk. The `AD`, `RD` and `CD` cases of those counts are locked by `cmd/coach/worktree_staged_then_deleted_acceptance_test.go`. A sparse checkout, skip-worktree or assume-unchanged entry, a dirty submodule, an untracked nested repository, and a directory git could not open qualify the verdict as a whole, through `worktree_scan_incomplete`; the nested-repository case is locked by `cmd/coach/worktree_nested_repository_acceptance_test.go`. Under `--fail-on-incomplete-coverage`, the verdict exits `3` for the four incomplete-read kinds in section 8.3 and for incomplete required project coverage. Still planned: one `worktree_scan_incomplete` entry per cause, each with a `reason` and its root directory, in place of the run-level qualification, as section 8.3 requires; it becomes implemented only when acceptance tests on main lock it. Also planned: a machine-readable `reason` field on unreadable-file diagnostics. |
| Explicit `--project-config` that is missing or unusable on disk | Exit `2` with `project_config_missing` when nothing is at the path, with no file-local fallback. An unusable path, as section 8.4 defines it, exits `2` with `project_config_invalid`, and `--check-project` reports the same file as a policy gap. A restore is offered only where it overwrites nothing. Ships in the release that carries this PR. Locked by `cmd/coach/project_config_missing_acceptance_test.go`, `cmd/coach/project_config_unusable_on_disk_acceptance_test.go`, `cmd/coach/project_config_restore_safety_acceptance_test.go`, `cmd/coach/project_config_staged_edit_visibility_acceptance_test.go`, `cmd/coach/project_config_git_edges_acceptance_test.go`, `cmd/coach/project_config_uncommitted_on_disk_acceptance_test.go`, and `cmd/coach/check_project_explicit_policy_acceptance_test.go`. |
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
- Agent self-review before commit (section 5) is a bet. `--worktree` is experimental until [#720](https://github.com/lousy-agents/coach/issues/720) merges, and no example hook is documented yet.
- An agent that edits the policy can hide its own violations, because the scan reads the policy on disk. The example hook shall treat `scope.policy.matches_head: false` as not reviewed-clean: it tells the agent and surfaces it to the user. The mechanics belong in the hook documentation.
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
- A web user interface.
- Cycle detection between modules. Coach has no cycle rule. A cycle between two layers can be expressed as forbidden pairs in both directions, but a cycle inside one layer cannot. Revisit when the owner scopes a cycle rule.

If a later path adds model judgments, each finding must stay tagged as deterministic or agent. Agent output must not hide a deterministic finding. A schema failure on the agent path must still return the deterministic report. That contract is not a requirement of the default CLI path.

## 11. Risks as tests

Do not treat this table as a substitute for customer evidence. Each row is a test. A failed test blocks expansion of audience. This is the only kill-test table.

| Risk | Question | Cheapest test that kills the bet |
| --- | --- | --- |
| Value | Will a pilot user choose this report over hand review or a chat paste? | After one report, the user does not run a second scan (a scan a person starts, as section 4 counts it) and cannot name one action they took. |
| Usability | Can a first-time TypeScript user complete readiness and a first scan from the README? | The user hits a hidden step, a prompt with no terminal, or a false clean result. |
| Feasibility | Can the shipped analyzer examine the change the user asked about? | A requested path is skipped and the text still reads as complete and clean. |
| Business viability | Does this stay a private advisory tool? | A GitHub write, a person score, or a merge block lands on the default path. |

## 12. Release criteria for leaving pilot

Owner decision, 26 September 2026: the living evaluation's restamp against `HEAD` governs leaving pilot. Stay in pilot until a named audience can be told an honest job on a fresh CLI corpus. Closed children of [#282](https://github.com/lousy-agents/coach/issues/282) do not end the pilot. GitHub `CLOSED` is not the gate.

Leave pilot only when all of these statements are true:

- A new evaluation restamp against current `HEAD` agrees that a named audience can be told an honest job. The jobs it judges are a private report over recent committed work, and agent self-review before commit (section 5). Until the evaluation carries a claim for agent self-review, that job is not judged.
- A first-time user can follow the published README and obtain either a report or a clear next action. The user does not receive a false all-clear.
- A user who did not write Coach runs a second scan without a prompt from the owner. A hook's scans do not count (section 4).
- Security-category claims have a recorded precision review on a named corpus. The review includes true positives, false positives, and known misses.
- The default command path still writes nothing to GitHub and still scores no person.

If those statements are not true, the correct product action is to stay in pilot.

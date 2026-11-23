# `coach codesignal` CLI contract

Machine-oriented contract for the local `coach codesignal` CLI and the
analyzer libraries. Customer onboarding lives in [README.md](../README.md).

This documents shipped behavior. It is not a gate and not a 1.0 promise.

## Install

Coach needs `git` 2.42 or later on `PATH`. [Git version](#git-version) lists what older versions lack.

Install [mise](https://mise.jdx.dev/) if you do not have it, then pin the CLI from [GitHub Releases](https://github.com/lousy-agents/coach/releases):

```sh
curl https://mise.run | sh
~/.local/bin/mise use -g github:lousy-agents/coach
```

That mutates a **global** mise config. Hook mise into your shell so `coach` is on `PATH` (pick the one you use):

```sh
echo 'eval "$(~/.local/bin/mise activate bash)"' >> ~/.bashrc && eval "$(~/.local/bin/mise activate bash)"
# echo 'eval "$(~/.local/bin/mise activate zsh)"' >> ~/.zshrc && eval "$(~/.local/bin/mise activate zsh)"
# ~/.local/bin/mise activate fish | source
```

Pin a release if you want a fixed binary. Replace `<tag>` with a release tag from the Releases page:

```sh
mise use -g github:lousy-agents/coach@<tag>
```

To run without changing a global toolchain, prefix the command with `mise exec`: `mise exec github:lousy-agents/coach -- coach codesignal --baseline`.

Release archives exist for `darwin_arm64`, `darwin_x86_64`, `linux_x86_64`, and `windows_x86_64`. There is no `linux_arm64` build yet. The signed-archive procedure without mise is in [Signed archive without mise](#signed-archive-without-mise).

### From source

This path uses the repo's `mise.toml` to pin the **dev toolchain** (Go 1.27.1, Node 24). That is not the same as installing the published `coach` binary.

```sh
git clone https://github.com/lousy-agents/coach.git
cd coach
mise install
mise exec -- go install ./cmd/coach
```

## Run

From any Git worktree. Prefer a one-shot that does not mutate a global toolchain:

```sh
mise exec github:lousy-agents/coach -- coach codesignal --base main
```

```sh
coach codesignal --base main          # diff HEAD against main
coach codesignal --baseline           # every tracked Go/TS/TSX file at HEAD
coach codesignal --base main --format json
```

`--base` can be any ref Git can resolve (branch, tag, or SHA). Default output is text. `--base` and `--baseline` are mutually exclusive.

Signals are ranked, with `severity` and `rule_id` printed on each: lifecycle group first (with `--base`, introduced and changed findings come first), then severity, confidence, and path, with findings of the same metric rule (cognitive complexity, branch density, nesting depth) ordered by how far each exceeds its threshold. To read a long report, add `--min-severity high|medium|advisory|low` to hide lower severities (`--min-severity medium` also hides `advisory` layer-rule findings, and a `--top` cap can push them out because `advisory` ranks below every `medium` finding in the same lifecycle group. Only `complexity.cognitive_complexity` and `complexity.max_nesting_depth` findings can be `high`, so `--min-severity high` shows only those two rules. `security.toctou_check_then_act` and `state.hidden_input_mutation` are always `medium`, so a `high` floor withholds them) or `--top N` to keep the first `N` in that order (with `--base`, that order puts `resolved` findings ahead of `unknown` ones, so a cap can spend slots on findings already fixed. See [Report order](#report-order)). Both state how many signals they withheld (text also says how to show them all: the command, or a note to re-run without the narrowing flags you gave when an argument is unsafe to print. The printed command starts with `coach`, so if you launched through mise, run it as `mise exec github:lousy-agents/coach -- coach codesignal ...`), leave `summary` and `coverage` describing the full analysis, and never change the exit status. See [Narrowing the rendered report](#narrowing-the-rendered-report).

Default `--scope` is `production`. `--build-target <pattern>` further limits Go production reachability. It is a no-op under `--scope all`.

## What you should see

A short text report per signal (`rule_id`, `severity`, `path`, `line`,
`lifecycle`, `source_scope`, `changed`, `evidence`, why it matters,
recommendation), or a quiet report if nothing matched. `lifecycle` is `introduced`, `existing`, `resolved`, or `unknown`
(and `baseline` under `--baseline`). The text report also prints a `policy:` line when a project policy is selected. `--top` caps findings, not diagnostics: diagnostics still print after the findings, so on a large repository the text report can be long. Use `--format json` for machine reads.

Diff-mode limits (do not treat these as a clean PR):

- Diff mode runs `git diff --name-status` with `--find-renames` (git's
  default 50% similarity) and `--find-copies-harder` (also considers
  unmodified files as copy sources, more aggressive than git's defaults).
  Rename (`R`) and copy (`C`) **new paths** are analyzed. A pure git
  rename or copy without old-path continuity yields `lifecycle: unknown`
  plus a `continuity_not_determined` path diagnostic. Coach does not
  diff a rename against its old path, so a pure move is not classified
  `existing` or `introduced`. Adopters who only watch
  `introduced_signals` shall treat that path as incomplete, not green.
- Statuses the CLI does not analyze (`T`, `U`, `X`, `B`) stay
  `unsupported_change_type`. Those paths increment
  `summary.files_unanalyzed` and `summary.files_with_diagnostics`, and the
  text headline qualifies the all-clear with the numeric unanalyzed count.

Text `line` is 1-based. JSON `location.start_row` is 0-based.

Coach analyzes Go, TypeScript, and TSX. Other files get an `unsupported_language` diagnostic and are skipped.

Lifecycle values and the counters that add them up are in [Lifecycle and counters](#lifecycle-and-counters).

## Commands

```text
coach <command> [flags]
coach --help
coach --version
```

The only subcommand is `codesignal`.

```text
coach codesignal (--base <ref> | --baseline) [--format text|json] [--scope production|all] [--min-severity high|medium|advisory|low] [--top N] [--build-target <package>] [--project-config <path>] [--project-language go|typescript] [--no-interactive] [--fail-on-incomplete-coverage] [--worktree]
coach codesignal --worktree [--format text|json] [--scope production|all] [--min-severity high|medium|advisory|low] [--top N] [--build-target <package>] [--project-config <path>] [--project-language go|typescript] [--no-interactive] [--fail-on-incomplete-coverage]
coach codesignal --baseline --suggest-project-config [--output <path>]
coach codesignal --baseline --suggest-project-config --project-language typescript [--output <path>] [--no-interactive]
coach codesignal --baseline --check-project --project-language typescript [--project-config <path>] [--format text|json]
coach codesignal --baseline --prepare-compiler --project-language typescript [--project-config <path>] [--no-interactive]
```

`--base` and `--baseline` are mutually exclusive. `--worktree` composes with either, or
stands alone and compares the working tree with `HEAD`. `--check-project`,
`--suggest-project-config`, and `--prepare-compiler` are not scans (except
that suggest/prepare still require `--baseline`).

| Flag | Default | Notes |
| --- | --- | --- |
| `--base <ref>` | | Diff HEAD against any Git-resolvable ref. |
| `--baseline` | | Scan every tracked Go/TS/TSX file at HEAD. |
| `--worktree` | | Analyze uncommitted work: read supported-language source and project configuration files from the working tree. Alone, it compares the working tree with `HEAD`. Must run from the repository root. See [Uncommitted work](#uncommitted-work---worktree). |
| `--format` | `text` | `text` or `json`. |
| `--scope` | `production` | `production` or `all`. |
| `--min-severity <level>` | none | Render only signals at or above `high`, `medium`, `advisory`, or `low` (ranked `high` > `medium` > `advisory` > `low`). `advisory` ranks below `medium`, so `--min-severity medium` also withholds `advisory` `architecture.layer_violation` and `architecture.layer_bypass` findings. Only `complexity.cognitive_complexity` and `complexity.max_nesting_depth` can be `high`, so `--min-severity high` shows only those two rules ([Severity](#severity)). No floor by default. Any other value is a usage error: exit `2`. See [Narrowing the rendered report](#narrowing-the-rendered-report). |
| `--top <N>` | none | Render only the first `N` signals in report order ([Report order](#report-order): lifecycle group first, then severity, confidence, and path with within-rule magnitude), after any `--min-severity` floor. `advisory` findings rank below every `medium` one in the same lifecycle group, so a cap can push out declared `architecture.layer_violation` and `architecture.layer_bypass` findings, just as `--min-severity medium` hides them. No cap by default. `N` must be a positive integer; anything else is a usage error: exit `2`. |
| `--build-target <pattern>` | empty | Go package pattern for production reachability. Silent no-op under `--scope all`. |
| `--project-config <path>` | `.lousy-agents/coach.json`, else a schema-valid root `project.json` | Explicit path wins and no other file is consulted. Otherwise `.lousy-agents/coach.json` at the git top-level, its content read from disk. Otherwise a schema-valid root `project.json`, always with `project_config_deprecated`. An invalid canonical file is an error and does not fall back. A TypeScript scan opens no guided authoring for it, with or without a terminal, because the only session it has ends by asking the user to save a candidate over the file; it exits `2` with no report and says in words that the policy on disk cannot be used: fix the file. On a terminal, a TypeScript scan offers guided policy authoring after the report when the file is valid but declares no layers and the scan exits `0`. A selected policy enables `schema_version: "2"`. An explicit path is taken from the repository root, whatever directory the scan starts in; an absolute path, one that climbs out with `..`, and one inside `.git` (where git tracks nothing) are refused with exit `2` and a message, and no authoring is offered for them. See [`.lousy-agents` config](./conventions/lousy-agents-config.md). |
| `--project-language` | `go` | `go` or `typescript`. The language selects the project backend when a policy is selected (explicit path or the discovery order above). With no policy selected, the flag does not start project analysis. |
| `--suggest-project-config` | | Without `--output`, writes `.lousy-agents/coach.json`. Go also prints the candidate on stdout. TypeScript writes the file after confirmation and does not print the candidate on stdout. `--output` still wins. Never auto-applied; never invents layers. |
| `--output <path>` | `.lousy-agents/coach.json` | Create-only path for a suggest candidate. When set, this path wins over `.lousy-agents/coach.json`. |
| `--check-project` | | Read-only TypeScript readiness. Exits `0` on gaps. Not a scan. It reads the project policy from the working tree, as `--project-config` or the discovery order selects it: a usable regular file there is the policy, whatever the index and the analyzed revision hold at its path, and the roots the check uses come from its bytes. How that file differs from the analyzed revision is a warning, never a gap (see the paragraphs below the exit codes). A file on disk whose content is not a usable policy (invalid, over the size budget, or holding conflict markers) is `policy_invalid` with no next action and the cause `undetermined`: authoring refuses a path that exists, so the way out is to fix the file. A policy gap names one next action only where nothing is on disk: `author_policy` for a policy that was never committed and that the index holds nothing for, or for one whose committed copy is a regular file whose content is not a valid policy or is over the size budget (guided authoring writes only at a path with nothing at it, so a policy that exists and cannot be used is not authored over: for Go the scan names a candidate written beside the file with `--suggest-project-config --output`, with words that say to compare it with the file, replace the file with it, commit and rerun, and no command to paste; a TypeScript scan opens no session for a policy that exists, with or without a terminal, because the only session it has ends by asking the user to save a replacement over the file), or `restore_policy` for an explicit `--project-config` that is committed at the analyzed revision and gone from disk, unless a restore would overwrite or recreate something or git cannot run it (a staged edit, an unmerged entry, a symbolic link, a sparse checkout, a skip-worktree mark, or a question git could not answer) or the path lies under a file in the way (the message names the file, and the policy is restored after it is moved). Neither is executable (`executable: false`), and a gap that a new file or a restore cannot fix names neither: that includes a path with nothing on disk where the index holds a staged edit, a staged copy or a conflict, or the analyzed revision commits a symbolic link (the cause is `undetermined`), a path under which the index or the revision holds files (`directory`), and a path inside a submodule checkout or committed as a submodule (`in_submodule`, which wins over a file on disk). A usable file on disk is never a gap for what the index or the revision hold at its path: untracked, edited, staged, committed as a symbolic link or conflicted, it passes and carries a warning. An index entry that holds no content (`git add --intent-to-add`, or an empty file staged) counts as nothing: over a committed policy the restore still brings back the committed copy. |
| `--prepare-compiler` | | Interactive consented **mise-only** TypeScript compiler setup. Requires a TTY and an attended session; refuses otherwise. It judges the project policy as `--check-project` does, from the working tree, refuses with `a project policy that passes --check-project is required before compiler setup (<gap code>)` while the policy has a gap, and prints the policy warnings before its prompt, with no extra confirmation. Also refuses, naming the gap, for a runtime-boundary gap (`node_missing`/`node_unsupported`) — the same restriction the scan path's own compiler-setup offer applies, so the two cannot disagree. It is narrower than the scan's offer, which also runs the project package manager: a repository whose only executable choice is `project_package` is never handed this command as remediation, because it would exit `0` reporting nothing to set up. |
| `--no-interactive` | `false` | Treat the invocation as unattended even with a controlling terminal attached. On a scan, the interactive compiler-setup and guided-policy-authoring offers are skipped, falling through to the same message-only remediation a piped invocation gets; the TypeScript form of `--suggest-project-config`, and `--prepare-compiler`, refuse with exit `2` instead of prompting (the Go form of `--suggest-project-config` never prompts, and rejects the flag as a usage error). When a legacy root `project.json` would otherwise prompt for a copy, the unattended invocation skips that prompt and emits `project_config_migration_skipped` with `top="$(git rev-parse --show-toplevel)" && mkdir -p "$top"/.lousy-agents && git -C "$top" mv -- project.json .lousy-agents/coach.json`. Also triggered by a non-empty `CI` environment variable, so a pty-allocating runner cannot hang on an unanswered prompt. |
| `--fail-on-incomplete-coverage` | `false` | Exit `3` when required project coverage (model or bypass phase, on any analyzed side) is incomplete — including when the project backend is unavailable. A Go report records no provenance, so for Go this reads `project_coverage.complete`, which folds the model and any `required_layer` bypass search, and, for the base side of a diff, the coverage diagnostics. Go cases that count as incomplete include a repository with more than 5,000 tracked files of any type (Go discovery counts every file), a `required_layer` prefix that matches no function, and no `go` on `PATH` when `required_layer` is set. The report is always written to stdout first. Valid only on scan modes (`--base`, `--baseline`, `--worktree`). |

Requires `git` on `PATH`. Analyzes **committed Git objects**, not the dirty
worktree, unless `--worktree` is passed; the project policy is the exception, read from the working tree. TypeScript project mode also inspects
worktree `package.json` / `node_modules` / `mise.toml` for compiler and Node
readiness.

`coach codesignal` takes no positional operands. An unexpected positional
argument is a usage error: exit 2, usage on stderr, and nothing on stdout.

## Exit codes

| Code | When |
| --- | --- |
| `0` | Completed analysis, readiness report, successful suggest/prepare, or `--help` / `--version`. Signals, and narrowing the report with `--min-severity` or `--top`, do not change this. |
| `1` | Operational: not a git repo, missing `git`, unresolvable `--base`, empty repo, I/O. Also `--check-project` when no usable policy is on disk and git cannot answer a question the check needs for an explicit `--project-config` (whether it is committed at the analyzed revision, what the index holds for it, or whether the revision holds it in another case): stdout is empty and stderr names the question and what git said, because a verdict built on an unanswered question could name a restore or a new policy over work it never read. A usable policy on disk needs no such answer: it passes, with the warning `project_policy_head_unreadable` where git cannot say what the revision holds. Also, in a partial clone: with a git older than 2.44, which cannot be told not to fetch, a revision the scan reads that lacks objects is refused before anything else runs, and this does not depend on the repository's configuration, since git takes a remote with `remote.<name>.partialclonefilter` for a promisor whatever `remote.<name>.promisor` says (a repository that lacks nothing the scan reads is read as usual), and a scan or `--check-project` that needs an object the clone lacks fails, because Coach never fetches (see [What repository configuration can run](#what-repository-configuration-can-run)). |
| `2` | Usage (including an invalid `--min-severity` or `--top` value), invalid `--project-config` (empty stdout, stderr names the path/revision), a `--project-config` that does not exist on disk (empty stdout, stderr names the path and `project_config_missing`, no fallback to another policy; a committed policy gets the one `git restore` command that brings it back unless a restore would overwrite or recreate something or git cannot run it (a staged edit, whose message names the restore from the index that keeps the edit and, as the labelled way to drop it, the restore from the revision; an unmerged entry, a symbolic link, a sparse checkout, a skip-worktree mark, or a question git could not answer); a policy that was never committed but that the index holds gets the command that restores the staged copy from the index when it is a staged regular file, and the reason with no command when it is unmerged, is not a regular file, or is marked skip-worktree or left out by a sparse checkout; a path under a regular file gets the cause and the file in the way with no restore command; and none of these is offered authoring) or that is unusable on disk: a symbolic link, a directory or other file that is not a regular file, a path under a symbolic link, a file that cannot be read, or a parent that cannot be searched (`project_config_invalid`, naming the reason), unsupported `--project-language`, or a TypeScript **scan** that cannot resolve a supported compiler or host Node (empty stdout except the policy-candidate document on the guided-authoring path; one or more stderr lines). |
| `3` | Required project coverage (model or bypass, on any analyzed side) is incomplete **and** `--fail-on-incomplete-coverage` was supplied — the report is still written to stdout. Under `--worktree` it also covers part of the working tree that is in scope and was not read: a `worktree_scan_incomplete` entry whose `hides_in_scope` is `yes` or `unknown`, or a changed file that could not be read. An entry with `hides_in_scope` of `no` exits `0` ([Uncommitted work](#uncommitted-work---worktree)). Also: `--prepare-compiler` operational failure resolving the baseline revision or computing readiness (a deliberate exception to the scan contract's `1`, kept because the flag follows suggestion mode's table). |

`--check-project` exits `0` so you inspect the payload; do not treat that
status as a gate.

`--check-project`, `--prepare-compiler` and the compiler setup a TypeScript scan offers
judge the project policy from the working tree, as `--project-config` or the discovery
order selects it. A usable regular file there is the policy, whatever the index and the
analyzed revision hold at its path, and the roots they use come from its bytes. A root
`project.json` that discovery falls back to counts only when it is a valid policy, and is
read with `project_config_deprecated`. The policy's own path is never in
`dirty_worktree`, and `relevant_changes` is `false` unless another path counts.

How the file on disk differs from the analyzed revision is a warning, never a gap, and no
warning changes `status`. Each is an entry `{code, root}` of the JSON `warnings` array and a
line of the `Warnings:` section of the text output (`project_policy_not_committed
(root=<path>)`). The compiler setup prints it before its choice prompt
(`TypeScript compiler setup: warning <code> (root=<path>): ...`) and asks nothing more
because of it. The codes:

- `project_policy_not_committed`: the file is on disk and the analyzed revision holds
  nothing at its path.
- `project_policy_differs_from_head`: the revision holds a different copy of the file, holds
  a symbolic link or another entry that is not a regular file at the path, or the index
  holds the path as an unresolved merge conflict.
- `project_policy_head_unreadable`: the revision's copy cannot be read (a blob that a
  partial clone lacks, an index that another git process holds locked, or a question that
  git fails), so the two cannot be compared.

Each code names the same relation as `scope.policy.cause` in a scan, not the same word: `not_committed` is `project_policy_not_committed`; `differs_from_head`, and a policy that the revision commits as a link or holds in conflict, are `project_policy_differs_from_head`; `head_unreadable` is `project_policy_head_unreadable`. A policy committed as a symbolic link with a regular file on disk is `not_committed` in a scan and `project_policy_differs_from_head` here.

The file is read from the working tree and compared with the committed blob, and git is not asked for a diff, so nothing is written to the index and no hook runs; a skip-worktree or assume-unchanged mark does not hide an edit. Bytes that differ are the committed copy all the same when git's own conversion of the file for its path gives the blob (the line endings that `core.autocrlf` and the `text` and `eol` attributes change, `$Id$` expansion and working-tree encoding), so a checkout made on Windows is not a file that differs. Git is asked for that conversion only when no `filter` attribute is set for the path, and it refreshes no index, takes no lock and runs no hook or program. A file that a `filter` attribute selects a program for, or that git cannot be asked about, is taken to differ, because no program runs on a scan.

Where no usable file is on disk, the index and the analyzed revision decide the way out, and
a restore from the revision is offered only where it overwrites nothing. Where the index
holds work for the path (a staged edit, a staged copy, a staged link or an unmerged entry),
the cause is `undetermined` and the restore from the revision, which would discard that
work, is never named. The `--prepare-compiler` refusal then names the restore from the
index that keeps a staged edit or copy, `git restore -- ':(top,literal)<path>'`, and for
any other entry says why and names no command. A file on disk whose content is not a
usable policy is `policy_invalid` with no remedy and the cause `undetermined`, and the
refusal says to fix the file.

A policy gap that names neither `author_policy` nor `restore_policy` says why in a
`cause` on the gap, and the text prints the same word after the code
(`policy_missing cause=undetermined`). The field is absent from every other gap,
and absence has one meaning: the gap is not a policy gap, or a restore or a new
policy fixes it, which `next_actions` names (`restore_policy` or `author_policy`).
A policy gap with no way out always has a `cause`; `undetermined` is the word for a
state Coach has no other word for, which includes a file on disk that is not a usable
policy and a path with nothing on disk where the index holds work for it, so a consumer
never has to read a missing `cause` as "unknown". The values `--check-project` emits are
stable: `in_submodule` (the policy lies inside a submodule checkout, present or
missing, even when a name below the submodule differs in case from the checkout's, or the analyzed revision commits the path itself as a submodule: the superproject
records the submodule as one commit, so the file cannot be committed here, and the way out is a
policy this repository commits), `parent_not_directory`, `skip_worktree` and
`sparse_checkout` (both only where no usable file is on disk: a file that is there passes),
`directory` (the index or the analyzed revision holds files under the name) and
`undetermined`. A path on disk
that cannot be a policy has one word for each fix: `not_a_regular_file` (a directory, a pipe
or another entry on disk, or an entry of another kind that the analyzed revision commits; name
a regular file), `symbolic_link` (the policy is a link or is reached
through one; name the file it points to), `unreadable_on_disk` (the process cannot stat or
read it; fix the permissions) and `name_case_differs` (the revision commits the policy in another case than the path Coach reads; make git record it at that path).

The words `not_committed`, `staged_edit`, `staged_copy`, `changed_on_disk`,
`staged_not_a_file`, `merge_conflict`, `committed_as_link`, `index_locked` and
`git_unreadable` say what the index or the analyzed revision holds at a path. A file on disk
makes that a warning and not a gap, and where nothing is on disk the cause is `undetermined`,
so `--check-project`, `--prepare-compiler` and the compiler setup of a scan never print
them. The scan's own refusals of an explicit `--project-config` may still name some of them.

The analyzed revision decides what a policy path names, for a policy named with `--project-config` and for the one Coach finds alike. When the revision commits exactly the path as written, that is the policy that is read, whatever spelling the disk shows for the file and whatever `core.ignorecase` says: with `cfg/p.json` named and committed, a disk that shows `CFG/p.json` or `cfg/P.json` is no reason to refuse, and v0.11.0 read it. The disk is asked only whether a regular file is at the path (a symbolic link, a directory, an unreadable file and a file where a parent directory belongs are refused as before); it is not asked how the name of that file is spelled. <!-- TODO(#168 A2b/A3): the revision no longer decides what a policy path names once the disk does (ADR-009 s5); rewrite when A2b-1 (on-disk spelling) lands -->

When the revision does not commit the path as written but commits the same path in another case, the policy is not read. The scan exits `2` with `project_config_invalid`, and `--check-project` reports `policy_invalid` with the cause `name_case_differs` and offers no `author_policy`, because a new policy cannot be written over the one that is committed. One message serves a policy named with `--project-config` and a policy Coach finds, in a scan and in `--check-project`, on a file system that folds case and on one that tells cases apart, and whatever `core.ignorecase` says, because the committed names are what differ. With one committed spelling it says that the policy is committed as the spelling it names, but Coach reads the path it names, so it is not read, and that git has to record the policy at that path: rename it in the repository, or commit a rename you have staged, then commit and run again. It states the step in words and prints no command. It always warns that if another file already exists at that path it is to be moved aside first, because with `core.ignorecase` set git can overwrite it. A rename that is already staged is told the same as one that is not, on every file system, because the revision still holds the old spelling until the rename is committed. A policy named with `--project-config` has one more way out, because a name typed in another case is often a slip and renaming the committed policy to match it would break everyone who passes the committed spelling: the message first says to pass the committed spelling, or one of them when several are committed, and then says to make git record the policy at the name given. The path Coach finds has one spelling it can be read at, so for it renaming is the only way out and the message does not offer to pass anything. A policy named with the default policy's path in another case (`.lousy-agents/COACH.json`) is not told to make git record the policy at the spelling typed, because a scan without `--project-config` reads only `.lousy-agents/coach.json` and would then refuse it. It is told to pass the committed spelling or to leave the flag off; when the revision commits the default spelling beside others it is told to keep that one and delete the others, and the default spelling is never the one to delete; when only another spelling is committed, git is to record the policy at the default spelling. Where git is to record the policy at the default spelling instead of the one typed, the message names the default spelling throughout, so with several spellings committed it says to keep one of them at the default spelling and not at "that path", and it ends by saying that the next run is to pass the default spelling or leave `--project-config` off, because the flag as typed would still refuse the policy. When the rename changes the case of a directory's name, the message adds that git cannot do that in one step on a file system that folds case, and says to rename the directory to another name first and then to the one wanted. When the revision commits the policy under several spellings, the message lists every one of them (up to 16, then "and more"), says to keep one of them at that path and remove the others, names the one to keep and the ones to delete, and gives the same warning; past 16 spellings it names none to keep, and says only to keep one of them at that path and remove the others.

Nothing is refused when the revision commits the path as written, whatever else it commits beside it, or when it commits nothing like it. A policy that is gone from disk while the revision commits it in another case gets the same message, and no restore and no new policy is offered, because either would write into the other spelling. A file in the worktree that the revision commits in no spelling at all is not a case problem: on a file system that folds case it is the file at the path and is not committed (`not_committed`), so a policy named with `--project-config` is refused with `project_config_invalid` and "exists in the worktree but is not committed", and a discovered one is skipped with a diagnostic that says so, while on a file system that tells cases apart a file spelled in another case is not at the path, so the policy is missing and `author_policy` is offered. The same holds for every component of the path: a missing policy under a parent typed in another case that the revision holds no spelling of is simply missing, and is not refused for the parent.

A policy that Coach discovers, with no `--project-config`, is read only at `.lousy-agents/coach.json`. When the revision does not commit a policy there as written and commits the same path in another case (`.Lousy-Agents/Coach.json`, say), the scan is refused with the message above, and `--check-project` reports a policy gap with the cause `name_case_differs` and no `author_policy`. Before, the scan ran without the policy and said nothing, which dropped its layer checks. It holds on any file system, since the committed names are what differ.

Only the policy's own path leaves `dirty_worktree`. It does so whether the file is usable, unusable
or absent: the working tree decides the verdict, so a staged edit it could not supply does
not stay listed. Every other uncommitted path the report did not read still counts when it
lies under the roots of the policy. The roots are those of the policy read from the working
tree. A policy the working tree refused or lacks has no roots of its own, so they come from
its committed copy at the analyzed revision. When nothing valid is committed there
are no roots to take, and only package and compiler metadata (`package.json`,
lockfiles, `.npmrc`, `bunfig.toml`, `tsconfig*`) counts.

On a controlling terminal, a TypeScript scan whose `--project-config` names a
policy that does not exist on disk and was never committed opens a
guided-authoring session instead of only printing a remediation line. A policy
that is committed, or that the index holds, gets what the README's explicit-policy
cases describe instead (a restore command where one works, otherwise the reason),
and no authoring. With
`--format json` this session's exit-2 output is a JSON object carrying
`"schema_version": "1"` and `roots`, not a CodeSignal report — the absent
`scope`/`signals`/`coverage` keys distinguish it. A scan cannot pass
`--output`, so the approved candidate is written to **stdout** and no file is
created: save it to the `--project-config` path yourself, review it, commit
it, then rerun.

Prompts and setup transcripts go to stderr. Redirecting stderr away from your
terminal (`2>run.log`) therefore hides the prompts while the session still
waits for an answer; pass `--no-interactive` when you redirect stderr and do
not intend to answer.

Every command Coach appends as remediation on that exit-2 path is itself
interactive, so each is printed prefixed with `on a terminal:` — running one
from the same piped context that printed it exits `2` without making progress.
With no policy, `checks.project_shape` and `checks.package_manager` report
`not_checked` rather than guessing from the worktree root in place of the
roots a policy would have selected, so every other gap readiness reports
alongside a missing policy is already independent of that missing policy —
the line simply names the `--check-project` rerun with no hedge about
whether committing a policy would change it.

On a controlling terminal, a TypeScript scan with a policy that passes
`--check-project` whose compiler check fails offers interactive setup covering everything
`--prepare-compiler` runs standalone, plus the project package manager. It
names the failing check and states that a confirmed, successful choice makes
Coach recheck readiness and resume the same scan, then prints the policy warnings of `--check-project`, which
say how the policy on disk differs from the analyzed revision, and shows a menu of up to
four choices (`project_package`, `project_mise`, `global_mise`, `cancel`),
each labelled with what it would touch. A choice that would open only to
report nothing to do is withheld, and a gap offering nothing installable never
opens a prompt — it reports why each choice was ruled out instead. A
`project_package` install whose policy selects more than one manifest context
is withheld too: one consented command runs in one directory, and the
readiness rerun would still report the gap.
Selecting `project_package` shows a preview (executable, arguments, working
directory, expected on-disk changes, network disclosure, script-suppression
policy, and `SetupPreviewTimeout` = 5 minutes) and requires a separate,
single-use `confirm`/`install` answer before running; any other answer, or a
second leftover answer, cancels or is ignored. Cancelling or failing exits 2
with no report and (on failure) no change to the repository's tracked files.
Succeeding reruns the full readiness check and continues the same scan to a
report only if that rerun reports no gap; the offer runs at most once per
invocation even then.

Without a controlling terminal, a compiler gap whose only genuinely
executable setup choice is `project_package` cannot be resolved by
`--prepare-compiler` — it runs mise scopes only — so the scan instead
appends `on a terminal: coach codesignal --baseline --project-config <path>
--project-language typescript -- offers project-package setup`, naming the
same scan invocation rather than a standalone subcommand: rerunning it on a
terminal is what opens the combined setup offer above.

## Project policy

Opt into cross-file layer checks with a committed JSON config. Coach does not guess your architecture: you declare layers and forbidden import edges.

```sh
# Go: discover module/workspace roots (candidate only; review, add layers, commit)
coach codesignal --baseline --suggest-project-config

# after the file is committed at the analyzed revision
coach codesignal --base main --project-config .lousy-agents/coach.json
coach codesignal --baseline --project-config .lousy-agents/coach.json --project-language typescript
```

Move a root `project.json` to that path, from any directory of the repository:

```sh
top="$(git rev-parse --show-toplevel)" && mkdir -p "$top"/.lousy-agents && git -C "$top" mv -- project.json .lousy-agents/coach.json
```

An explicit `--project-config` still reads only that path. A schema-valid root `project.json` is still read with a deprecation diagnostic until the follow-up removal ([issue #511](https://github.com/lousy-agents/coach/issues/511)). The lookup is [`.lousy-agents` config](./conventions/lousy-agents-config.md).

`--project-config` content is read from the working tree, and `scope.policy` discloses how it differs from `HEAD`. The path itself is checked on disk first, and what is there decides what Coach says. Every case below exits `2`, and stdout stays empty except for the policy-candidate document on the guided-authoring path. <!-- TODO(#168 A2b-1): the refusal list below still names the staged copy, merge conflict, index lock and committed-as-link causes of the scan refusals. Rewrite when A2b-1 (disk-judged scan refusals) merges -->

- Nothing is there, the policy was never committed, and the index holds nothing
  for it: `project_config_missing`. A Go scan then prints
  `--suggest-project-config`. A TypeScript scan on a controlling terminal opens
  guided authoring, and `--check-project` names `author_policy`. An index entry
  that holds no content, made with `git add --intent-to-add` or by staging an
  empty file, counts as nothing, unless it is marked skip-worktree: git add
  refuses a new file at a path the mark hides, so Coach calls it an empty entry,
  names the mark (or the sparse checkout), and offers no new policy.
- Nothing is there but the policy is committed at the analyzed revision:
  `project_config_missing`, with the one command that brings it back,
  `git restore --source=<revision> --staged --worktree -- ':(top,literal)<path>'`,
  and no authoring. `--check-project` names it as the `restore_policy` next
  action, with `executable: false`. Coach offers that command only when it
  overwrites nothing and git can run it. If the index holds a staged edit to the
  policy, the restore from the revision would discard the edit, so Coach does not
  offer it as the default. It says why and offers no new policy, and it prints two
  choices: the restore from the index, which keeps the edit, and, labelled as the
  way to drop it, the restore from the revision, to drop it and bring back the
  committed copy. If the index holds an unmerged entry for the policy, if the
  policy is committed as a symbolic link, if a sparse checkout leaves it out of the
  working tree or its index entry is marked skip-worktree, or if git cannot say
  whether it is committed or what the index holds, Coach says why and offers
  neither a restore nor a new policy.
  An index entry that holds no content does not count as an edit: the restore
  replaces it, and the message says that it brings back the committed copy,
  unless the entry is marked skip-worktree, which stops a restore and is named
  instead.
- Nothing is there and the policy was never committed, but the index holds an
  entry for it: `project_config_missing`, and no authoring, because a new policy
  would replace staged work. A staged regular file gets the one command that
  brings it back from the index, `git restore -- ':(top,literal)<path>'`. Coach
  prints no command, and says why, when the entry is unmerged, is a symbolic link
  or another entry that is not a regular file, or is marked skip-worktree or left
  out by a sparse checkout, because the command would fail or would recreate what
  the scan refuses. `--check-project` names neither next action.
- A path under a regular file: `project_config_missing`. The message names the
  file in the way. When the analyzed revision commits that file, the message says
  it is a file the analyzed revision commits, so the path is probably wrong, and
  gives no advice to move or remove it, because the file is the repository's and
  the path is what is wrong. For a file nothing commits, the message says to move
  or remove it, then to restore the policy from git when it is committed, unless
  the index holds a staged edit to it, which the message names instead because a
  restore would discard it, or it is committed as a symbolic link, which the
  message says instead because a restore would recreate a link the scan refuses.
  When the policy is only staged, never committed, it says to move the file and
  then restore the staged copy from the index. When the index holds an unmerged
  entry for the policy, the step that brings the conflicted file back
  is described behind the step that moves the file, in words and with no command,
  because bringing it back would delete that file as well. It prints no `git restore`
  command and offers no authoring, because a restore would delete that file.
- A symbolic link, a directory or other file that is not a regular file, a path
  that runs through a symbolic link, a file that cannot be read, or a parent
  that cannot be searched: `project_config_invalid`, naming the reason, with no
  restore and no authoring.
- A policy path that the analyzed revision does not commit as written, while it
  commits the same path in another case (`CFG/p.json` named, `cfg/p.json`
  committed, or `.Lousy-Agents/Coach.json` committed where Coach reads
  `.lousy-agents/coach.json`): `project_config_invalid`, and `--check-project`
  names `name_case_differs` and offers no new policy. One message serves a
  policy you name and one Coach finds, on any file system and whatever
  `core.ignorecase` says: it names the committed spelling and the path Coach
  reads and says in words to make git record the policy at that path, then
  commit and run again. It prints no command, and it always warns that a file
  already at that path is to be moved aside first, because git can overwrite it
  with `core.ignorecase` set. A rename you have already staged is told the same
  as one you have not, because the revision still holds the old spelling until
  you commit. A policy you name is also told to pass the committed spelling,
  which is the way out for a slip of the case, and a directory renamed in case
  only is told to take two steps on a file system that folds case. A revision
  that commits exactly the path as written is read
  whatever spelling the disk shows. See the [CLI contract](#commands).
- A policy inside a submodule checkout, or at a path the analyzed revision
  commits as a submodule, naming the submodule: `project_config_invalid` while
  the checkout is there (the file in it, or the submodule itself), and
  `project_config_missing` once it is gone (`git submodule deinit`).
  `--check-project` names the cause `in_submodule` for all of them, and so for
  a name below the submodule that differs in case from the checkout's: the
  superproject holds no spelling of what is inside, so Coach does not report a
  difference in case there. A name that spells the submodule's own path in
  another case than the index records it, where git ignores case
  (`core.ignorecase`), is inside it all the same, and the message names the
  submodule as the index records it. The superproject records a submodule as one commit,
  so the file can be neither restored nor committed here. Name a policy this
  repository commits. See the
  [CLI contract](#commands).

An uncommitted policy file is read from disk like a committed one. It passes with a warning: `project_policy_not_committed` in `--check-project`, and `scope.policy.cause` `not_committed` in a scan (see [Policy disclosure](#policy-disclosure-scopepolicy)). A valid config switches the report to `schema_version: "2"` and can emit `architecture.layer_violation`. `required_layer` can also emit `architecture.layer_bypass` (Go and TypeScript) on a narrow handler→SQL registry.

TypeScript `--project-config` uses Coach's embedded analyzer and a host-resolved
compiler whose own `package.json` version is in the supported set (`7.0.2`
today). Version ranges (`^`, `~`, `latest`) never count. If you previously
vendored Coach's analyzer under the analyzed repository, you can delete that
copy. `--suggest-project-config` never invents `layers` or `forbidden_imports`,
and it never auto-applies a config.

Go `--suggest-project-config` prints a candidate. TypeScript
`--suggest-project-config --project-language typescript` is an **interactive
TTY session** and writes nothing until you confirm. Do not run it unattended.

## TypeScript project readiness

```sh
coach codesignal --baseline --check-project --project-language typescript
coach codesignal --baseline --suggest-project-config --project-language typescript
```

Run `--check-project` first. It is read-only: it reports compiler, Node, and
policy fit and exits `0` so you inspect the result instead of treating process
status as a gate. It is not a scan. It reads the project policy from the working
tree: a usable regular file at the policy's path is the policy, whatever the index and
the analyzed revision hold there, and its roots come from that file. How the file
differs from the analyzed revision is a warning, never a gap, and it does not change
`status`: `project_policy_not_committed`, `project_policy_differs_from_head` or
`project_policy_head_unreadable`, in the JSON `warnings` and the text `Warnings:`
section. A file whose content is not a usable policy is `policy_invalid` with no new
policy to author: fix the file. Where nothing is on disk and the index holds work for
the path, the cause is `undetermined` and no restore from the revision is offered over
that work. A **scan** that cannot resolve a supported
compiler or host Node exits `2` with empty stdout except the policy-candidate
document on the guided-authoring path, and one or more stderr lines. On a
controlling terminal, a TypeScript scan whose `--project-config` names a
policy that does not exist on disk and was never committed instead opens a
guided-authoring session (a policy that is committed, or that the index holds, gets what the cases above describe instead).
With `--format json` this session's exit-2 output is
a JSON object carrying `"schema_version": "1"` and `roots`, not a CodeSignal
report. The absent `scope`, `signals`, or `coverage` keys distinguish it.

Coach's supported Node majors are exactly `24` and `26`. Any other major is the
`node_unsupported` readiness gap. `@lousy-agents/coach-semantics` declares this
as `engines.node: "^24 || ^26"`.

`--prepare-compiler` is a consented, interactive mise TypeScript compiler-setup
session (requires a TTY, and refuses otherwise). It prompts before installing. It
also refuses, naming the gap without opening a menu, when the readiness gap is
a runtime boundary (missing or unsupported host Node): Coach has no setup
command for a Node version problem, and the flag shares that gate with the
scan's own compiler-setup offer so the two paths cannot disagree on it. The
flag judges the policy the same way as `--check-project`, refuses while it has a gap,
and prints the policy warnings before its prompt, as the scan's offer does. The
flag remains **mise-only**, so it is narrower than the scan's offer in what it
can install. Coach never names it as remediation for a gap only the project
package manager could resolve.

A **scan** (`--baseline` or `--base` with `--project-config`, `--project-language
typescript`, a policy that passes `--check-project`, and an unresolved compiler) offers a superset
of that setup interactively when it runs on a controlling terminal: it names
the failing check and what happens after a successful install, then shows a
menu of up to four choices,
`project_package` (run the project's own package-manager install, e.g. `npm
ci --ignore-scripts`), `project_mise`/`global_mise` (run `mise install` for a
project- or globally-pinned supported TypeScript version), and `cancel`. A
choice that would open only to report nothing to do is withheld from the
menu, and a gap with nothing installable never opens a prompt at all. Picking
`project_package` shows a preview (exact executable, arguments, working
directory, expected on-disk changes, network use, and a bounded 5-minute
timeout) before a separate, single-use `confirm` or `install` answer is
required to actually run it. Typing anything else, or a second leftover
answer, cancels or is ignored rather than running twice. On cancel, nothing
runs and the scan exits 2 with the original remediation. On failure, the scan
exits 2, reports the failure, and never modifies the repository's tracked
files. On success, Coach reruns the full readiness check and only continues
the same scan through to a report if that rerun is clear. A rerun that still
reports a gap still exits 2 with no report.

Pass `--no-interactive`, or set a non-empty `CI` environment variable, to make
Coach treat the run as unattended even when a real controlling terminal is
attached, for example, a pty-allocating CI runner (`docker run -t`, `ssh -t`)
with nobody at the keyboard. A scan then falls straight through to the same
message-only remediation a piped invocation gets, and
`--suggest-project-config` or `--prepare-compiler` refuse with exit `2` rather
than opening a prompt nobody will answer.

Still being built. Do not expect these to work yet: unattended package-manager
setup for the scanned project, Bun as a project runtime, and a packaged
foreign-repository TypeScript journey.

## Agent self-review hook (preview)

The hook in the [README](../README.md#agent-self-review-preview) runs `coach codesignal --worktree` when a Claude Code session is about to stop. It blocks the stop with a message when the review did not run or was not clean. `--worktree` is labelled experimental. The PRD change that brings this job into scope is [#730](https://github.com/lousy-agents/coach/pull/730).

Each flag in the hook's command has a reason:

- `--no-interactive` makes the run unattended. Without it, a TypeScript scan could open a setup prompt that no one answers. With it, the scan falls through to message-only remediation ([--no-interactive](#commands)).
- `--fail-on-incomplete-coverage` turns incomplete coverage into exit `3`, so an unread part of the tree cannot read as clean ([Exit codes](#exit-codes)).
- `--format json` lets the hook read `scope.policy.matches_head`.

The hook's exit-code handling:

| Exit | What the hook does |
| --- | --- |
| `0` | The review ran. It is clean unless `scope.policy.matches_head` is `false`. A report with no `scope.policy` key makes the hook block, because the policy check could not run. `"policy":null` (no policy selected) passes. |
| `1`, `2`, `127` | The review did not run: an operational failure (`1`), a usage error such as running outside the repository root (`2`), or `coach` not on `PATH` (`127`). The hook says so and blocks. |
| `3` | Required coverage is incomplete: a project-level gap, or part of the in-scope working tree was not read. Not clean. |
| any other | Not clean. |

`matches_head: false` counts as not clean. The hook needs a coach build that reports `scope.policy`: the v0.12.0 release does not, so on that release the hook blocks with a message instead of passing without the check. The policy file is read from disk, so an uncommitted edit that loosens `.lousy-agents/coach.json` would otherwise be reviewed against itself and pass. The hook covers uncommitted work only: a policy or code change already committed is not flagged, so review it in the pull request. `scope.policy.cause` says why the policy differs: `differs_from_head` or `not_committed` ([Policy disclosure](#policy-disclosure-scopepolicy)).

Clean does not mean no findings. The hook does not block on findings, which stay advisory, and it passes no findings to the agent: its message only says whether the review ran and was clean.

Claude Code sets `stop_hook_active` to `true` in the hook's input when a Stop hook has already continued the session. On that second attempt the hook exits `0`, so it cannot block the same stop forever. It enforces one nudge per stop, not a guarantee. Claude Code does not pass stderr from a hook that exits `0` to the model, so the reason is not shown to Claude on that attempt. The first block carries it. Claude Code also caps consecutive stop-hook continuations, as described in the hooks reference.

The hook needs `coach` on `PATH` (see [Install](#install)) and uses `CLAUDE_PROJECT_DIR` as the repository root. To test it by hand, pipe input to it, for example `echo '{}' | sh .claude/hooks/coach-self-review.sh`, because the hook reads its input from stdin.

## Git version

Coach runs git for every read, and passes options that only a recent git knows. Git 2.42 or later is the version to use.

| git | what Coach uses | on an older git |
| --- | --- | --- |
| 2.15 | `--no-optional-locks`, passed to every git call so that a read never takes the index lock | every scan exits 1 at once, whatever the repository, because a git that does not know the option fails the question of its own version too: `could not determine the version of git (exit status 129: "unknown option: --no-optional-locks\nusage: git [--version] [--help] [-C <path>] [-c <name>=<value>]"); Coach needs git 2.16 or later` |
| 2.16 | `rev-list --missing=print` and `--filter=blob:none`, asked of every repository on a git older than 2.44 (see [what repository configuration can run](#what-repository-configuration-can-run)) | every scan on a git older than 2.44 exits 1: `could not tell whether this repository has the objects Coach reads (`, what git said about the option, and `)`; git's words differ by version, so none are quoted here |
| 2.18 | `git config --type=bool`, and `git status --no-renames`, so that the status of the working tree never compares a staged deletion with a staged addition (that reads the blob of each, and a git before 2.44 fetches one that only the index names) | the question whether a sparse checkout is on is not answered, and a policy that is off disk says that Coach could not check it; the status is run without the flag; the exit code is the same |
| 2.42 | `git cat-file --batch -Z`, with which `--baseline` reads each file it analyzes | a `--baseline` scan exits 0 and analyzes nothing: every file gets a `head_read_failed` diagnostic |
| 2.44 | `GIT_NO_LAZY_FETCH`, so that a read never fetches a missing object of a partial clone | a partial clone is scanned only when every object the scan reads is already there |

Under `--worktree`, git 2.44 is also the line for a staged rename (`git mv`): before it the status is run with `--no-renames` and the rename is two paths, `introduced` for the new name and `resolved` for the old one, with no continuity diagnostic; from 2.44 the rename is detected and reads `unknown` with `continuity_not_determined`.

Of these, v0.11.0 passed only `cat-file -Z` (in `--baseline`), so the 2.42 line is not new. The 2.15, 2.16 and 2.18 lines are: v0.11.0 passed none of those options, and this release needs them on every scan.

## What repository configuration can run

A scan reads the repository through git with `core.fsmonitor` disabled and without taking the index lock, and it runs no hook, external diff, textconv, pager or askpass program. It also removes from git's environment the settings of the caller that change what the pathspecs it builds mean or which file `git config` reads: `GIT_LITERAL_PATHSPECS`, `GIT_GLOB_PATHSPECS`, `GIT_NOGLOB_PATHSPECS`, `GIT_ICASE_PATHSPECS` and `GIT_CONFIG`. Everything that says where the repository is stays as the caller set it, as in v0.11.0 (`GIT_DIR`, `GIT_WORK_TREE`, `GIT_COMMON_DIR`, `GIT_INDEX_FILE`, `GIT_NAMESPACE`, `GIT_CEILING_DIRECTORIES`, `GIT_DISCOVERY_ACROSS_FILESYSTEM`, `GIT_OBJECT_DIRECTORY`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`), so a work tree whose git directory is elsewhere is scanned through them, and so are the objects of a server-side hook, which arrive through a quarantine directory and the repository's own. Configuration the caller supplies through `GIT_CONFIG_COUNT` with its keys and values, `GIT_CONFIG_PARAMETERS`, `GIT_CONFIG_GLOBAL`, `GIT_CONFIG_SYSTEM` or `HOME` is left too: every git command reads it alike, Coach's partial-clone check included, and a container that sets `safe.directory` that way needs it. `GIT_CONFIG` is removed because it makes `git config` read a single file and no other git command honours it. `GIT_NO_REPLACE_OBJECTS` and `GIT_REPLACE_REF_BASE` are left as the caller set them. Git sets `GIT_DIR`, `GIT_WORK_TREE` and `GIT_INDEX_FILE` for a hook, so a pre-commit hook that runs Coach scans the repository git gave it, and during a partial commit it reads git's temporary index and not the repository's own: that index is a lock file, which Coach reports as a locked index and so offers neither a restore nor a new policy, and a staged edit that the partial commit leaves out is not disclosed. It reads the source snapshot as the committed blobs, byte for byte, with `git ls-tree` and `git cat-file`, so no smudge filter runs (a git-lfs install defines one globally, and a committed `.gitattributes` can select it for every file) and no line-ending conversion, `$Format$` substitution or `export-ignore` changes what is analyzed; the snapshot used to be `git archive`, which applied all of them. Like `git archive`, it refuses a tree that holds a git directory (`.git` in any case, and the spellings that NTFS and HFS+ fold to it: `git~1`, trailing dots and spaces, and ignorable code points) or a path component that is empty, `.` or `..`, with an error that names the path. The scan stops and does not skip the entry, because a git directory in the snapshot is a repository whose config names programs git runs, and a skipped entry would be a report on a tree other than the one committed. The `go list` that finds the production files for `--build-target`, and the load of a Go project's packages, run with `-buildvcs=false` and `GOTOOLCHAIN=local`, so go neither runs git in the snapshot nor fetches a toolchain its `go.mod` names. A repository whose `go.mod` requires a newer Go than the `go` on `PATH` therefore fails, even when the caller sets `GOTOOLCHAIN`: `--build-target` exits 1 with go's own message (`go.mod requires go >= X (running go Y; GOTOOLCHAIN=local)`), and for a Go project `project_coverage.complete` is false, with the diagnostic `project_callgraph_build_failed` carrying the same message, so the call graph and the `required_layer` search are reported incomplete. To run it, install the Go version the `go.mod` names, or put it first on `PATH`; Coach does not download it. The Go project analysis refuses a tree with such a path in the same way before it writes the tree to a temporary directory for `go list`, so one rule holds everywhere a committed tree is written to disk. That read ends after 10 minutes if git stops answering, with an error that names the bound. The listing of the tree that names the files (`git ls-tree -r -z`) is read as a stream, a record at a time, for the snapshot and for the discovery of the tracked files alike, so it has no limit on its size and a repository of any number of files is scanned, as it was when the snapshot was `git archive`; it ends after 2 minutes if git stops answering, with an error that names the bound. A scan holds an entry per file, a path and an object id, and not the listing. An object that is in the object store and that git cannot read (a loose object that does not inflate, say) is told apart from one that is missing: `git cat-file --batch` answers "missing" for both, and Coach reads the error git printed beside the answer, so a damaged object ends the scan as damaged, quoting git's words and naming `git fsck`, and offers no fetch, which would not replace it. (Project analysis lists the tree for its model separately, and holds that listing to 64 MiB.) A committed file larger than 32 MiB is never held in memory. A source, a Go module file or a JSON file that large is written to the snapshot in full, a chunk at a time, because `go list` parses it (an empty file in its place would fail the scan with a parse error of its own), and it is reported as `file_too_large` with its path by the read that analyzes it, in a `--baseline` scan and in a `--base` scan alike (a `--base` scan reads each changed file with `git show`, which holds a file to the same 32 MiB; the base side of a modified file is reported as `base_analysis_failed`, as it is for a file past the analyzer's own limit); any other file that large is read past, and its path stays in the snapshot as an empty file, so the tree keeps its shape and a `//go:embed` of it still resolves. A Go project whose policy sets `required_layer` is the exception: its layer-bypass search copies every file of the revision to disk through that same `git show`, so one file past 32 MiB, of any type, ends the scan with exit 1 and an error that names the file and the budget (`git stdout exceeded 33554432-byte budget`). v0.11.0 ends the same way. The snapshot writes at most 4 GiB in all (a file counts once for every path that names it): the files an analyzer reads are written first, the rest become empty files once the total is spent, and the scan fails only when the files an analyzer reads pass 4 GiB by themselves. A `--baseline` scan reads the content of each file it analyzes from one `git cat-file --batch`; each request ends after 30 seconds, as an error that names `git cat-file`, and a file larger than 32 MiB is not held and gets a `file_too_large` diagnostic. It also fetches nothing: in a partial clone, an object that is not in the local object store (a blob left out by `--filter=blob:none` or `--no-checkout`, say) is reported as unavailable instead of being fetched from the promisor remote, so no `GIT_SSH_COMMAND`, `core.sshCommand` or credential helper runs and no object is written. A scan that needs such an object fails with Coach's own message, which says how many objects the snapshot lacks, names the first, and gives the batch below, and `--check-project`, where no usable policy is on disk (a usable one passes with `project_policy_head_unreadable`), reports that it could not check whether the policy is committed, says that the policy's content is not in the partial clone and that Coach does not fetch objects, and names the `git cat-file -e '<revision>:<path>'` command that fetches it, and the batch that fetches every file of the revision (shown only when the names can be printed safely). Any other file the check reads from the revision, `package.json` for example, is explained the same way, and a scan that stops at missing files says how many and names the first, with the batch; fetching only the policy is not always enough. To run again, bring the missing objects in yourself, in your own shell where git may fetch them. For `--check-project`, run the command the message names: it fetches the policy's blob. A scan reads every file of the revision it analyzes (`HEAD`, and with `--base` the merge base of the base and `HEAD` too), so fetch what the revisions list with one command, without running any file through a filter. The message prints it, `for pass in 1 2 3 4; do ids=$(git rev-list --objects --missing=print --no-walk <revision> | sed -n 's/^?//p'); [ -n "$ids" ] || break; printf '%s\n' "$ids" | git fetch origin --no-tags --no-write-fetch-head --filter=blob:none --stdin || break; done`, where `origin` is the promisor remote, and with `--base` it lists the merge base and `HEAD` together as `{ git rev-list ...; git rev-list ...; }`. It repeats until nothing is missing, because a clone made with `--filter=tree:0` lacks the trees as well: the first pass fetches the trees of the revision, and only then can the next list the blobs they name. A pass that finds nothing missing ends it without running `git fetch`, which would otherwise read the remote's refspecs and update refs. One paste is enough for a `tree:0` clone, a `blob:none` clone and a diff. `git backfill` is the alternative in a git new enough to have it (it is absent from git 2.47). A clone made with `--filter=tree:0` lacks the root tree of `HEAD`, which nothing can be listed without, so on a git that never fetches (2.44 and later) a scan or `--check-project` is refused before anything is read, with `HEAD` as the revision that lacks objects; a git before 2.44 refuses it with the rest of the revisions it reads. `--no-walk` matters: without it `rev-list` walks the whole history and the fetch brings in every blob that ever was, which is what a partial clone exists to avoid. A diff (`--base`) compares each changed file with its version at the merge base of the base and `HEAD`, which a partial clone checked out at `HEAD` usually lacks: on any git, the run then exits 1 before it analyzes anything, naming the merge base, how many objects it lacks and the batch for the merge base and `HEAD`, and never records the changed files as unreadable and exits 0; `--fail-on-incomplete-coverage` does not change that. `git fetch` and `git fetch --refetch` do not help: in a `--filter=blob:none` clone they bring commits and trees, not the blobs the filter leaves out, and the scan fails the same way afterwards. Not fetching rests on `GIT_NO_LAZY_FETCH`, which git reads from 2.44. An older git ignores it and would fetch, so on it Coach first asks, for every repository and whatever its configuration says, which objects of the revisions the scan reads are missing, with `git rev-list --objects --missing=print --no-walk --quiet`, which lists them and fetches nothing: `HEAD`, and with `--base` the merge base of the base and `HEAD`. With `--base` it asks first, with questions that read no object, whether the commit the base names is here, because finding the merge base reads commits and that git fetches one it does not have: a base commit that is not here is refused before the merge base is asked for. If none is missing there is nothing to fetch and the scan runs as usual. If any is missing, or git cannot answer, Coach runs nothing else and exits 1, naming the git version, the revision and how many objects it lacks, and offering git 2.44 or later or the batch above. The `git status` that lists the uncommitted paths does not go into submodules on such a git (`--ignore-submodules=all`), because it starts a nested status in each populated submodule, which fetches into one that is a partial clone and lacks objects; a change inside a submodule is therefore not listed there, and is on a git that can be told not to fetch. A revision that does not resolve (a `--base` that names nothing or has no merge base with `HEAD`, a directory that is no repository, a repository with no commits) is not asked about: the scan ends on it and says so in the words it uses on a current git, because a newer git would not change it. An ordinary clone has nothing missing, so the question costs it a walk of the trees and nothing else. The configuration is only the wording of the refusal, because git takes more for a promisor remote than the settings Coach reads (a remote with `remote.<name>.partialclonefilter` is one, with `promisor` unset or false), and a read of a missing object from it would fetch. On a git that can be told not to fetch, a `remote.<name>.promisor` setting counts as the git it is written for: a key with no value, or any non-zero number, is a promisor remote, and one git cannot read is an error. The one channel left is a clean filter, reached by one read: when `git status` has to hash a tracked file that a repository's `.gitattributes` gives a `filter.<name>.clean` program, git runs that program as defined in the repository's own configuration, and coach does not override it, because disabling filters would make every git-lfs file read as modified. A file that is modified is hashed, and so is one that is not modified but whose index entry is racily clean, which is the state of a file just after the commit that recorded it. v0.11.0 hashed such a file once and then rewrote the index, which ended the racy state. A scan does not write the index (`--no-optional-locks` is what keeps it from taking the lock and running the `post-index-change` hook), so the file is hashed, and the clean program runs, on every scan until something else refreshes the index, for example a `git status` of your own. The comparison of a policy named with `--project-config` that is committed but cannot be used (invalid or over the size budget) with its copy in the working tree is not a second channel for a clean filter: Coach reads the file through the repository root and compares its bytes with the committed blob's, and only when they differ asks git for the `filter` attribute of the path and, when none is set, to hash the file for its path with git's own conversions, which refreshes no index, takes no lock and runs no hook or program; a file with a `filter` attribute is taken to have work in it and no filter runs. A scan does not ask it for a policy that is usable or gone. `--check-project` and the compiler setup ask it for a usable policy on disk whose bytes differ from the committed blob, with the same conversion and the same refusal of a `filter` attribute, to tell a file that differs from one that does not. The repository's own configuration is the trust boundary, as it is for git itself.

## Uncommitted work (`--worktree`)

`--worktree` analyzes work that is not committed. It is a local, pre-commit check, not a pull-request check. It exits `2` with `coach: --worktree must run from the repository root` and the `cd` to run when the directory is not the repository root.

| Invocation | Compares the working tree with |
| --- | --- |
| `--worktree` | `HEAD`. A signal absent at `HEAD` is `introduced`. |
| `--worktree --base <ref>` | The merge base of `<ref>` and `HEAD`, by the rules of a committed diff. |
| `--worktree --baseline` | Nothing. Every signal is `baseline`. |

What it reads:

- Supported-language source and project configuration files (for example `go.mod`, `go.work`, `package.json`, `tsconfig*.json`) come from the working tree. New and untracked files that git does not ignore are included. A deleted file is absent, not empty.
- A symbolic link is not followed. `worktree_symlink_skipped` names it.
- A rename staged with `git mv` depends on the git version. On a git before 2.44 the status runs with `--no-renames`, so the rename is two paths: the moved file's signals are `introduced`, the old name's signals are `resolved`, and there is no `continuity_not_determined` diagnostic. On git 2.44 or later the status detects the rename and it is selected as a committed rename is: its signals are `unknown`, with `continuity_not_determined`.
- A changed file that cannot be read, or that changes while it is read, is named by `worktree_changes_not_analyzed`. The message gives the cause: permission denied, not a regular file, over the size limit, a failed read, or a name on disk that differs from the name git records.
- `--scope`, `unsupported_language` and `coverage.unsupported` apply as they do without `--worktree`.
- `git status` can run a repository's clean filter when it must compare same-size content. No flag disables it. Coach reads file bytes itself, not through git's content filters.

`scope.worktree` is present only under `--worktree`:

| Field | Meaning |
| --- | --- |
| `revision` | The commit the working tree sits on. |
| `modified`, `added`, `deleted`, `untracked` | Counts of distinct paths listed by `git status`. |
| `config_files` | The project configuration files read from disk (always present, possibly empty). |

Text prints one line under the scope summary: `worktree: <counts> on revision <sha>`, then `, compared with merge-base <sha>` for a diff, then `; configuration read from disk: <files>`. When no counts are non-zero it says `no uncommitted changes`, or `git status reports no uncommitted changes` when a `worktree_scan_incomplete` entry is present, or `the uncommitted-change counts could not be read` when the counts are zero and a `worktree_status_check_failed` diagnostic is present.

The project policy is read from the working tree under `--worktree`, as in every scan. An uncommitted change to the selected policy file, or an untracked policy file, is the policy the scan uses, and `worktree_changes_not_analyzed` does not name it. `scope.policy` reports it with `matches_head` `false` and `cause` `differs_from_head` or `not_committed`, the text prints a WARNING, and a clean verdict is qualified ([Policy disclosure](#policy-disclosure-scopepolicy)).

### Policy disclosure (`scope.policy`)

Every scan reports which project policy it used, and not only under `--worktree`:

- JSON `scope.policy` holds `path`, `digest`, `matches_head` and `cause`. It is `null` when neither the disk nor `HEAD` selects a policy.
- `cause` is a closed enum that says why the policy used is or is not the one at `HEAD`. It is present whenever `scope.policy` is an object. The human reason that a policy on disk was unusable is text-only and is not in the JSON.
- `matches_head` is `true` when the policy selected from disk and the policy selected at `HEAD` have the same path and the same digest.
- When only one of the disk and `HEAD` selects a policy, `matches_head` is `false` and the text prints a WARNING. A difference qualifies a clean result.
- Not implemented: no `scope.policy.cause` reports a committed copy that cannot be read at `HEAD` within git's time or stderr bound, and there is no `head_unreadable` value. The intended behavior is that Coach uses the policy on disk, `matches_head` is `false`, and the WARNING says the committed copy could not be read and names the bound that tripped, not that the content differs. <!-- TODO(#168 A2b-1): add the `head_unreadable` cause and its WARNING; until then this bullet describes no shipped behavior. -->
- Text prints a policy line when a policy is selected or the two differ.

| `cause` | `matches_head` | Meaning |
| --- | --- | --- |
| `same_as_head` | `true` | The policy on disk is the committed one, or a line-ending-converted copy of it. The scan used the committed bytes. |
| `differs_from_head` | `false` | The policy on disk differs from the committed policy at the same path. The scan used the bytes on disk. |
| `not_committed` | `false` | The policy on disk has no committed copy at that path at `HEAD`. The scan used the bytes on disk. |
| `missing_on_disk` | `false` | The policy is committed at `HEAD` and absent on disk. The scan used the committed copy. <!-- TODO(#168 A2b/A3): interim until A3 (discovery fallbacks when the canonical policy is deleted); remove this row's interim status when A3 merges. --> |
| `unusable_on_disk` | `false` | The policy on disk cannot be used: over the size bound, unreadable, invalid, or not a regular file. The scan used the committed copy. <!-- TODO(#168 A2b/A3): interim until A2b-1 (an unusable policy on disk exits `2` `project_config_invalid` with no fall-through); remove this row's interim status when A2b-1 merges. --> |

A report from a version that predates `cause` has no `cause` key. A consumer reads `matches_head` alone for it.

### Unread parts of the working tree

Git can leave part of the working tree out of what a scan reads. `--worktree` reports each cause as a `worktree_scan_incomplete` diagnostic. The entry has an empty `path`, because a path would count as an unanalyzed file. A cause is one reason at one root directory, so a reason that affects several directories gets an entry per directory. Each entry carries:

| Field | Meaning |
| --- | --- |
| `reason` | One of `sparse_checkout`, `skip_worktree`, `assume_unchanged`, `submodule`, `nested_repository`, `unreadable_directory`, `git_version`, `unrecognized_git_warning`. |
| `root` | The repository-relative directory the cause affects. `.` is the repository root. Absent for a sparse checkout that hides nothing, for `unrecognized_git_warning`, and for the entry that counts unlisted roots. Text prints it with a trailing `/` (`./` for the root). A control character in a name is escaped, never dropped. |
| `hides_in_scope` | `yes`, `no` or `unknown`: whether the cause hides a path the scan would have analyzed. An entry without the field qualifies the verdict, as `yes` and `unknown` do. |
| `roots_not_listed` | Only on the further entry described below: how many roots were not listed. |

The reasons:

| `reason` | What it names | `hides_in_scope` |
| --- | --- | --- |
| `sparse_checkout` | Tracked paths outside the sparse checkout, which the scan reads from the commit, not from disk. Skip-worktree entries that the sparse checkout set are reported here and not again as `skip_worktree`. | Per root, by the paths hidden under it. |
| `skip_worktree` | Tracked paths marked skip-worktree. | Per root, by the paths hidden under it. |
| `assume_unchanged` | Tracked paths marked assume-unchanged. | Per root, by the paths hidden under it. |
| `submodule` | A submodule with working-tree changes. | `no`, unless the submodule's own path is an in-scope source file, such as a submodule at `x.ts`. |
| `nested_repository` | An untracked directory that holds its own repository. | `no`, with the same exception. |
| `git_version` | Every submodule, when git is older than 2.44. That git cannot be told not to fetch, so its status does not look into submodules. The message names the git version. There is no `submodule` entry for that root. | `no`, with the same exception. |
| `unreadable_directory` | A directory git could not open, such as an untracked directory without read permission. Git leaves it out of the status without failing. | `unknown`. |
| `unrecognized_git_warning` | A warning git printed on stderr while reading the status that Coach does not recognize. The message states how many were printed. | `unknown`. |

A hidden path is in scope when it is the selected policy file, a project configuration file the scan reads, or a Go, TypeScript or TSX file that the active `--scope` selects. A root is judged by the paths hidden under it alone. A sparse checkout, skip-worktree or assume-unchanged root is named by its directory, widened to the parent for as long as every tracked path under the parent is hidden too. A sparse or skip-worktree file is never named on its own. Coach never analyzes another repository's files, so a submodule or nested repository hides no in-scope path. If the scope filter cannot answer, the root is `unknown`.

At most 20 roots are listed per reason. The roots that qualify the verdict come first: `yes`, then `unknown`, then `no`, each alphabetical. When a reason has more roots, one further entry for that reason has no `root`, carries `roots_not_listed`, and takes the most severe `hides_in_scope` of the roots it leaves out, so the cap never turns a qualifying cause into an unqualified one. Text states `N more roots not listed` on that entry. The message of a reason is printed once in text, on its first entry. No entry relays git's stderr.

What qualifies the verdict:

- An entry with `hides_in_scope` of `yes` or `unknown`, or without the field, qualifies it. The text says `but the analysis is incomplete:` and names the cause once per reason, and `--fail-on-incomplete-coverage` exits `3`. This holds whether or not the entry's root is listed.
- An entry with `hides_in_scope` of `no` is disclosed as an entry in JSON and a line in the text. It does not qualify the verdict and does not change the exit code.
- `worktree_changes_not_analyzed`, `worktree_symlink_skipped` and `worktree_status_check_failed` also count as unread and exit `3` under `--fail-on-incomplete-coverage`. `unsupported_language` and `continuity_not_determined` qualify the verdict text and exit `0`, as in a committed scan.
- A failed `git status` is an operational error. The CLI exits `1`, writes `coach: listing working tree changes: ...` to stderr, and prints no verdict.

A text entry looks like this:

```text
path: , kind: worktree_scan_incomplete, message: an untracked nested repository is present and the worktree scan does not analyze it; it is another repository, so no in-scope source is hidden and the verdict is not qualified, reason: nested_repository, root: nest/, hides_in_scope: no
```

## Diff-mode caveats

- Without `--worktree`, dirty / uncommitted files are ignored as analysis input. A TypeScript project scan emits `worktree_report_reflects_committed_head` when relevant worktree changes exist; the report still applies to committed HEAD. Supported-language file-local dirt uses `worktree_changes_not_analyzed`. Unsupported dirt uses `worktree_not_clean`. The status is run without rename detection, so a file renamed in the index (`git mv`) is two paths wherever it is reported: its old path, deleted, and its new path, added, in `dirty_worktree.paths`, in the warning of `--check-project`, and in the count and the names of `worktree_changes_not_analyzed` ("2 staged files"), where v0.11.0 reported the new path alone. A status-check failure uses `worktree_status_check_failed`. With `--worktree` the supported-language files are read from the working tree instead; see [Uncommitted work](#uncommitted-work---worktree).
- Diff mode passes `--find-renames` (default 50% similarity) and
  `--find-copies-harder` (unmodified files are copy-source candidates) to
  `git diff --name-status`. Rename (`R`) and copy (`C`) new paths are
  analyzed. A pure git rename or copy without old-path continuity yields
  `lifecycle: unknown` plus a `continuity_not_determined` path diagnostic.
  Old-path continuity is not established. Adopters who only watch
  `introduced_signals` shall treat that path as incomplete, not green.
- A project finding that a `--base` comparison would call `introduced` and
  that touches a rename/copy new path, or would call `resolved` and touches
  its old path, is reported `lifecycle: unknown` instead and counts toward
  neither. A finding touches a path through any of its locations, or through
  the importer or importee in its identity (for Go, the moved file's package
  directory). Project identities
  are path-keyed, so a moved finding is otherwise indistinguishable from a
  fix plus a new defect. Each such path is named by a
  `project_change_lifecycle_indeterminate` diagnostic: the new path with
  `side: head`, the old path with `side: base`. A finding whose identity
  survives the move (for Go, a rename inside one package directory) stays
  `existing`.
- A diagnostic that names a path preventing a trustworthy comparison
  — `continuity_not_determined` on a rename/copy, the
  `project_root_scope_incomplete` / `base_project_root_scope_incomplete`
  model-coverage diagnostics, the CLI's
  `project_layer_bypass_coverage_incomplete` /
  `base_project_layer_bypass_coverage_incomplete` diagnostics, and
  `project_change_lifecycle_indeterminate` for a project change degraded to
  `lifecycle: unknown` — additionally carries optional `side` (`"head"` or
  `"base"`) and `revision` (that side's resolved commit SHA — for `base`, the
  merge-base, not the `--base` argument) JSON fields. A `--baseline` run
  stamps the head-side ones the same way, with `side: head`. Text
  output renders the same attribution as a trailing `, side: <head|base>,
  revision: <sha>` clause on that diagnostic's line; a diagnostic without
  those fields set renders unchanged.
- Statuses the CLI does not analyze (`T`, `U`, `X`, `B`) emit
  `unsupported_change_type`, increment `summary.files_unanalyzed` and
  `summary.files_with_diagnostics`, and qualify the text all-clear.
- Added (`A`) files are `lifecycle: introduced` and increment `introduced_signals`.
- `lifecycle` is relative to the merge-base (`introduced` / `existing` / `resolved` / `unknown`). `unknown` is residual classification, not "new in this diff." An unanalyzable merge-base carries a `base_read_failed`, `base_syntax_errors`, or `base_analysis_failed` diagnostic. A rename or copy without old-path continuity is also `unknown` and carries `continuity_not_determined`; that is not a green `introduced_signals` miss.

## Lifecycle and counters

A finding's fingerprint is computed from its rule, path, subject, evidence, and occurrence ordinal. It is not computed from its line, its severity, or `rule_version`. So a line shift or a severity change keeps the fingerprint, while a change of file path or of evidence gives the finding a new one. The `id` also includes the start position, so a line shift changes the `id` but not the fingerprint.

Every entry in `signals[]` is counted in exactly one of `introduced_signals`,
`existing_signals`, `resolved_signals`, `baseline_signals`,
`unknown_signals`. `summary.unknown_signals` counts `unknown` lifecycle
signals. `active_signals` is `len(signals)` after the include-resolved
filter, when no narrowing flag is set. That relationship holds for both
`--base` and `--baseline`:

- `--base` includes `resolved` in `signals[]` by default, so `active_signals`
  includes them. That field is not "problems present at HEAD".
- `--baseline` does not include `resolved`. Signals there are `baseline`.

A consumer who wants problems present at HEAD should sum `introduced` +
`existing` + `unknown` (and `baseline` under `--baseline`), not trust
`active_signals` alone in diff mode.

With `--min-severity` or `--top`, `signals[]` can be shorter than `active_signals`.
When `signals_withheld` is present, `active_signals == len(signals) +
below_min_severity + beyond_top`, reading an absent key as `0`. In a
`schema_version: "2"` report, `project_changes[]` is shortened to the changes
whose mirrored signal survived, so `len(project_changes)` can also be below
`project_summary.active_changes`, which keeps the full count. Each project
change is also a signal, so the withheld ones are already counted in
`below_min_severity` and `beyond_top`. See
[Narrowing the rendered report](#narrowing-the-rendered-report).

## JSON report

`--format json` writes one object to stdout.

Always present on a successful scan:

- `schema_version`: `"1"` when no project policy is selected; `"2"` when discovery or an explicit `--project-config` selects a valid policy.
- `scope`, `summary`, `signals`, `diagnostics`, `coverage`.

Present only when `--min-severity` or `--top` was supplied: `signals_withheld`
(see [Narrowing the rendered report](#narrowing-the-rendered-report)).

`scope.worktree` is present only under `--worktree` (see
[Uncommitted work](#uncommitted-work---worktree)).

`schema_version: "2"` also includes `project_changes`, `project_facts`,
`project_summary`, and `project_coverage`. TypeScript project reports
additionally include `project_provenance` and `project_scope`, and
`project_next_actions` when a complete scan finds no configured covered
match. Go schema-2 reports omit those keys.

Go and TypeScript schema-2 reports include `layer_membership` when the policy
declares at least one layer and the count could be computed: `head`, and `base`
when a base revision was analyzed, each with `revision`, `analyzed_files`, and
`files_outside_layers`. It counts distinct project files under the configured
`roots` (the files the project model analyzed) that no layer prefix contains,
matched the way the layer rules match (a Go package directory, a TypeScript file
path). `analyzed_files` here is that same population, not the summary's
`files_analyzed`. Code outside `roots` is not in this count, but the file-local
`files_analyzed` may still include it. For TypeScript, a tsconfig `include`
that reaches outside its root (for example `../shared/**`) adds those files to
the project model, so they count in `analyzed_files`. A missing
`layer_membership` means "not computed": the policy declares no layers, the
project model is incomplete (any incomplete model, such as one cut short by
the input file or byte budget, or, for TypeScript, a root whose candidate files
were not all incorporated), a configured root has no analyzed file at or under
it (Go: no module sits at or under the root, for example `roots: ["pkg"]` with
`go.mod` at the repository root; TypeScript: the root has no tsconfig and no
other root analyzes files under it, while a nested root such as `pkg` under a
root `.` that has the tsconfig is counted), or a TypeScript root reports an
analyzed-file count that its path list does not match. In each case a count
would leave files out, so it is not reported. In a diff, a head side that was
not computed omits the whole block, including a base count that was computed.
A present `files_outside_layers` of `0` is a real zero. It is
information, not a finding, and it changes no lifecycle, fingerprint, or exit
code.

`summary` counts: `files_analyzed`, `files_with_diagnostics`, `active_signals`,
`introduced_signals`, `existing_signals`, `resolved_signals`, `baseline_signals`,
`unknown_signals`.

Each signal includes `id`, `fingerprint`, `rule_id`, `rule_version`, `kind`,
`category`, `severity`, `confidence`, `lifecycle`, `changed`, `path`,
`location`, `why_it_matters`, `recommendation`, `provenance`. JSON
`location.start_row` is 0-based. Text `line` is `start_row + 1`.

JSON and text both carry `rule_id` and `severity`. A text finding block prints
`rule_id:` and `severity:` lines before `path:`, and a project finding block
prints `severity:` beside its `rule_id:`.

### Severity

`severity` is one of `high`, `medium`, `advisory`, or `low`. There is no
`critical`. Most rules emit a fixed value. Three rules measure a metric against
a threshold and emit `medium` from the threshold up. Two of them escalate to
`high` when the measured value is at least twice the threshold. Escalation is
part of the deterministic core, so coach-api baseline jobs, which share it
(system-overview §3C), also carry the escalated severity in their stored
findings. Baseline judgment prioritization covers only
`state.hidden_input_mutation`, a rule that never escalates, so escalation does
not change it:

| `rule_id` | Metric (evidence) | Threshold | `high` from |
| --- | --- | --- | --- |
| `complexity.cognitive_complexity` | `cognitive_complexity=N` | 15 | 30 |
| `complexity.max_nesting_depth` | `max_nesting_depth=N` | 4 | 8 |
| `complexity.branch_density` | `branch_sum=N` | 12 | never; always `medium` |

`complexity.branch_density` does not escalate because `branch_sum` is a
whole-file additive total: it grows with file length and a split clears it, so
twice the threshold says nothing about how hard the code is to follow. It is
still ranked by magnitude among `complexity.branch_density` findings (see
Report order).

Only `complexity.cognitive_complexity` and `complexity.max_nesting_depth` can
emit `high`, so `--min-severity high` shows only those two rules. Every other
rule emits a fixed severity, including `security.toctou_check_then_act` and
`state.hidden_input_mutation`, which are always `medium`, so a `high` floor
withholds them.

`max_nesting_depth` is the file-wide maximum nesting depth, not a per-function
value: one deeply nested function sets it for the whole file.

`rule_version` tracks what a rule detects: its trigger and its evidence.
Severity derivation is documented here and changing it does not bump
`rule_version`. Fingerprints and `id`s are computed from the rule, path,
subject, evidence, and occurrence ordinal (and, for `id`, start position), not
from `severity` or `rule_version`, so a signal keeps its identity and lifecycle
when its severity changes.

Entries in `project_changes[]` differ: their fingerprint and `id` include
`rule_version`, `backend_version`, and `config_digest`. Changing any of them,
including any byte of the policy file, gives every project change a new
identity.

Layer rules (`architecture.layer_violation`, `architecture.layer_bypass`) emit
`advisory`; the density rules `structure.constructor_density` and
`structure.pointer_return_density` emit `low`.

### Report order

`signals[]` is sorted by these keys, each breaking ties left by the one before:

1. Lifecycle group: introduced and changed, existing and changed, introduced
   and unchanged, existing and unchanged, resolved, then everything else
   (`baseline`, `unknown`).
2. Severity: `high`, `medium`, `advisory`, `low`.
3. Confidence: `high`, `medium`, `low`.
4. Path, start row, start column, `rule_id`, then `id`.

Confidence orders findings within a severity.
`complexity.cognitive_complexity` carries `high` confidence and
`complexity.branch_density` and `complexity.max_nesting_depth` carry `medium`,
so cognitive-complexity findings precede `medium`-confidence findings,
including every `complexity.branch_density` and
`complexity.max_nesting_depth` finding, of the same severity and lifecycle
group whatever their path.
`structure.react_component_orchestration_density` also carries `high`
confidence, so `medium` cognitive-complexity findings and React orchestration
findings share a tier and interleave by path; the magnitude ranking below then
reorders the cognitive-complexity findings only among the positions they hold.

Then, inside each tier of equal lifecycle group, severity, and confidence, the
findings of each metric rule (`complexity.cognitive_complexity`,
`complexity.branch_density`, `complexity.max_nesting_depth`) are reordered by
magnitude, the metric divided by the rule's threshold, highest first (ties keep
path order). They are reordered across the positions that rule already occupies
in the tier. A `cognitive_complexity=28` finding precedes a
`cognitive_complexity=16` one in the same tier regardless of path.

Ranking is within a rule only. A signal with no numeric metric (for example
`security.toctou_check_then_act` or `state.hidden_input_mutation`) keeps its
path position, and findings of different rules are never compared by
magnitude, so a whole-file `branch_sum` cannot outrank another rule's finding.
Magnitude is a ratio to a rule's own threshold, so it means nothing across
rules.

Severity outranks magnitude, so a `medium` signal never precedes a `high` one
because its metric is larger.

The lifecycle group order has one consequence for `--base`: `resolved` findings
rank ahead of `unknown` ones (residual classification: an extra occurrence of
an existing finding, an unanalyzable merge-base, a rename or copy without
old-path continuity, or, in project mode, a project finding whose lifecycle is
indeterminate (`project_lifecycle_indeterminate`)). An `unknown` finding still
present at HEAD can therefore rank behind resolved findings, and a `--top` cap
can withhold it; the withheld count and the see-all clause still state that.

### Narrowing the rendered report

`--min-severity` and `--top` are opt-in and apply to the report after analysis.
Without either flag the output is unchanged. They narrow `signals[]` (and, in a
`schema_version: "2"` report, `project_changes[]`, which mirrors the project
findings in `signals[]`) and nothing else:

- `summary`, `coverage`, `diagnostics`, and the project coverage and summary
  blocks keep describing the full analysis. `active_signals` is not reduced, so
  it can exceed the number of signals shown.
- The floor applies first, then the cap takes the first `N` of what remains in
  report order: lifecycle group first (in `--base` mode, introduced and changed
  findings come first), then severity, confidence, and path with within-rule
  magnitude. The two flags compose.
- In `schema_version: "2"` text, the cap still keeps the first `N` in report
  order, but the text prints every file-local finding first and then the project
  findings. File-local findings print in report order. Project findings then
  print in `project_changes[]` order (by `semantic_key`, then `rule_id`), not
  rank order, so a higher-ranked project finding can print after a lower-ranked
  one.
- `advisory` ranks below `medium`, so `--min-severity medium` withholds
  `advisory` `architecture.layer_violation` and `architecture.layer_bypass`
  findings. That ordering is deliberate
  ([#259](https://github.com/lousy-agents/coach/issues/259)).
- The number withheld is never silent. JSON adds `signals_withheld` only when a
  flag was supplied. Each flag contributes its own pair of keys:
  `min_severity` with `below_min_severity`, and `top` with `beyond_top`. For
  example, `{"min_severity":"high","below_min_severity":2,"top":2,"beyond_top":2}`.
  A flag that withheld nothing still reports its count, as `0`.
  `signals_withheld` describes presentation narrowing only. A filter that
  changes what is analyzed, such as suppression or vendor exclusion, changes
  `summary` and `coverage` and does not use it.
- Text prints a `withheld:` line in the summary. For `--min-severity` it reads
  `withheld: 2 signals below --min-severity high; summary counts describe the full analysis; see all: coach codesignal --baseline`.
  For `--top` it reads
  `withheld: 76 signals beyond --top 3; summary counts describe the full analysis; see all: coach codesignal --baseline`.
  With both flags it reports the total and each count:
  `withheld: 4 signals (2 below --min-severity high, 2 beyond --top 2); summary counts describe the full analysis; see all: coach codesignal --baseline`.
  When nothing was withheld the line has no see-all clause:
  `withheld: 0 signals beyond --top 6; summary counts describe the full analysis`.
  The see-all command is printed as `coach codesignal ...` with the user's own
  arguments minus the narrowing flags, quoted for a POSIX shell. It does not
  echo a different binary path such as `./coach`, or the launcher you used. A
  user without `coach` on `PATH` runs the printed command through the same
  launcher, for example
  `mise exec github:lousy-agents/coach -- coach codesignal --baseline`. It is omitted when any
  argument is not valid UTF-8 or contains a character in Unicode category Cc
  (C0, DEL, or C1), Cf (format characters: bidirectional controls, zero-width
  space, word joiner, byte order mark, soft hyphen, tag characters), Zl, or Zp
  (line and paragraph separators). Quoting stops a shell from reinterpreting
  such a character but not a terminal or a line reader: control characters can
  drive the terminal, bidirectional controls reorder what it shows, other
  format characters alter what a reader sees, and Zl and Zp break the printed
  command across lines. The check is by category, not by visibility: some
  invisible characters outside those categories, such as U+034F or U+3164,
  still print.
  When the command is omitted and something was withheld, the line ends
  `; see all: re-run without --top`, `... without --min-severity`, or
  `... without --min-severity and --top` instead, naming only the flags given.
  It is printed in text only; JSON carries the counts, not the command. When the
  `--min-severity` floor withheld at least one signal and none remain, the text
  prints `No active CodeSignal findings at or above --min-severity <level>.`
  instead of the all-clear. A report with no findings at all prints the plain
  all-clear, whatever the floor.
- Neither flag changes the exit status. Exit codes depend on analysis outcome
  and `--fail-on-incomplete-coverage`, never on which signals a report holds or
  hides.
- Neither flag is accepted by `--check-project`, `--suggest-project-config`, or
  `--prepare-compiler`.

## Rule IDs

File-local (no project policy selected):

| `rule_id` | Source |
| --- | --- |
| `state.hidden_input_mutation` | finding `mutates_input` |
| `coupling.tight_constructor_init` | finding `tight_coupling` |
| `structure.constructor_density` | finding `constructor_func` (density-gated) |
| `structure.pointer_return_density` | finding `pointer_return` (density-gated) |
| `security.toctou_check_then_act` | finding `toctou_check_then_act` |
| `complexity.max_nesting_depth` | metrics |
| `complexity.branch_density` | metrics |
| `coupling.deep_relative_import` | TypeScript/TSX imports |
| `complexity.cognitive_complexity` | per-function score (threshold in codesignal) |
| `structure.react_component_orchestration_density` | TS/TSX component facts |

With a selected project policy (`--project-config` or discovery): `architecture.layer_violation` and, when
`required_layer` is set, `architecture.layer_bypass` (Go and TypeScript).

For TypeScript, `architecture.layer_violation` evaluates type-only imports
(`import type`, `export type ... from`, and imports or re-exports whose every
named specifier is inline `type`) the same way it evaluates value imports. A
type-only import is a compile-time dependency, and a layer rule forbids it just
as it forbids a runtime one. A dynamic `import("...")` and a `require("...")`
whose specifier is a string literal, or a template literal without
substitutions, are evaluated the same way. A specifier that matches a
`compilerOptions.paths` pattern in the project's `tsconfig.json` (including one
inherited through `extends`) is followed to the aliased file for `require()` and
`import()` as well as for static imports.

Edges that layer rules cannot place on a layer are reported, not skipped, when
the importing file sits inside a declared layer:

- An import that did not resolve within the snapshot gets one
  `project_import_unresolved` diagnostic per edge, naming the file, the
  specifier, and the line; the revision and side are on the diagnostic. A broken
  alias could hide a forbidden edge, so it makes the no-findings verdict
  incomplete. A diagnostic recorded on the base side of a diff has the kind
  `base_project_import_unresolved`.
- Imports of external packages, and imports whose specifier ends in a
  non-source extension such as `.json`, `.css`, or `.png`, are counted in one
  `project_external_imports_not_evaluated` diagnostic per revision, for example
  "3 import edges to external packages were not evaluated by layer rules". A
  package or a stylesheet cannot match a path-prefix layer, so the count only
  informs and does not qualify the verdict. The analyzer is not given non-source
  files, so it cannot tell a present `./data.json` from a missing one; both are
  counted here.

An alias that `compilerOptions.paths` does not declare, such as a bundler alias
or a `package.json` `imports` entry like `#utils/x`, is not followed. Its
specifier looks like a package, so it is counted as external rather than
reported as unresolved, and an edge through it that crosses a forbidden
boundary is not found. A package whose name a `paths` pattern other than a bare
`*` also matches, such as a published `@org/lib` under `"@org/*"`, is reported
as unresolved when no aliased file exists, because the analyzer has no
`node_modules` to fall back to. An import of a plain JavaScript file (`.js`,
`.cjs`, or `.mjs`) with no `.ts` or `.tsx` counterpart is reported as
unresolved, because the analyzer receives only `.ts` and `.tsx` sources.

A dynamic load whose specifier is not a literal, such as `import(path)`,
`` import(`./adapters/${name}`) ``, or `require(name)`, names no target, so layer
rules cannot place it. When the importing file sits inside a declared layer it
gets one `project_import_non_literal` diagnostic per site, naming the file, the
call, the expression, and the line. The expression is shown on one line, quoted
and escaped, and cut to 120 characters with a trailing `…` when longer. The
no-findings verdict says it is incomplete: "1 dynamic load with a non-literal
specifier was not evaluated, so layer rules cannot tell whether it crosses a
layer". Only a call to a bare
`import(...)` or `require(...)` is recognized, including `(require)(name)` and
`require!(name)`, each of which gets its own diagnostic. An alias such as
`const r = require; r(name)`, a callee wrapped in a cast or comma expression
such as `(require as any)(name)`, `(0, require)(name)`, or
`(require satisfies F)(name)`, `require.resolve(name)`, a method
named `require` on another object, and loaders such as `createRequire(...)(name)`
or a bundler's glob import are not detected. A diagnostic recorded on the base
side of a diff has the kind `base_project_import_non_literal`.

None of these diagnostics changes `--fail-on-incomplete-coverage`: an
unresolved import and a non-literal load qualify the verdict text, but the flag
keeps its exit status.

Set `"exclude_type_only_edges": true` in the policy file to leave type-only
edges out. It is optional and defaults to `false`, and it has no effect on Go
projects, which have no type-only import. A pair of files that also has a value
import is still reported, and only its value-level edges count.

When every edge in a reported pair is type-only, the finding's
`machine_evidence` carries `removed_if_set: exclude_type_only_edges`, meaning
the finding is removed if `exclude_type_only_edges` is set. The key marks
findings the option would remove; it is not documented to mean anything when
absent.

The value must be `true` or `false`. `null` is read as `false`, as it is for
the other optional policy keys, and any other value is a configuration error
(exit `2`). Coach versions before this option reject a policy that contains the
key (exit `2`).

The TypeScript `architecture.layer_violation` `rule_version` is `"2"`. The Go
rule stays at `"1"`.

Unrecognized finding kinds are ignored. An empty `signals` array is not a
clean bill of health.

`architecture.layer_violation` checks an import against `forbidden_imports` only
when both the importing and the imported code (Go: the package directory) sit
inside a declared layer. Code outside every declared layer is still analyzed by
the file-local rules, but is not evaluated by that rule, so an empty set of
layer-violation findings says nothing about imports to or from it. The report
states how many project files under the configured roots sit outside every
declared layer, as information rather than a finding. The text line describes
the head revision; when the base revision of a diff also has files outside
every layer, the text prints one line per revision, each starting `Head
revision <sha>:` or `Base revision <sha>:`. For Go the line adds `(a Go file is
in a layer when its package directory is)`, because a layer prefix that names a
file (`pkg/db/db.go`) contains no package directory, so that file counts as
outside. When the policy declares `forbidden_imports` and either revision's
count is above zero, the text verdict carries the same scope: the all-clear
reads `...; forbidden_imports was evaluated only between files counted inside
declared layers`, the complete-scan verdict reads the same after `no configured
covered match`, and an incomplete verdict keeps its causes and adds
`Separately, forbidden_imports was evaluated only between files counted inside
declared layers.` A policy with no `forbidden_imports` gets the count alone,
with no `forbidden_imports` wording. `architecture.layer_bypass` is defined against
`required_layer` alone, and file-local rules run on every analyzed file
regardless of layers.

## Signed archive without mise

Release workflow signs `checksums.txt` with cosign (`sign-blob`, keyless
GitHub Actions OIDC) and attests provenance. This procedure follows that
workflow; it is not a CI-proven customer install.

From [Releases](https://github.com/lousy-agents/coach/releases) download, for
your platform:

- `coach_darwin_arm64.tar.gz` / `coach_darwin_x86_64.tar.gz` / `coach_linux_x86_64.tar.gz` / `coach_windows_x86_64.zip`
- `checksums.txt`
- `checksums.txt.bundle`

```sh
cosign verify-blob \
  --bundle checksums.txt.bundle \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/lousy-agents/coach/' \
  checksums.txt

shasum -a 256 -c checksums.txt --ignore-missing   # GNU: sha256sum -c checksums.txt --ignore-missing
tar -xzf coach_darwin_arm64.tar.gz                # windows: unzip coach_windows_x86_64.zip
```

Put the `coach` binary on `PATH`. Archives exist for those four platforms
only; there is no `linux_arm64` build yet.

## `pkg/semantics` quickstart

Deterministic structural analysis of raw Go / TypeScript / TSX bytes. No Git,
no GitHub, no model.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func main() {
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		log.Fatal(err)
	}
	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "greeter.go",
		Language: semantics.LanguageGo,
		Content:  []byte("package greet\n\nfunc NewGreeter() *Greeter { return &Greeter{} }\n\ntype Greeter struct{}\n"),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.ParseStatus)
	for _, f := range result.Findings {
		fmt.Println(f.Kind, f.Name)
	}
}
```

```sh
go get github.com/lousy-agents/coach/pkg/semantics
```

Syntax errors return a partial `*Result` (`ParseStatus == "syntax_errors"`)
and a non-nil error satisfying `errors.Is(err, semantics.ErrSyntax)`. Other
sentinels: `ErrEmptyContent`, `ErrUnsupportedLanguage`, `ErrFileTooLarge`,
`ErrBinaryContent`, `ErrParseFailure`. JSON field names on `Result` are
frozen `snake_case`.

`pkg/semantics` shall not import `pkg/githubingest`.

## JavaScript / TypeScript quickstart

`@lousy-agents/coach-semantics` is **not published to npm**. Clone this
repository. Node `^24 || ^26`. `npm install` / `prepare` builds the Go
stdio backend, so a Go toolchain is required.

```sh
cd js/semantics
npm install
```

```ts
import { createAnalyzer, SemanticsSyntaxError } from "@lousy-agents/coach-semantics";

const analyzer = await createAnalyzer();
try {
  const result = await analyzer.analyzeBytes({
    path: "widget.go",
    language: "go",
    content: "package p\n",
  });
  console.log(result.parse_status, result.metrics);
} catch (err) {
  if (err instanceof SemanticsSyntaxError) {
    console.log(err.partialResult.syntax_errors);
  } else {
    throw err;
  }
} finally {
  analyzer.dispose();
}
```

Go and JS outputs for shared fixtures are kept byte-identical
(`js/semantics/test/parity.test.ts`).

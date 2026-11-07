# `coach codesignal` CLI contract

Machine-oriented contract for the local `coach codesignal` CLI and the
analyzer libraries. Customer onboarding lives in [README.md](../README.md).

This documents shipped behavior. It is not a gate and not a 1.0 promise.

## Commands

```text
coach <command> [flags]
coach --help
coach --version
```

The only subcommand is `codesignal`.

```text
coach codesignal (--base <ref> | --baseline) [--format text|json] [--scope production|all] [--min-severity high|medium|advisory|low] [--top N] [--build-target <package>] [--project-config <path>] [--project-language go|typescript] [--no-interactive] [--fail-on-incomplete-coverage]
coach codesignal --baseline --suggest-project-config [--output <path>]
coach codesignal --baseline --suggest-project-config --project-language typescript [--output <path>] [--no-interactive]
coach codesignal --baseline --check-project --project-language typescript [--project-config <path>] [--format text|json]
coach codesignal --baseline --prepare-compiler --project-language typescript [--project-config <path>] [--no-interactive]
```

`--base` and `--baseline` are mutually exclusive. `--check-project`,
`--suggest-project-config`, and `--prepare-compiler` are not scans (except
that suggest/prepare still require `--baseline`).

| Flag | Default | Notes |
| --- | --- | --- |
| `--base <ref>` | | Diff HEAD against any Git-resolvable ref. |
| `--baseline` | | Scan every tracked Go/TS/TSX file at HEAD. |
| `--format` | `text` | `text` or `json`. |
| `--scope` | `production` | `production` or `all`. |
| `--min-severity <level>` | none | Render only signals at or above `high`, `medium`, `advisory`, or `low` (ranked `high` > `medium` > `advisory` > `low`). `advisory` ranks below `medium`, so `--min-severity medium` also withholds `advisory` `architecture.layer_violation` and `architecture.layer_bypass` findings. Only `complexity.cognitive_complexity` and `complexity.max_nesting_depth` can be `high`, so `--min-severity high` shows only those two rules ([Severity](#severity)). No floor by default. Any other value is a usage error: exit `2`. See [Narrowing the rendered report](#narrowing-the-rendered-report). |
| `--top <N>` | none | Render only the first `N` signals in report order ([Report order](#report-order): lifecycle group first, then severity, confidence, and path with within-rule magnitude), after any `--min-severity` floor. `advisory` findings rank below every `medium` one in the same lifecycle group, so a cap can push out declared `architecture.layer_violation` and `architecture.layer_bypass` findings, just as `--min-severity medium` hides them. No cap by default. `N` must be a positive integer; anything else is a usage error: exit `2`. |
| `--build-target <pattern>` | empty | Go package pattern for production reachability. Silent no-op under `--scope all`. |
| `--project-config <path>` | `.lousy-agents/coach.json`, else a schema-valid root `project.json` | Explicit path wins and no other file is consulted. Otherwise `.lousy-agents/coach.json` at the git top-level, read at the **analyzed revision**. Otherwise a schema-valid root `project.json`, always with `project_config_deprecated`. An invalid canonical file is an error and does not fall back; on a terminal, a TypeScript scan offers guided policy authoring for it instead of the scan (exit `2`, candidate on standard output, no file written), and offers it after the report when the file is valid but declares no layers and the scan exits `0`. A selected policy enables `schema_version: "2"`. An explicit path is taken from the repository root, whatever directory the scan starts in; an absolute path, one that climbs out with `..`, and one inside `.git` (where git tracks nothing) are refused with exit `2` and a message, and no authoring is offered for them. See [`.lousy-agents` config](./conventions/lousy-agents-config.md). |
| `--project-language` | `go` | `go` or `typescript`. The language selects the project backend when a policy is selected (explicit path or the discovery order above). With no policy selected, the flag does not start project analysis. |
| `--suggest-project-config` | | Without `--output`, writes `.lousy-agents/coach.json`. Go also prints the candidate on stdout. TypeScript writes the file after confirmation and does not print the candidate on stdout. `--output` still wins. Never auto-applied; never invents layers. |
| `--output <path>` | `.lousy-agents/coach.json` | Create-only path for a suggest candidate. When set, this path wins over `.lousy-agents/coach.json`. |
| `--check-project` | | Read-only TypeScript readiness. Exits `0` on gaps. Not a scan. A policy gap names one next action: `author_policy` for a policy that was never committed and that the index holds nothing for, or for one that is committed as a regular file whose content is not a valid policy (a new document replaces it), or `restore_policy` for an explicit `--project-config` that is committed at the analyzed revision and gone from disk, unless a restore would overwrite or recreate something or git cannot run it (a staged edit, an unmerged entry, a symbolic link, a sparse checkout, a skip-worktree mark, or a question git could not answer) or the path lies under a file in the way (the message names the file, and the policy is restored after it is moved). Neither is executable (`executable: false`), and a gap that a new file or a restore cannot fix names neither: that includes a policy the index holds a staged copy of (never committed), an explicit path that is a regular file on disk but not committed, and a path the analyzed revision commits as a symbolic link, a directory or a submodule (`committed_as_link`, `directory`, `in_submodule`), whether or not a file is on disk. An index entry that holds no content (`git add --intent-to-add`, or an empty file staged) counts as nothing: over a committed policy the restore still brings back the committed copy. |
| `--prepare-compiler` | | Interactive consented **mise-only** TypeScript compiler setup. Requires a TTY and an attended session; refuses otherwise. Also refuses, naming the gap, for a runtime-boundary gap (`node_missing`/`node_unsupported`) — the same restriction the scan path's own compiler-setup offer applies, so the two cannot disagree. It is narrower than the scan's offer, which also runs the project package manager: a repository whose only executable choice is `project_package` is never handed this command as remediation, because it would exit `0` reporting nothing to set up. |
| `--no-interactive` | `false` | Treat the invocation as unattended even with a controlling terminal attached. On a scan, the interactive compiler-setup and guided-policy-authoring offers are skipped, falling through to the same message-only remediation a piped invocation gets; the TypeScript form of `--suggest-project-config`, and `--prepare-compiler`, refuse with exit `2` instead of prompting (the Go form of `--suggest-project-config` never prompts, and rejects the flag as a usage error). When a legacy root `project.json` would otherwise prompt for a copy, the unattended invocation skips that prompt and emits `project_config_migration_skipped` with `mkdir -p .lousy-agents && git mv project.json .lousy-agents/coach.json`. Also triggered by a non-empty `CI` environment variable, so a pty-allocating runner cannot hang on an unanswered prompt. |
| `--fail-on-incomplete-coverage` | `false` | Exit `3` when required project coverage (model or bypass phase, on any analyzed side) is incomplete — including when the project backend is unavailable. A Go report records no provenance, so for Go this reads `project_coverage.complete`, which folds the model and any `required_layer` bypass search, and, for the base side of a diff, the coverage diagnostics. Go cases that count as incomplete include a repository with more than 5,000 tracked files of any type (Go discovery counts every file), a `required_layer` prefix that matches no function, and no `go` on `PATH` when `required_layer` is set. The report is always written to stdout first. Valid only on scan modes (`--base` / `--baseline`). |

Requires `git` on `PATH`. Analyzes **committed Git objects**, not the dirty
worktree. TypeScript project mode also inspects worktree `package.json` /
`node_modules` / `mise.toml` for compiler and Node readiness.

`coach codesignal` takes no positional operands. An unexpected positional
argument is a usage error: exit 2, usage on stderr, and nothing on stdout.

## Exit codes

| Code | When |
| --- | --- |
| `0` | Completed analysis, readiness report, successful suggest/prepare, or `--help` / `--version`. Signals, and narrowing the report with `--min-severity` or `--top`, do not change this. |
| `1` | Operational: not a git repo, missing `git`, unresolvable `--base`, empty repo, I/O. Also `--check-project` when git cannot answer a question the check needs for an explicit `--project-config` (whether it is committed at the analyzed revision, what the index holds for it, or whether the revision holds it in another case): stdout is empty and stderr names the question and what git said, because a verdict built on an unanswered question could name a restore or a new policy over work it never read. Also, in a partial clone: with a git older than 2.44, which cannot be told not to fetch, a revision the scan reads that lacks objects is refused before anything else runs (a clone that lacks nothing the scan reads is read as usual), and a scan or `--check-project` that needs an object the clone lacks fails, because Coach never fetches (see [What repository configuration can run](#what-repository-configuration-can-run)). |
| `2` | Usage (including an invalid `--min-severity` or `--top` value), invalid `--project-config` (empty stdout, stderr names the path/revision), a `--project-config` that does not exist on disk (empty stdout, stderr names the path and `project_config_missing`, no fallback to another policy; a committed policy gets the one `git restore` command that brings it back unless a restore would overwrite or recreate something or git cannot run it (a staged edit, an unmerged entry, a symbolic link, a sparse checkout, a skip-worktree mark, or a question git could not answer); a policy that was never committed but that the index holds gets the command that restores the staged copy from the index when it is a staged regular file, and the reason with no command when it is unmerged, is not a regular file, or is marked skip-worktree or left out by a sparse checkout; a path under a regular file gets the cause and the file in the way with no restore command; and none of these is offered authoring) or that is unusable on disk: a symbolic link, a directory or other file that is not a regular file, a path under a symbolic link, a file that cannot be read, or a parent that cannot be searched (`project_config_invalid`, naming the reason), unsupported `--project-language`, or a TypeScript **scan** that cannot resolve a supported compiler or host Node (empty stdout except the policy-candidate document on the guided-authoring path; one or more stderr lines). |
| `3` | Required project coverage (model or bypass, on any analyzed side) is incomplete **and** `--fail-on-incomplete-coverage` was supplied — the report is still written to stdout. Also: `--prepare-compiler` operational failure resolving the baseline revision or computing readiness (a deliberate exception to the scan contract's `1`, kept because the flag follows suggestion mode's table). |

`--check-project` exits `0` so you inspect the payload; do not treat that
status as a gate.

With an explicit `--project-config`, `--check-project` reads the path from the
working tree. When the working tree decides the policy gap (the file is gone,
something unusable is there, a parent is a file or a link, or the file is there and
not committed), that path is left
out of `dirty_worktree.paths`, because the check did read it, and
`relevant_changes` is `false` unless another path counts. A staged edit to a
committed policy, an unmerged entry for it, or a staged entry for a policy that
was never committed, is the exception: nothing read it, so the path stays in
`dirty_worktree`, and Coach offers no restore over an edit.

A policy gap that names neither `author_policy` nor `restore_policy` says why in a
`cause` on the gap, and the text prints the same word after the code
(`policy_missing cause=not_committed`). The field is absent from every other gap,
and absence has one meaning: the gap is not a policy gap, or a restore or a new
policy fixes it, which `next_actions` names (`restore_policy` or `author_policy`).
A policy gap with no way out always has a `cause`; `undetermined` is the word for a
state Coach has no other word for, so a consumer never has to read a missing `cause`
as "unknown". The values are stable: `not_committed` (a file on disk that the analyzed revision
does not hold; commit it), `in_submodule` (the policy lies inside a submodule checkout, present or
missing, or the analyzed revision commits the path itself as a submodule: the superproject
records the submodule as one commit, so the file cannot be committed here, and the way out is a
policy this repository commits), `parent_not_directory`, `staged_edit`,
`staged_copy` (a staged copy of a policy that was never committed),
`staged_not_a_file`, `merge_conflict`, `skip_worktree`, `sparse_checkout`,
`committed_as_link` (the analyzed revision commits the policy as a symbolic link,
whether or not a file is on disk), `directory` (the index or the analyzed revision holds files
under the name),
`index_locked`, `git_unreadable` (only a read that trips the time or stderr bound reaches a
document; any other unanswered git question is exit `1`) and `undetermined`. A path on disk
that cannot be a policy has one word for each fix: `not_a_regular_file` (a directory, a pipe
or another entry on disk, or an entry of another kind that the analyzed revision commits; name
a regular file), `symbolic_link` (the policy is a link or is reached
through one; name the file it points to), `unreadable_on_disk` (the process cannot stat or
read it; fix the permissions) and `name_case_differs` (rename it to the revision's spelling).

The analyzed revision decides what an explicit `--project-config` names. When it holds exactly the path as typed,
that is the policy that is read, whatever spelling a file system that folds case shows for the file on disk: with
`cfg/p.json` typed and committed, a disk that shows `CFG/p.json` or `cfg/P.json` (a rename in case only, which `git
status` does not show there) is no reason to refuse, and v0.11.0 read it. The same holds for the policy a printed
move left under a differently spelled directory, so a discovered scan and an explicit one of the same committed file
agree. A name that differs only by case from what is on disk is refused only when the revision does not hold the
spelling that was typed. The refusal has exit `2` and `project_config_invalid`; `--check-project` reports it as
`policy_invalid` with the cause `name_case_differs`. A file system that folds case resolves `CFG/p.json` for
`cfg/p.json`, but the analyzed revision tells the two spellings apart, so the scan would read a committed policy the
working tree does not hold under that name. The message names both spellings and says which of them the revision
holds, and what to do follows from that:

- The revision holds the spelling on disk and the user typed another: pass the spelling
  on disk, as in `pass --project-config cfg/p.json`. Nothing is renamed, because the
  revision already holds that spelling.
- A file on disk that is not committed under that name, where the user typed its spelling
  on disk and the revision holds the same path in another case, on a repository that
  ignores case (the file was renamed in case only and the rename was never recorded): the
  file can be renamed back, or the rename recorded with `git -C "$(git rev-parse --show-toplevel)" mv -- '<revision
  spelling>' '<spelling on disk>'`, committed, and the spelling on disk passed, with the cause `name_case_differs`, not
  `not_committed`. The printed move commands run at the repository root, which they find themselves, so they work when
  pasted from a subdirectory. `git add` cannot do that: on a repository that ignores case it records nothing for a
  rename that changes only the case of a name, and the commit says there is nothing to commit. `git mv` refuses to
  rename a directory in case only, so a directory is renamed back and no `git mv` is offered.
- The revision holds neither spelling: the message says so and gives no advice,
  because no spelling is the right one to move to. It gives none either when git cannot
  answer.

A policy that Coach discovers, with no `--project-config`, is read only at `.lousy-agents/coach.json`. When nothing is committed there and the analyzed revision holds the same path in another case (`.Lousy-Agents/Coach.json`, say), the scan is refused with `project_config_invalid`, and `--check-project` reports a policy gap with the cause `name_case_differs` and no `author_policy`, because a new policy cannot be written over the one that is committed. Before, the scan ran without the policy and said nothing, which dropped its layer checks. The message names the spelling that is committed and the commands that move it, `top="$(git rev-parse --show-toplevel)" && mkdir -p "$top"/'.lousy-agents' && git -C "$top" mv -- '.Lousy-Agents/Coach.json' '.lousy-agents/coach.json'`, to be committed (the `mkdir` is for a file system that tells cases apart, where the directory does not exist under the new spelling; both run at the repository root, so they work from a subdirectory too). It holds on any file system, since the committed names are what differ. With several spellings committed (`.lousy-agents/Coach.json` and `.LOUSY-AGENTS/coach.json`, say) the message lists every one of them, up to 16, and prints the commands that keep one: it moves the one that needs the fewest components renamed (a file can be renamed in the index with `git mv`, a directory cannot be renamed in case only, so a spelling whose directory is already right is kept first) to the path Coach reads and takes the others out of the index with `git -C "$top" --literal-pathspecs rm --cached -q -- ...`, the index only because on a file system that folds case they name the file just moved. Past 16 spellings, or with a name that cannot be shown to a terminal, it prints no command and asks for one spelling at that path.

The check covers every component of the path, and one run reports the first component
that differs, with the advice for that component alone. When two differ (`CFG/P.json` for
`cfg/p.json`), correcting the first leads to the message for the next, one component per run:
the right spelling of a later component comes from the disk when it exists there and from the
revision when only the revision holds it, and Coach does not guess between them. It also
applies when the file is missing, and when a deeper directory is missing too: a missing policy
under a parent spelled in another case (`Cfg/new/zz.json` where only `cfg` exists) is refused
for that parent and is not offered a restore or a new policy, because either would write into
the other spelling.

A policy that is missing from disk has no spelling on disk to compare, so the revision's
spelling is used instead. When the revision holds the path in another case than the one
typed, for example `cfg/P.json` typed while `cfg/p.json` is committed and deleted from
disk, the scan is refused with `project_config_invalid`, the message gives the revision's
spelling to pass, and no new policy is offered, because writing one there would replace
the committed policy under its own name. Passing the revision's spelling gets the usual
restore offer. This holds for any component of the path and on any file system, since it
asks the revision and not the disk. When the revision holds several spellings the message
lists them, and when it holds none the policy is plainly missing.

Only the policy's own path leaves `dirty_worktree`. Every other uncommitted path
the report did not read still counts when it lies under the roots of the policy.
A policy the working tree refused has no roots of its own, so they come from its
committed copy at the analyzed revision. When nothing valid is committed there
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

On a controlling terminal, a TypeScript scan with a committed policy whose
compiler check fails offers interactive setup covering everything
`--prepare-compiler` runs standalone, plus the project package manager. It
names the failing check and states that a confirmed, successful choice makes
Coach recheck readiness and resume the same scan, then shows a menu of up to
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

## What repository configuration can run

A scan reads the repository through git with `core.fsmonitor` disabled and without taking the index lock, and it runs no hook, external diff, textconv, pager or askpass program. It also removes from git's environment the settings of the caller that change what the pathspecs it builds mean or which file `git config` reads: `GIT_LITERAL_PATHSPECS`, `GIT_GLOB_PATHSPECS`, `GIT_NOGLOB_PATHSPECS`, `GIT_ICASE_PATHSPECS` and `GIT_CONFIG`. Everything that says where the repository is stays as the caller set it, as in v0.11.0 (`GIT_DIR`, `GIT_WORK_TREE`, `GIT_COMMON_DIR`, `GIT_INDEX_FILE`, `GIT_NAMESPACE`, `GIT_CEILING_DIRECTORIES`, `GIT_DISCOVERY_ACROSS_FILESYSTEM`, `GIT_OBJECT_DIRECTORY`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`), so a work tree whose git directory is elsewhere is scanned through them, and so are the objects of a server-side hook, which arrive through a quarantine directory and the repository's own. Configuration the caller supplies through `GIT_CONFIG_COUNT` with its keys and values, `GIT_CONFIG_PARAMETERS`, `GIT_CONFIG_GLOBAL`, `GIT_CONFIG_SYSTEM` or `HOME` is left too: every git command reads it alike, Coach's partial-clone check included, and a container that sets `safe.directory` that way needs it. `GIT_CONFIG` is removed because it makes `git config` read a single file and no other git command honours it. `GIT_NO_REPLACE_OBJECTS` and `GIT_REPLACE_REF_BASE` are left as the caller set them. Git sets `GIT_DIR`, `GIT_WORK_TREE` and `GIT_INDEX_FILE` for a hook, so a pre-commit hook that runs Coach scans the repository git gave it, and during a partial commit it reads git's temporary index and not the repository's own: that index is a lock file, which Coach reports as a locked index and so offers neither a restore nor a new policy, and a staged edit that the partial commit leaves out is not disclosed. It reads the source snapshot as the committed blobs, byte for byte, with `git ls-tree` and `git cat-file`, so no smudge filter runs (a git-lfs install defines one globally, and a committed `.gitattributes` can select it for every file) and no line-ending conversion, `$Format$` substitution or `export-ignore` changes what is analyzed; the snapshot used to be `git archive`, which applied all of them. Like `git archive`, it refuses a tree that holds a git directory (`.git` in any case, and the spellings that NTFS and HFS+ fold to it: `git~1`, trailing dots and spaces, and ignorable code points) or a path component that is empty, `.` or `..`, with an error that names the path. The scan stops and does not skip the entry, because a git directory in the snapshot is a repository whose config names programs git runs, and a skipped entry would be a report on a tree other than the one committed. The `go list` that finds the production files for `--build-target`, and the load of a Go project's packages, run with `-buildvcs=false` and `GOTOOLCHAIN=local`, so go neither runs git in the snapshot nor fetches a toolchain its `go.mod` names. The Go project analysis refuses a tree with such a path in the same way before it writes the tree to a temporary directory for `go list`, so one rule holds everywhere a committed tree is written to disk. That read ends after 10 minutes if git stops answering, with an error that names the bound. A committed file larger than 32 MiB is read past and not held, and its path stays in the snapshot as an empty file, so the tree keeps its shape and a `//go:embed` of it still resolves; no analyzer reads a file that large (source is refused past 2 MiB, a policy past 1 MiB), and a source file that large is reported as `file_too_large` with its path by the read that analyzes it, in a `--baseline` scan and in a `--base` scan alike (a `--base` scan reads each changed file with `git show`, which holds a file to the same 32 MiB; the base side of a modified file is reported as `base_analysis_failed`, as it is for a file past the analyzer's own limit). A Go project whose policy sets `required_layer` is the exception: its layer-bypass search copies every file of the revision to disk through that same `git show`, so one file past 32 MiB, of any type, ends the scan with exit 1 and an error that names the file and the budget (`git stdout exceeded 33554432-byte budget`). v0.11.0 ends the same way. The snapshot writes at most 4 GiB in all (a file counts once for every path that names it): the files an analyzer reads are written first, the rest become empty files once the total is spent, and the scan fails only when the files an analyzer reads pass 4 GiB by themselves. A `--baseline` scan reads the content of each file it analyzes from one `git cat-file --batch`; each request ends after 30 seconds, as an error that names `git cat-file`, and a file larger than 32 MiB is not held and gets a `file_too_large` diagnostic. It also fetches nothing: in a partial clone, an object that is not in the local object store (a blob left out by `--filter=blob:none` or `--no-checkout`, say) is reported as unavailable instead of being fetched from the promisor remote, so no `GIT_SSH_COMMAND`, `core.sshCommand` or credential helper runs and no object is written. A scan that needs such an object fails with Coach's own message, which says how many objects the snapshot lacks, names the first, and gives the batch below, and `--check-project` reports that it could not check whether the policy is committed, says that the policy's content is not in the partial clone and that Coach does not fetch objects, and names the `git cat-file -e '<revision>:<path>'` command that fetches it, and the batch that fetches every file of the revision (shown only when the names can be printed safely). Any other file the check reads from the revision, `package.json` for example, is explained the same way, and a scan that stops at missing files says how many and names the first, with the batch; fetching only the policy is not always enough. To run again, bring the missing objects in yourself, in your own shell where git may fetch them. For `--check-project`, run the command the message names: it fetches the policy's blob. A scan reads every file of the revision it analyzes (`HEAD`, and with `--base` the merge base of the base and `HEAD` too), so fetch what the revisions list with one command, without running any file through a filter. The message prints it, `for pass in 1 2 3 4; do ids=$(git rev-list --objects --missing=print --no-walk <revision> | sed -n 's/^?//p'); [ -n "$ids" ] || break; printf '%s\n' "$ids" | git fetch origin --no-tags --no-write-fetch-head --filter=blob:none --stdin || break; done`, where `origin` is the promisor remote, and with `--base` it lists the merge base and `HEAD` together as `{ git rev-list ...; git rev-list ...; }`. It repeats until nothing is missing, because a clone made with `--filter=tree:0` lacks the trees as well: the first pass fetches the trees of the revision, and only then can the next list the blobs they name. A pass that finds nothing missing ends it without running `git fetch`, which would otherwise read the remote's refspecs and update refs. One paste is enough for a `tree:0` clone, a `blob:none` clone and a diff. `git backfill` is the alternative in a git new enough to have it (it is absent from git 2.47). A clone made with `--filter=tree:0` lacks the root tree of `HEAD`, which nothing can be listed without, so on a git that never fetches (2.44 and later) a scan or `--check-project` is refused before anything is read, with `HEAD` as the revision that lacks objects; a git before 2.44 refuses it with the rest of the revisions it reads. `--no-walk` matters: without it `rev-list` walks the whole history and the fetch brings in every blob that ever was, which is what a partial clone exists to avoid. A diff (`--base`) compares each changed file with its version at the merge base of the base and `HEAD`, which a partial clone checked out at `HEAD` usually lacks: on any git, the run then exits 1 before it analyzes anything, naming the merge base, how many objects it lacks and the batch for the merge base and `HEAD`, and never records the changed files as unreadable and exits 0; `--fail-on-incomplete-coverage` does not change that. `git fetch` and `git fetch --refetch` do not help: in a `--filter=blob:none` clone they bring commits and trees, not the blobs the filter leaves out, and the scan fails the same way afterwards. Not fetching rests on `GIT_NO_LAZY_FETCH`, which git reads from 2.44. An older git ignores it and would fetch, so in a partial clone Coach first asks which objects of the revisions the scan reads are missing, with `git rev-list --objects --missing=print --no-walk --quiet`, which lists them and fetches nothing: `HEAD`, and with `--base` the merge base of the base and `HEAD`. If none is missing there is nothing to fetch and the scan runs as usual. If any is missing, or git cannot answer, Coach runs nothing else and exits 1, naming the git version, the revision and how many objects it lacks, and offering git 2.44 or later or the batch above. An ordinary clone has nothing to fetch and is not affected. A `remote.<name>.promisor` setting counts as the git it is written for: a key with no value, or any non-zero number, is a promisor remote, and one git cannot read is an error. The one channel left is a clean filter: when `git status` meets a modified tracked file that a repository's `.gitattributes` gives a `filter.<name>.clean` program, git runs that program as defined in the repository's own configuration, and coach does not override it, because disabling filters would make every git-lfs file read as modified. The repository's own configuration is the trust boundary, as it is for git itself.

## Diff-mode caveats

- Dirty / uncommitted files are ignored as analysis input. A TypeScript project scan emits `worktree_report_reflects_committed_head` when relevant worktree changes exist; the report still applies to committed HEAD. Supported-language file-local dirt uses `worktree_changes_not_analyzed`. Unsupported dirt uses `worktree_not_clean`. A status-check failure uses `worktree_status_check_failed`.
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

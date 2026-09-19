# Loop protocol

Re-entrancy, state file, git, scope, baseline, and report. Load this at
Prepare. One invocation = one outer pass, then commit the report and
stop.

## State file

Path: `.cleanup-loop.md` at the repository root. Track it with the
branch (and the PR, when one exists). The reviewer deletes it at
merge. Do not gitignore it.

Write the pass header after every Prepare blocker has passed and
before any coach or SOLID work. Skeleton:

```markdown
# Cleanup loop

## Pass 1
- started: 2026-09-14T12:00:00Z
- step: prepare
- mode: pr
- base: origin/main (a1b2c3d, from: gh pr view)
- glob: none
- window: --base origin/main
- cap-window: pass 1 of 5

## Coach
- coach binary: (path rank / resolved digest / verified checksum or built commit)
- policy revision: (merge base / user-named at HEAD / none)
- policy blob: (`pcfg_…` at that revision, matched at HEAD / mismatched → scan skipped / none)
- pre-coach check: `<test-cmd>` exit 0; failed-tests: none; lint-violations: none (inventory, with counts); diagnostics: none
- tier 1: (commands / Stage-A count / Stage-B items / holding tests / SHAs)
- tier 2: absent
- tier 3: skipped

## SOLID baseline
- solid-base:          (the commit SOLID starts from; step 5 diffs against it)
- test-cmd:            (exit 0)
- lint-cmd:            (exit 0)
- failed-tests: none
- lint-violations: none (one entry per violation, `file | rule | message`, counted)
- diagnostics: none
- comment-count-method:

## Scope
- path/to/file.ts  todo

## Structure findings
- none

## Pending rulings
- none

## Reports
```

Keep these sections, in order:

1. **Pass header** — pass number, start time (ISO-8601), current
   step (`prepare`, `coach`, `solid`, `verify`, `done`), `mode` (`pr`
   or `branch`), resolved `base` with its OID and the step that
   produced it (`inferred` when the git-only path chose the default
   branch), and coach window (`--base <ref>` or `--baseline`). One
   header only: each pass rewrites it. Edit `step` as each phase
   starts, so a resumed pass knows where the previous one stopped.
2. **Coach section** — the pre-coach check (per command: exit status,
   failed tests by full name, the lint inventory, and other
   diagnostics, from Prepare's start-proof run); then per tier:
   commands, Stage-A count, each Stage-B item
   (path, rule, severity, confidence), the holding test each fix
   added, Stage-A-only items with reason, files edited, SHAs; or
   `coach: skipped (<reason>)`. Tier 2 records the detected path or
   `absent`, and whether `--check-project` ran.
3. **SOLID pass-1 baseline** — written **after** all coach tiers:
   `solid-base` (`git rev-parse HEAD` at 4a, after the last coach
   commit), failed tests by full name, lint violations as
   the lint inventory, test command, lint command, comment-count
   method. Step 5 reviews `git diff <solid-base> HEAD`: 4e commits as
   it goes, so by then a bare `git diff` is empty and proves nothing,
   while the merge base would drag the PR's own feature work and
   coach's licensed Stage-B fixes into a gate that exists to forbid
   behavior change.
4. **Scope list** — one line per file:
   `path  <todo|in-work|clean|ruling (category)|ruling (decision)>  [limit/reason]`.
5. **Structure findings** — one line per accepted SOLID finding, in
   the `solid-diagnostics.md` form ending `apply` or `defer
   (<reason>)`; written before the 4c edits, kept across passes;
   empty is `none`.
6. **Pending rulings** — one item per line; empty is `none`.
7. **Appended reports** — one `### Pass N` block per completed pass,
   holding that pass's Output block verbatim.

## Re-entrancy

### Whose checkpoint is it

`.cleanup-loop.md` and the `Cleanup-Loop` trailer are ordinary tracked
content, so on an incoming PR they are the contributor's data, not
evidence that this skill ran. An author can ship a checkpoint whose
header matches the invocation, mark every eligible path `clean`,
append `DONE — thorough`, and put the trailer on the same commit as
the source change. The newest trailer-bearing commit is then `HEAD`,
the re-admission scan compares `HEAD` with `HEAD` and finds nothing,
and every DONE-shortcut condition is satisfied without a pass ever
having run. The comparison is sound; its **reference point** was not.

A checkpoint is trusted when one of these holds, and only then:

- this invocation wrote it;
- it is reachable from the **merge base** — it was established on
  the reviewed base branch, not by the change under review;
- `mode: branch` on the default branch, where the branch itself is the
  reviewed trunk;
- the invocation explicitly accepts it (`trust-checkpoint`), which is
  how an operator resumes their own loop on a PR branch across
  sessions, and how a CI re-invoke continues one.

Otherwise the checkpoint is a **record, not a certificate**: keep its
Reports for history, ignore its `clean` statuses and its `DONE`, enter
every in-scope path as `todo`, and record one pending ruling
(`checkpoint arrived with the change under review — statuses rebuilt;
re-run with trust-checkpoint to continue it`). Say it in one line so
the operator can opt in next time. No stored checksum fixes this: it
would sit in the same author-controlled diff.

### Re-entrancy rules

- Missing `.cleanup-loop.md` → this is pass 1. Create it once Prepare
  has passed every blocker.
- Header says pass N and Reports has a `### Pass N` block → this pass
  is N+1.
- Header says pass N and Reports has no `### Pass N` block → an
  earlier pass stopped before finalize. Resume that N. Reconcile with
  `git log --grep="Cleanup-Loop: pass=N$"` (anchor the number so
  `pass=1` does not match `pass=10`) so you do not redo committed
  work. The header's `step` says where it stopped: copy it to
  `resumed-from: <step>` before Prepare rewrites `step`.
- Either `DONE` status → compute this invocation's scope **and every
  invalidation the Scope rules define** first, then re-emit and stop
  only when the checkpoint is trusted (above) and that set is empty:
  no path in that scope is missing from the saved list, the header's
  mode, base, and glob match this invocation, no listed path's
  content changed since that finalize, **the invocation resolves
  no `ruling (decision)`, and it grants no permission the saved
  report recorded as withheld**. Consume the Scope result here; do
  not restate a content-only test as if it were the whole of
  eligibility.
  A resolution with no file change is exactly the answer this loop
  asked for, and a shortcut that looks only at content re-emits the
  old report and stops before reconciliation can act on it — the
  operator would have to invent a dummy commit to be heard. A
  newly-supplied `allow-behavior-fix` or `allow-unverified-coach` is
  the same act: the report asked for it by name. Re-emitting DONE at
  a licence the last pass requested answers "I supplied the flag you
  advertised" with the old report.
  Reopening for any of these reasons restarts the cap window the same
  way. Otherwise start pass N+1: new paths enter as `todo`, re-admitted paths return to `todo`,
  every other status is kept. A developer commit that only adds a
  file is the case a scope-blind shortcut misses. Starting over on
  purpose is a human act: delete `.cleanup-loop.md` in a commit.
  The cap bounds work on one scope, not the file forever: reopening
  after `DONE — max iterations` restarts the cap window for the newly
  eligible work, noted in the header as `cap-window: restarted at
  pass N (scope changed)` so the cap stays auditable. Nobody should
  have to delete the state file to get a file added after the cap
  looked at.
- Cap is 5 outer passes across re-invokes. Pass 5 that is not DONE
  reports `DONE — max iterations`.

Every pass ends with a finalize commit that carries the appended
report (see Git). That is why a re-invoke starts from a clean tree
and why a fresh checkout of the branch contains every report.

A later re-invoke repeats Prepare → Coach tiers → SOLID → report.
If Stage-B is already empty, phase 1 is a short scan that records
`coach: clean` and falls through to another SOLID pass.

## Scan window

Detect **PR context** first. First match wins. Record `mode` and
which step won. Do not switch branches. A detached HEAD
(`git symbolic-ref -q HEAD` fails) is a Prepare blocker before any of
this: loop commits must land on a branch.

0. Invocation said `whole-repo` → `mode: branch`; stop here. It wins
   over a PR number or a base ref given in the same invocation; say so
   in one line.
1. Invocation PR number or URL —
   `gh pr view <n> --json number,baseRefName,headRefName`, or the GitHub MCP tool `pull_request_read` (method `get`: `head.ref`, `base.ref`) when the harness offers that instead of `gh`. HEAD
   must already be that PR's head; otherwise a blocker.
2. Invocation-supplied base ref. Forces `mode: pr` against that base
   even when a GitHub PR targeting another branch exists.
3. The current branch's open PR: `gh pr view --json
   number,baseRefName`, or the GitHub MCP tool `list_pull_requests`
   with `state: open` and `head: <owner>:<branch>`, owner and repo
   parsed from `git remote get-url origin` — only when that URL is on
   github.com. Exactly one open PR → its `base.ref`.
4. `$GITHUB_EVENT_NAME` is `pull_request` and `$GITHUB_BASE_REF` is
   set.
5. `$CLAUDE_CODE_BASE_REF` is set, names a branch other than HEAD's,
   and resolves in this repository after `git fetch origin <ref>`.
   Claude Code Remote sets it to the branch the session is forked
   from, which is the natural base of the session's work branch. In
   any other checkout (a scratch repo, a second clone) it names a
   branch that does not exist there — ignore it in one line, never
   block on it. Equal to HEAD's branch → ignore it too (a session
   started on a PR branch has that PR found by step 3).
6. Git-only path — no PR found by `gh` or MCP (`gh` missing, erroring,
   or printing "no pull requests found"), and no env var applies.
   Resolve the default
   branch, first that succeeds: `git symbolic-ref -q
   refs/remotes/origin/HEAD`; `git remote set-head origin --auto`
   then the same `symbolic-ref` (needs the remote; failure is
   tolerated); `origin/main`; `origin/master`; local `main`; local
   `master` (each via `git rev-parse --verify -q "<ref>^{commit}"`).
   None → one-line blocker: pass a base ref or `whole-repo`. Then:
   - `git rev-parse --abbrev-ref HEAD` *is* that default branch →
     `mode: branch`;
   - otherwise → `mode: pr` against the default branch, recorded as
     `base: <resolved> (<oid>, from: inferred)`.

Missing or failing `gh` is one line of narration and never a blocker.
Do not repair credentials. Claude Code Remote has no `gh` and no
`GITHUB_*` event vars but exposes the GitHub MCP tools; use them in
steps 1 and 3 exactly as you would `gh`. Never map "gh is missing" to
"no PR exists": a feature branch without `gh` still diffs against the
default branch, so coach Stage-B stays capped to that diff.

- **`mode: pr`** — normalize the base (below). Coach uses
  `--base <resolved-base>`. Scope is the merge-base diff.
- **`mode: branch`** — only on the default branch or when asked for
  `whole-repo`. Coach uses `--baseline`. Scope is all tracked files
  on the current branch. Do not invent `HEAD~N`.

### Base normalization

The invocation and `gh` hand you a *ref*, not necessarily a remote
branch name. Fetch by branch, resolve to one verified ref, and use
that ref everywhere.

```bash
ref="<as supplied or detected>"
branch="${ref#refs/remotes/origin/}"
branch="${branch#refs/heads/}"
branch="${branch#origin/}"
git fetch origin "$branch" 2>/dev/null || true   # no remote copy is not fatal yet
if git rev-parse --verify -q "origin/$branch^{commit}" >/dev/null; then
  base="origin/$branch"
elif git rev-parse --verify -q "$ref^{commit}" >/dev/null; then
  base="$ref"                                     # local branch, tag, or SHA
else
  base=""                                         # blocker: base ref does not resolve
fi
```

`main` and `origin/main` therefore select the same base. An empty
`base` is a Prepare blocker: one line, Output `BLOCKED`, no edits,
stop. Record `base: <base> (<oid from git rev-parse --short>, from:
<step>)` in the pass header and reuse `$base` verbatim for
`git merge-base` and for coach `--base`. Do not treat a failed
resolution as an empty diff.

## Scope

PR mode:

```bash
mb="$(git merge-base "$base" HEAD)" || mb=""
[ -n "$mb" ] || exit 1   # blocker: no merge-base (shallow clone or unrelated history)
git diff --name-only --diff-filter=d "$mb..HEAD"
```

Check `merge-base` on its own. Substituting a failed `merge-base`
into the diff yields `..HEAD`, which exits 0 with no output and would
be misread as an empty diff — the default `actions/checkout`
(`fetch-depth: 1`) produces exactly that. The blocker line names the
cause and the remedy (`git fetch --unshallow origin <branch>`, or
`fetch-depth: 0`).

`--diff-filter=d` drops paths the branch deleted: there is nothing
at HEAD to tidy. A rename lists only its new path. Empty diff →
one-line blocker naming the base, Output `BLOCKED`, stop. Tell the
user that `whole-repo` hunts the branch instead.

Branch mode:

```bash
git ls-files
```

Remove `.cleanup-loop.md`. If the invocation supplied a path glob,
intersect: keep only scope paths that match the glob. A glob that
leaves no path is a one-line blocker naming the glob (`BLOCKED`),
never a vacuous `DONE — thorough`. In PR mode do not add files the
diff did not touch. In branch mode do not add untracked files.

Compute this list before the state file is created or edited, so a
blocker never leaves a dirty tree.

### Reconcile, never reset

On pass 1, Write the skeleton and list every scope path as `todo`,
with the pre-marks below. On every later pass, Read the state file
and Edit the Scope section by reconciling membership while preserving
progress:

- A path already on the list keeps its status, its limit, and its
  reason. Never rewrite an existing line back to `todo` except by one
  of the two invalidations below.
- A path new to the scope is added as `todo` (or pre-marked).
- A path that left the scope is dropped from the list. If it was
  `in-work` or `ruling`, say so under `questions`. Its pending ruling
  stays in Pending rulings.
`ruling` covers three unlike things, and only one of them is
permanent. A **category** ruling (`config/docs — these rules do not
apply`) says the rules never applied to that file. A **decision**
ruling says one production file is waiting on a human answer. A
**permission** ruling says the remedy is known and withheld: coach
classified a Stage-B defect on that path and the invocation did not
say `allow-behavior-fix`. Write the kind on the line — `ruling
(category)`, `ruling (decision)`, or `ruling (permission)` — because
they reconcile differently.

Three invalidations:

- A `clean` path returns to `todo` when its **content** differs from
the last state this loop certified (someone pushed new work between
passes).
- A `ruling (decision)` path returns to `todo` when its content
  differs, **or** when the invocation says that ruling is resolved.
  Otherwise a file is frozen by the very question the pass raised: the
  developer answers it, rewrites the file, re-invokes — and scope
  reconciliation keeps `ruling`, coach skips it, both work-set
  selectors skip it, and `DONE — thorough` reports the repository
  finished while that file was never looked at again.
  Re-admission is not authorization. The pending ruling stays in
  Pending rulings as a restriction on **that item**: the file is
  diagnosed and tidied everywhere else, and the blocked refactoring
  waits for the decision that releases it. A recorded finding whose
  file has changed is stale — re-diagnose it rather than reusing the
  old evidence.
- A `ruling (permission)` path returns to `todo` when its content
  differs, **or** when this invocation supplies the token it was
  waiting on. That is the whole point of asking: the operator read
  `questions`, passed `allow-behavior-fix`, and expects the fix this
  loop already described. Without this the gate is a trap — the pass
  reports the defect, marks the file finished, and no later
  invocation can reach it, because coach only touches `todo` /
  `in-work`. Re-admission is not blanket authorization here either:
  any `ruling (decision)` on the same file stays a restriction on
  that item.
- A `ruling (category)` path never re-admits on content. The rules
  still do not apply to a rewritten lockfile or workflow.

Compare trees, not commit lists:

  ```bash
  last_loop="$(git log --grep='Cleanup-Loop: pass=' -1 --format=%H)"
  git diff --name-only -z "$last_loop" HEAD --
  ```

  `git log --name-only` prints nothing for a merge commit, so an edit
  made **only** while resolving or integrating a merge is invisible to
  it: an incoming branch touches `notes.md`, the merge also rewrites
  `source.py`, and the log form reports `notes.md` alone while
  `source.py`'s blob has changed. `clean` would then mean "reviewed
  before the merge", not "these contents were reviewed". The tree
  comparison sees it, needs no trailer reasoning, and survives a
  conflict resolution, an amend, or a rewritten branch. It also stays
  narrow: a docs-only merge invalidates nothing but the docs.

  `last_loop` is the most recent trailer-bearing commit, which is the
  previous pass's finalize normally and a partial pass's own last
  commit when resuming — so a resumed pass does not re-admit the work
  it just did.

  Write `re-admitted: <sha>` on that line, and re-admit the colocated
  test file of a re-admitted production file (`re-admitted: colocated
  test of <path>`): tests-first must be able to hold the behavior the
  foreign commit added. Nothing else changes a `clean` line.

Pre-mark before any tidy:

- `clean` with reason: binaries, lockfiles, generated, vendored.
- `ruling` with reason `config/docs — these rules do not apply`:
  instruction files (`AGENTS.md`, `CLAUDE.md`, `GEMINI.md`,
  `copilot-instructions.md`), markdown/docs, license files
  (`LICENSE`, `COPYING`, `NOTICE`), dotfiles (`.editorconfig`,
  `.npmrc`, `.nvmrc`), static assets (`public/`, images, `.svg`),
  JSON/YAML config (`package.json`, `tsconfig.json`, `biome.json`),
  GitHub workflow YAML, Dependabot, and `*.config.*`
  (`astro.config.mjs`, `playwright.config.ts`, `vitest.config.ts`).
  Do **not** pre-mark `*.schema.ts` or other source that happens
  to encode a schema — that is production code. Changing a
  serialized format or schema *shape* is still a ruling during
  SOLID / coach; tidying comments in that file is not. These
  pre-marked lines are not work: record **one** pending-ruling entry
  for the group (`config/docs pre-marked ruling — confirm skip`), not
  one per file, and count them as done for `DONE — thorough`. These
  are `ruling (category)`; a production file blocked by an open
  decision is `ruling (decision)` and reconciles as above.

Coach Stage-B only touches paths still `todo` / `in-work` on this
list, plus mechanical call-sites a fix requires. A Stage-B signal on
 a `clean` or `ruling` path, or outside the invocation glob, is listed
under `questions` and left alone.

Unit of scope = the whole file. Set `in-work` when you start a file.
Set `clean` only after every comment in it has a decision and the other
rules agree. Set `ruling (decision)` when an open ruling blocks it,
naming the item in Pending rulings; that status is a pause, not a
verdict, and it lifts when the content changes or the decision arrives.
Set `ruling (permission)`, never `clean`, on a path carrying a Stage-B
item this pass withheld for want of `allow-behavior-fix`: the file is
still tidied everywhere else, but calling it `clean` would certify work
the pass declined to do. DONE across re-invokes is when no file is
`todo`.

A coach phase skipped for want of `allow-unverified-coach` leaves a
wider claim unmet: nothing in scope was scanned at all. Record
`withheld: allow-unverified-coach` in the header beside the coach line.
When a later invocation supplies it, that is an invalidation, and the
Go/TS/TSX paths in scope re-admit to `todo` — their `clean` marks
certified the SOLID pass only, never a coach examination that never
ran. Re-admit them once, on the invocation that grants the token.

### Branch-mode work set (this pass)

Whole-branch scope is often dozens of source files. One invocation
still does one pass, then stops. Select the SOLID work set in this
order:

1. **Eligible coach set**: paths named in this pass's coach signals
   (any stage) that are still `todo` / `in-work` **and** inside the
   invocation glob, plus their colocated tests. Filter by status and
   glob first; `clean`, `ruling`, and out-of-glob paths never count.
2. **Fallback**: if that eligible set is empty — whether coach named
   nothing, or named only paths the filter removed — at most 8
   remaining `todo` source files, highest comment count first.

A persistent Stage-A signal on a `clean` file must not starve the
fallback: `a.ts clean` with a signal and `b.ts todo` selects `b.ts`.

Leave every other `todo` file `todo`. Do not mark it `clean`.
Report `PASS n/5` with the remaining `todo` count in `questions` if
the list is not empty. PR mode is unchanged: walk the whole PR
scope.

Do not touch a file already marked `clean` unless Prepare re-admitted
it.

Files not in scope: change them only for mechanical call-site updates
a rename or extraction makes necessary.

### Generated, vendored, lockfile, binary

Do not apply comment or SOLID rules. Mark `clean` with the reason
(`lockfile`, `generated`, `binary`, `vendor`).

### Snapshots and goldens

Regenerate with the framework. Never hand-edit. An intentional change
already in this PR must explain each snapshot difference.

### Large files

If full tidy would ~2× the PR (or, in branch mode, swamp the pass
with low-value changes), do the high-value part only. Record the
limit on that file's scope line.

## Test and lint commands

Discover from project instructions, then task runner (`mise.toml`,
`Makefile`, `Taskfile.yml`), then CI, then the package manifest.
Record them. Use the same commands in later passes.

If project instructions name a version-manager prelude (`nvm use`,
`mise install`), run it before proving start. The same prelude is in
effect before any coach command that needs the repo's Node.

If they name several validation commands (e.g. `biome check`,
`actionlint`, `yamllint`), record all of them. The lint baseline is
one inventory of every violation those commands report, compared by
identity and multiplicity (Green).

Prefer the unit test command named in instructions (`npm test`,
`go test ./...`). Record the e2e command only when e2e files are in
scope.

Record the **check** form of each command. A manifest `lint` script
is often a fixer (`eslint . --fix`, `biome check --write`); bind to
the tool's `--help` for its report-only form (`--check`, `--no-fix`)
and record that. A fixer would rewrite tracked source during the
start-proof run, trip the dirty-tree guard on the next invocation,
and hide the very violations the baseline must list.

Prove they *start* during Prepare (they launch and produce output). A
red suite is not a start failure. A command that cannot start is a
blocker for the whole skill. Prepare runs on a clean tree, so its
snapshot is already committed: if the run changed anything — the
worktree, or the index, because a fixer-style script ran `git add` —
restore with the block below, reading `pre_tree` as
`git rev-parse HEAD^{tree}` instead of writing one. The same content
tests then apply before continuing.

Record that run's **pre-coach check**: per command
its exit status, its failed tests by full name, its lint
its lint inventory, and every other diagnostic it printed, each
as one stable identity (see Green). The coach regression guard in
`coach-phase.md` compares each Stage-B fix against it.

### Validation side effects after Prepare

Once the pass holds uncommitted work, HEAD and the index are no longer
the tree to restore to: `git checkout -- <path>` on an unstaged edit
returns the file to its committed content and discards the pending fix
together with the formatter's rewrite, and "delete untracked" cannot
tell a new holding test from a generated artifact.
So around every test or lint run that follows an edit:

```bash
snap="$(mktemp -d)"                                # outside the worktree
git add -A -- <every path you edited or created>   # index = the pre-run snapshot
pre_tree="$(git write-tree)"                       # content identity of that snapshot
git ls-files --others --exclude-standard -z | sort -z > "$snap/untracked.pre"

<test or lint command>; run_status=$?

[ "$(git write-tree)" = "$pre_tree" ] || git read-tree "$pre_tree"  # the run staged over you
git diff --name-only -z > "$snap/dirty"
[ ! -s "$snap/dirty" ] || xargs -0 git checkout -- < "$snap/dirty"  # worktree <- index
git ls-files --others --exclude-standard -z | sort -z > "$snap/untracked.post"
comm -z -13 "$snap/untracked.pre" "$snap/untracked.post" > "$snap/new"
[ ! -s "$snap/new" ] || xargs -0 rm -f -- < "$snap/new"

[ "$(git write-tree)" = "$pre_tree" ] || echo "stop: staged content differs from the snapshot"
git diff --quiet || echo "stop: worktree differs from the staged snapshot"
```

**Identify the snapshot by content, never by status.** `git status`
reports paths and status letters, not contents. A validation wrapper
that rewrites a staged file *and stages its rewrite* leaves the
porcelain byte-identical — `M  source.txt` before and after — while
the index tree moves from one object to another, and `git diff --name-only`
is empty because the worktree now agrees with the corrupted index, so
a restore driven by it has nothing to do. The intended fix is gone and
the check calls the tree preserved.
`git write-tree` is the content identity that catches it: it turns the
whole staged snapshot into one object id, and `git read-tree`
puts that exact snapshot back. Never hold `--porcelain -z` in a shell
variable either — command substitution strips NUL bytes, so the
comparison is not the byte-for-byte one it looks like. Keep
NUL-delimited inventories in files.

Take the untracked difference, not the whole list: `comm -z -13`
removes only what this run created, so an untracked file that was
already there — a developer's scratch note, a local config — is still
there afterwards. A blanket `ls-files --others | rm` deletes those
too. `comm -z` is GNU. On a BSD userland it exits
`illegal option -- z`, the difference file is empty, the run's
artifacts survive, and **both closing tests stay silent** because
neither of them looks at untracked paths — read the two inventories
with a NUL-safe loop there and delete only the paths absent from
`untracked.pre`. The `[ ! -s … ]` guards are there so BSD `xargs`
never runs its command once with no operands; that is what replaces
GNU `xargs -r`. Every inventory is NUL-delimited and
every removal ends its options with `--`: a path is not a word, and the
newline-delimited form splits `generated file` into `generated` and
`file`, **deleting those two tracked files** while leaving the
artifact. Spaces, tabs, newlines, and leading dashes are all ordinary
in a filename a formatter or a test may write. Use the same `-z` /
`-0` / `--` discipline wherever this skill restores or deletes by
inventory, Prepare included.

A staged holding test shows as `A `, never `??`; a rewritten pending
fix shows as `MM`, and the checkout returns it to the staged content.
When the run rewrote or staged anything, run it again on the restored
tree so the result you record describes the content that will be
committed. The two closing tests are the whole acceptance: the staged
tree hashes to `pre_tree` again, and no unstaged difference is left.
To discard your own staged work, use `git checkout HEAD -- <path>`
(a file HEAD does not have: `git rm -f -- <path>`); plain
`git checkout -- <path>` no longer discards once the work is staged.

Do not record the SOLID baseline until after coach.

## Comment-count method

Record one method that returns the same number for the same tree.
Prefer, in order:

1. `tokei --output json` on this pass's work set, summing `comments`.
2. Else count lines in this pass's work set matching
   `^\s*(//|#|/\*)` with the harness search tool, excluding
   lockfiles, generated artifacts, and binaries.

Edit the method into the baseline section as one line. `before` in
the report is the count at the start of this pass's SOLID phase.
Unvisited `todo` files are not in the metric.

## Git

- Commit only. Trailer every commit as the last line of the message:

  ```bash
  git add -- <files> .cleanup-loop.md
  git commit -m "$(printf '%s\n\nCleanup-Loop: pass=%s\n' "$subject" "$N")"
  ```

  Do not use `git commit --amend`.
- **Finalize commit, every pass.** After the report is appended to
  `.cleanup-loop.md` and the header says `step: done`:

  ```bash
  git add -- .cleanup-loop.md
  git commit -m "$(printf 'chore(cleanup-loop): record pass %s report\n\nCleanup-Loop: pass=%s\n' "$N" "$N")"
  git status --porcelain   # must print nothing
  ```

  Do this even when the pass changed no source file, so the next
  invocation starts from a clean tree and a fresh checkout carries the
  report. The report cannot embed this commit's own SHA, so `commits`
  lists source commits only; the finalize commit is always the last
  match of `git log --grep="Cleanup-Loop: pass=N$"`.
- No push unless the invocation said `push` / `open the PR`. When it
  did, push after the finalize commit: `git push` to the tracking
  remote, or `git push origin HEAD` when no upstream is set. Never
  `--force`.
- No amend, rebase, squash, reword, force, branch create/delete, PR
  title/body/label/review changes.
- Remove committed work with `git revert`. Discard uncommitted work
  with `git checkout HEAD -- <file>` (`git rm -f -- <file>` for a file
  HEAD does not have). Plain `git checkout -- <file>` restores to the
  index: that is the validation-restore step above, not a discard.
- One concern per SOLID commit: tests *or* structure *or* comments.
  Coach Stage-B commits are their own concern; each carries the fix
  and the holding test that proves it.
- State-file updates may ride any commit.
- A characterization test may share the commit with the refactor it
  permits, if a separate commit would make the suite weaker for a time.

## Green

A command's result is four things, and the baseline records all
four: its **exit status**, the set of failed tests by full name, a
lint **inventory** — one entry per reported violation, identified as
`(file, rule-id, message with its quoted symbol **and its measured
numbers**, source location stripped)`, counted with multiplicity —
and everything else it printed that is neither a test name nor a lint
violation: a compile
or type error, a parser crash, a missing binary, each recorded as one
identity per line of the form
`<command>:diagnostic:<path-or-package>:<message-head>`.

A bare `(file, rule-id)` **set** is not enough, and this is the hole
it leaves: a file with one `no-unused-vars` violation lints at exit 1;
a refactor that leaves that violation and adds a second unused
variable in the same file still lints at exit 1, and both sides
reduce to the same one-element set, so "no worse than baseline"
accepts twice the debt. Two violations of one rule in one file are
two entries, and one violation swapped for another is a different
entry at the same count. Keep the message text (the part naming the
symbol) in the identity, and keep the violation's **source location**
out of it, so an extraction that moves a violation down the file is
still the same violation.

Strip the location; do **not** strip every digit. A number in a lint
message is often the measurement the rule exists to report, and
erasing it accepts a worse violation as unchanged. Ruff's `PLR0913`
says `Too many arguments in function definition (6 > 5)`; blanket
numeric normalisation turns six arguments and seven into one
inventory entry at the same path, rule, count, and exit status, so
the pass calls that regression green:

```
normalising every number:     seven == six  -> accepted
preserving measured numbers:  seven != six  -> red, correctly
                              moved six-arg function == six -> still green
```

So prefer a structured format (`--output-format json`, `-f json`)
and build the identity from its fields, where path, row and column
arrive separately and the message needs no scrubbing at all. With
text-only output, strip the location fragments you can *identify* —
the `path:line:col:` prefix, a `(line N)` or `at line N` aside — and
leave the rest of the message intact. Measured complexity, argument
and statement counts, lengths, nesting depth, and the rule's own
threshold all stay. If you cannot establish a number's role, keep
it: a false red costs one investigation, a false green ships the
regression.

Green = all four match the recorded post-coach baseline by identity:
same exit status, same failed tests (through any recorded rename
mapping, `tidy-rules.md`), the same lint inventory with the same
multiplicity, same diagnostics. An entry the baseline does not hold is
red even when the totals agree; a higher count for an entry it does
hold is red too. Compare by identity, not totals, not line numbers.

A nonzero exit is never green on its own evidence. `go test ./...`
whose package no longer compiles exits 1 and prints
`undefined: Missing` with **no** `--- FAIL: Test…` line and no lint
pair; comparing only tests and pairs sees `{} == {}` and calls a
broken tree green. TypeScript build and type diagnostics, and any
linter that dies before it reports, fail the same way. So: a command
whose exit status differs from the baseline's, or that prints a
diagnostic identity the baseline does not hold, is red — repair it or
discard the change, whatever the test and lint sets say. A
baseline-red command stays green only while it reproduces the exact
recorded status and identities.

Record the SOLID baseline from the post-coach tree when either holds:

- no SOLID baseline exists in the state file yet (pass 1, including
  a pass 1 whose coach phase was skipped or landed nothing);
- this outer pass's coach tiers landed commits;
- Prepare's re-admission scan found changed content since the
  previous finalize (someone pushed new work; the recorded baseline
  describes a tree that no longer exists).

Otherwise reuse the recorded post-coach baseline. A newly-passing test
from a Stage-B fix is not a green failure. The coach regression guard
already rejected any fix that introduced a failure, so a refreshed
baseline never legitimizes a coach regression.

A red baseline is still green. After the baseline is recorded, do not
repair failures that are in it. A test that fails now and is not in the
baseline is yours to repair, from any pass. Do not skip SOLID when
Stage-B remains after the coach cap — list leftovers under
`questions`, then SOLID.

## Report fields

Field semantics for the Output block in SKILL.md.

- `tests` counts are this pass only, and include a holding test the
  coach phase added.
- `comments` is this pass's work set, not in-scope files only;
  unvisited `todo` files are not in the metric. `before` is the start
  of this pass's SOLID phase.
- `structure` = number of behavior-preserving commits from step 4c.
  One structure change = one such commit. The findings those commits
  applied, and the ones deferred, are the Structure findings section
  of the state file.
- `suite` / `lint` are pass or fail *vs baseline*, not zero-failure.
- `status`: only `PASS n/5`, `DONE — thorough`, `DONE — max iterations`, or `BLOCKED`. Prepare blockers use `BLOCKED`, never `PASS 0/5`.
  A Prepare blocker (missing mise when no PATH coach, dirty tree,
  detached HEAD, unresolvable base, no merge-base, empty diff, empty
  glob, commands that cannot start) still emits this block in chat:
  `BLOCKED` and one `questions` line naming the blocker. It is not
  appended to `.cleanup-loop.md`; a blocker never touches the state
  file.
  - PR mode: `DONE — thorough` iff every scope file is `clean` or
    `ruling`; else `PASS n/5`. A `ruling (decision)` whose content
    changed, or whose ruling was resolved, is `todo` by then and
    cannot be counted as finished.
  - Branch mode: `DONE — thorough` iff no `todo` remains across
    re-invokes. A pass that finished only its work set and left other
    `todo` files is `PASS n/5` with the remaining count under
    `questions`. Never mark unvisited files `clean` to reach DONE.
  - After any Stage-B commit this pass, `DONE — thorough` requires a
    `questions` disclosure that behavior may have changed, naming
    those SHAs; otherwise `PASS n/5`.
  - `DONE — max iterations` on pass 5 if not thorough.
- `commits` lists source SHAs this pass created, one line each. The
  finalize commit is excluded (see Git).
- `kept comments` lists only non-obvious keeps.
- `questions` one line each with a recommendation; omit the section if
  none. Every deferred structure finding has a line here.

Narration: one line per completed Work step, ≤10 words. Blockers in
one line at the time they are found. The person reading can see the
report and the diff, not tool output.

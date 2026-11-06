# Coach phase

Driver of the outer pass. Runs **before** SOLID. Load this after
Prepare. Baseline for SOLID is recorded only after every coach tier.

Coach has no Stage A/B. Classification is ours.

## Invoke

Resolve the binary once per invocation: the first path below that
runs `coach codesignal --help`. Every path but the first fetches and
executes code at run time, so record in the state file **which path
ran and the identity it resolved to** — the binary's `sha256sum`
always, plus the verified checksum or the built commit where the path
supplies one. Two passes that resolve different identities for the
same version is a stop, reported under `questions`.

**Target version.** This skill targets coach **v0.11.0 or newer**:
code-tidy and coach ship as one solution. Record `<binary> --version`
(a source build and the rank-3 Go backend print `dev`; record the
module version or built commit with the binary digest instead). When
the resolved coach is older than v0.11.0 — a PATH coach or a
repository pin — use it (the pin is the repository's decision), add
one `questions` line recommending v0.11.0 or newer, and bind every
flag below to what that binary's `--help` lists. An older coach
reports none of the incompleteness diagnostics, so record a Tier 3
with no finding as `tier 3: no findings; completeness not reported by
coach <ver>`, never clean.

0. **Already provisioned.** `command -v coach` finds one the operator
   installed. Use it; fetch nothing, and do not require mise on this
   path — an offline image with an approved coach is the point. Record
   its path and digest. Prepare resolves this before it asks for mise.

Every tier below runs `<binary>`, the path this ladder resolved. Do
not retype an acquisition command as the invocation: a repository
that forbids installing, or one whose approved coach came from
somewhere else, is then silently bypassed.

1. **Verified release** — coach signs `checksums.txt` with cosign
   (keyless GitHub Actions OIDC) and attests provenance. Where
   `cosign` is on PATH and the release is reachable, download the
   platform archive with `checksums.txt` and `checksums.txt.bundle`,
   then:

   ```bash
   cosign verify-blob --bundle checksums.txt.bundle \
     --certificate-oidc-issuer https://token.actions.githubusercontent.com \
     --certificate-identity-regexp '^https://github.com/lousy-agents/coach/' \
     checksums.txt
   sha256sum -c checksums.txt --ignore-missing    # macOS: shasum -a 256 -c
   ```

   Verification that fails is a stop, never a fall-through to a lower
   rank. Record the verified checksum. Archives exist for darwin
   arm64/x86_64, linux x86_64, and windows x86_64 only.

`<ver>` for ranks 2 to 4 is the coach version `mise.toml` /
`mise.lock` already pins. Default `v0.11.0` when the repository does
not pin coach. Never rewrite a repository's pin; recommend the upgrade
under `questions` instead.

2. Unverified, portable — the GitHub release through mise:

   ```bash
   mise exec github:lousy-agents/coach@<ver> -- coach codesignal --format json <args>
   ```

3. GitHub API refused. Claude Code Remote answers mise's release
   lookup with HTTP 403 (`GitHub access to this repository is not
   enabled for this session`) for a repository that is not attached
   to the session. Use mise's Go backend, which resolves through the
   Go module proxy and never calls the GitHub API:

   ```bash
   MISE_FETCH_REMOTE_VERSIONS_TIMEOUT=60s \
      mise exec "go:github.com/lousy-agents/coach/cmd/coach@<ver>" -- coach codesignal --format json <args>
   ```

   The timeout is load-bearing: mise's default 3 s version-resolve
   window times out behind the remote proxy; 60 s installs in about
   30 s. The module proxy serves an immutable, checksum-database
   backed module for that version; `GOFLAGS=-mod=mod GONOSUMDB=` and
   friends must not be used to disable that verification. This backend
   also prints `dev` for `coach --version`: record the module version
   (`<ver>`) and the binary's digest.

4. Source build from a scratch clone — anonymous git reads are served
   even where the API is not:

   ```bash
   src="$(mktemp -d)"; bin="$(mktemp)"
   git clone --depth 1 --branch <ver> https://github.com/lousy-agents/coach "$src"
   commit="$(git -C "$src" rev-parse HEAD)"   # the tag is mutable; this SHA is what built
   (cd "$src" && go build -o "$bin" ./cmd/coach)
   "$bin" codesignal --format json <args>
   rm -rf "$src" "$bin"
   ```

   Record `$commit` and the binary's digest: `coach --version` prints
   `dev` for a source build and identifies nothing. Use the Go on
   PATH, else the version the repository already pins
   (`mise exec go@<pinned> -- go build …`); never `go@latest`.

### Provenance

Ranks 2 to 4 resolve a **mutable tag**: a retagged release executes
different code in the customer's repository, and two runs are not
reproducible from the version string alone. Those ranks are
**fail-closed by default**. Attempt them only when the invocation
said `allow-unverified-coach`. A repository provenance requirement
(signed or pinned binaries, a cosign or binary allowlist, a security
policy naming those) is the same refusal even if the invocation asked
— **only ranks 0 and 1 qualify**. An npm `package-lock.json` is not
that, and neither is any other lockfile. Without the allow token,
ranks 2 to 4 are not attempted: record `coach: skipped (unverified
acquisition refused; pass allow-unverified-coach or provide a
verified binary)` plus `withheld: allow-unverified-coach` in the
header, so a later invocation that supplies the token reopens the
scope instead of re-emitting a DONE that no coach scan ever backed
(loop-protocol, Reconcile). Then run SOLID. Never substitute an unverified
fetch for a verification you could not perform. `cosign` absent with
no PATH coach is exactly that case.

Scratch-build **only** when `./cmd/coach` exists in cwd (the
repository *is* coach): `mise exec -- go build -o "$(mktemp)"
./cmd/coach` and use that binary. Do not call `go build ./cmd/coach`
in any other repo.

**Push-per-cycle (optional):** push after each Stage-B cycle commit
when the invocation includes `push` / `open the PR` **and** the
branch tracks a remote (`git push` to the tracking remote). Ordinary
PR tidy stays commit-only without that license; do not silently push
on a foreign PR. The same `push` / `open the PR` license also covers
the finalize push (loop-protocol). Never `--force`. Scratch
`go build ./cmd/coach` when `./cmd/coach` exists remains an
acquisition detail only — it does not gate push.

Delete every scratch clone and binary on every exit path. If none of
the paths runs, coach is skipped (`coach: skipped (<reason>)`) and
SOLID still runs.

Coach only reads the repository and writes its report to stdout; the
edits are always ours. Nothing here may grant it credentials, and no
acquisition path may write outside the scratch directory.

Read `<binary> codesignal --help` once per invocation. Do not invent
flags. Bind every flag you add to that help text. The version-manager
prelude from project instructions (`nvm use`, `mise install`) is in
effect before any coach command that needs Node, not only the
test/lint start-proof.

Pass `--no-interactive` on every scan when `--help` lists it (v0.7+).
An agent shell can still attach a controlling terminal, and a v0.11
scan otherwise offers to copy a root `project.json` into
`.lousy-agents/coach.json` (a worktree write) or starts guided policy
authoring. The scan command blocks below show the flag; on an older
coach drop it, or the scan exits 2 as a usage error. Never pass it to
`--check-project` or the Go `--suggest-project-config`: both reject it
with exit 2. Do not pass `--min-severity` or `--top`: they withhold
signals, and Stage A/B needs every one. Do not pass
`--fail-on-incomplete-coverage`; read incompleteness from the report
(Incomplete is not clean, below).

## Skip, do not fail

Read `.cleanup-loop.md` (Prepare already created it), then Edit in
`coach: skipped (<reason>)`. Never Write over it. Continue to SOLID
when:

- no Go/TS/TSX file is in the scope list
- codesignal cannot start after mise is present
- exit 2 for unsupported language or a TypeScript scan that cannot
  resolve a compiler / host Node

A Tier 1 exit 2 for an invalid discovered policy skips Tier 1 only;
Tier 2 still runs (see Tier 1).

Missing `mise` blocks only when no usable coach is already on PATH
and the mise ladder is the remaining acquisition route. A PATH coach
(rank 0) needs no mise. No coach and no mise after the one permitted
install attempt is a Prepare blocker for the whole skill.

Do **not** run `--prepare-compiler` (TTY, mutates the toolchain).
Do **not** run `--suggest-project-config --project-language
typescript` (interactive TTY). Go `--suggest-project-config` never
prompts, but since v0.11 it writes `.lousy-agents/coach.json` into the
worktree it runs in, and `--output` accepts only a repository-relative
path. Run it only as Tier 2's proposal step says: in a detached scratch
worktree.

Coach window follows Prepare `mode` (do not invent flags):

- **`mode: pr`** — `--base <resolved-base>`. Do not switch to
  `--baseline` because the PR diff analyzed 0 files.
- **`mode: branch`** — `--baseline`. This is the whole-branch hunt,
  chosen only on the default branch or when the user asked for
  `whole-repo`. A feature branch without `gh` is still `mode: pr`
  against the default branch. Do not invent `HEAD~N`.

## JSON consume path

Successful scan: one JSON object on stdout.

Always present: `schema_version`, `scope`, `summary`, `signals`,
`diagnostics`, `coverage`.

`schema_version` is `"2"` whenever a valid policy is selected: by
`--project-config`, or (v0.11+) by discovery. A scan without
the flag reads `.lousy-agents/coach.json` at the analyzed revision
(`HEAD` in both modes), else a schema-valid root `project.json` with a
`project_config_deprecated` diagnostic. Otherwise `"1"`.

Version 2 also has `project_changes`, `project_facts`,
`project_summary`, `project_coverage` (`complete`), and
`layer_membership` when the policy declares layers and the count could
be computed (`head`, plus `base` in a diff, each `revision`,
`analyzed_files`, `files_outside_layers`; absent means not computed,
never zero). TypeScript reports add `project_provenance`
(`config_digest`, analyzer version and digest) and `project_scope`.
Architecture findings live in `project_changes` (each with `rule_id`,
`lifecycle`, `changed`, `primary_anchor`, `evidence`,
`machine_evidence`, `recommendation`, `config_digest`) and are also
mirrored into `signals` as `architecture.*` rule IDs, in both modes. So
Tier 3 reads `project_changes`, and Tier 1 drops `architecture.*`.

Each signal: `path`, `rule_id`, `severity`, `confidence`,
`why_it_matters`, `recommendation`, `location.start_row` (0-based).

Exit codes (coach contract):

- `0` — command completed. **Not** "no findings". Signals do not
  change this.
- `1` — operational (not a git repo, unresolvable `--base`, I/O).
  Blocker for the coach phase only; skip coach, continue to SOLID.
- `2` — usage, `project_config_invalid`, unsupported language, or TS
  compiler/host unresolved (`node_unsupported`). Empty stdout.
  Skip-with-reason, continue to SOLID (except Tier 1 with an invalid
  discovered policy: go to Tier 2). Do not "fix" a config by
  inventing layers. Since v0.11 `project_config_invalid` also covers
  an invalid discovered `.lousy-agents/coach.json` (coach never falls
  back to `project.json`), and layer prefixes are also rejected when
  a sibling sorts between an ancestor prefix and its descendant.
- `3` — only under `--fail-on-incomplete-coverage`, which this skill
  does not pass. If it appears anyway, the report is on stdout: read it
  as exit 0 plus incomplete coverage.

Coach analyzes **committed Git objects**, not the dirty worktree.
Commit a Stage-B fix before the next scan or coach will not see it.
The same holds for policy: a worktree `.lousy-agents/coach.json` that
is not committed is ignored (`project_config_uncommitted`).
Stage-B production edits are **on by default**. Opt out with
`solid-only` / `no-behavior-change` / prune-comments-only: classify
Stage-B, list under `questions`, leave the tree unchanged. Legacy
`allow-behavior-fix` / `allow-behavior-change` is a redundant alias
for the default coach-loop.

## Incomplete is not clean

Applies to Tier 3 project scans only; Tier 1 drops these items with
the untrusted policy that produced them. A Tier 3 scan that carries
any item in the first list is **incomplete**: record the counts on the
tier's state-file line and one `questions` line, and never report that
tier as clean. An incomplete Tier 3 does not change `PASS` / `DONE` by
itself, and none of these items authorizes a policy edit. Each
diagnostic kind's `base_`-prefixed form (the base side of a diff)
counts the same.

Makes the tier incomplete:

- `project_import_unresolved`: an in-layer TypeScript import that did
  not resolve; a broken alias can hide a forbidden edge behind it.
  Stage B only when the import is itself broken — the file's tests or
  the compile fail on it; otherwise list it.
- `project_import_non_literal`: an in-layer `import(expr)` /
  `require(expr)` no layer rule can place. Rewriting it to a literal
  changes what loads → Ruling required.
- `project_coverage.complete` false, `project_coverage_incomplete`, or
  `project_lifecycle_indeterminate`. For a TypeScript report, coach
  judges each side from `project_provenance.head` / `.base`
  `.coverage`: `model` must be `complete`, and `bypass` `complete` or
  `not_requested`.
- A scan whose backend saw no code: `project_coverage.counts.files_seen`
  is 0, or `roots_seen` is 0 where present (TypeScript reports carry
  no `roots_seen`), or the policy declares `layers` and
  `layer_membership` is absent. A TypeScript policy scanned without
  `--project-language typescript` lands here: the Go backend reports a
  complete model of nothing.
- `layer_membership.head.files_outside_layers` > 0: record `N of M
  project files outside every declared layer`, with `base` alongside;
  `forbidden_imports` covered only the rest. Extending layers is a
  policy change → one `questions` line, never an edit.

Advisory, not incompleteness:

- `project_external_imports_not_evaluated`: record the count only.

## Stage classification

- **Stage A:** every finding with path, rule, severity, confidence.
- **Stage B include bar:** could cause incorrect behavior, a test
  failure, or a misleading result, **and** the file is in tidy scope
  (or a mechanical call-site of an in-scope fix), **and** the remedy
  is a production-code change. Coach-first **may change behavior**
  when the finding is a real defect — **on by default**. Prefer
  improving production code over silencing the scanner. Opt out with
  `solid-only` / `no-behavior-change` / prune-comments-only.
- Style, naming, unsupported-language diagnostics, informational
  coverage, and metric/density rules (`complexity.*`, density-gated
  `structure.*`) → stay Stage A. Generic "harder to follow / easy
  to miss an edge case" text in `why_it_matters` is still Stage A.
  Promote to Stage B only when `why_it_matters` names a specific
  incorrect result, a failing test, or a misleading output on a
  named path. Do not empty Stage A for its own sake. SOLID may
  still extract those functions later.
- `state.hidden_input_mutation`: Stage B only when an **exported**
  function mutates a caller-owned argument. A private helper that
  `.push`es onto a same-class accumulator stays Stage A; SOLID 4c
  may still return a copy.
- Stage B whose only fix changes a public contract, a schema, or
  layer policy → Ruling required, do not fix.
- Out-of-scope Stage-B paths → do not edit. List them Stage A-only
  with reason `out of scope`. The same applies to a Stage-B path whose
  scope status is `clean` or `ruling`, or that falls outside the
  invocation glob: list it under `questions` and leave it alone.

## Regression guard

Prepare's start-proof run recorded the **pre-coach check**: per
command its exit status, its failed tests by full name, its lint
inventory with multiplicity, and every other diagnostic it printed as one
identity each (`loop-protocol.md`, Green). Every Stage-B fix in every
tier must clear this guard before it is committed:

1. Orient on the test framework first (`tidy-rules.md`, Framework
   Orient). Add or extend a behavioral test in the repo's real
   container nodes that holds the corrected behavior. It fails on the
   defect and passes on the fix. Do not invent nodes. Put it where
   the repo already keeps tests for that package or directory — the
   nearest existing test file, or a new file beside it on the same
   runner. If the recorded test command cannot exercise that area at
   all, the item is Ruling required (behavior with no test), not a
   silent fix.
2. Run the smallest covering tests, then the full recorded test and
   lint commands.
3. Compare by identity with the pre-coach check:
   - an **exit status** different from the one recorded for that
     command → the fix is rejected, whatever the test and lint sets
     say. A removed symbol makes `go test ./...` exit 1 with
     `undefined: …`, no `--- FAIL: Test…` name and no lint violation;
     both identities stay empty and only the status tells you the tree
     is broken.
   - a failed test, a lint violation, or any other **diagnostic** that
     is **not** in the check set → the same rejection. So is a second
     occurrence of a violation the check set holds once: the guard
     compares the lint inventory with multiplicity, so accepting the
     repo's existing debt never licenses adding to it.
   - rejected means: repair it now, or discard the fix and its test
     (`git checkout HEAD -- <files>`; `git rm -f -- <the new test>`).
     Never commit it, never wait for 4a to "baseline" it.
   - a test that was failing in the check set and now passes because
     of the fix → accepted; remove it from the check set. An exit
     status may drop to 0 the same way; never upward.
4. Commit the fix and its holding test together. The check set after
   that commit is the guard for the next fix.

Two counterexamples this guard exists for. A coach edit clears a
signal but breaks `TestUnrelatedBehavior`; without the guard 4a would
record that failure as baseline and SOLID would be forbidden to
repair it. A coach edit deletes a symbol another file still calls;
the suite exits nonzero with a compile error and nothing the name-and-pair
sets can see, and without the exit status the guard would call it green.
With the guard both are repaired or reverted before any commit.

Record each holding test and check result under the tier's line in the
state file.

## Tier 1 — simple scan loop

No `--project-config`. This tier must run even when a project config
exists, so architecture policy never hides a simple finding.

Since v0.11 the scan discovers policy on its own (JSON consume path)
at `HEAD` — in PR mode, possibly the PR's own unreviewed policy. So
Tier 1 consumes only `signals` whose `rule_id` does not start with
`architecture.`, `diagnostics` whose `kind` does not start with
`project_` or `base_project_`, and `coverage`; it ignores every
`project_*` field and `layer_membership`. Architecture is Tier 3's,
behind the trust bind. When the scan exits 2 with
`project_config_invalid` naming a discovered policy, no flag turns
discovery off: record `coach: tier 1 skipped (discovered policy
invalid at HEAD)`, add one pending ruling naming the file, and go to
Tier 2, which reads the trusted revision on its own. Any Tier 1 exit 2
— an invalid discovered policy, or any other — also records
`withheld: coach-tier-1` in the header, so a later pass re-admits the
file-local scan (loop-protocol, Reconcile).

PR mode:

```bash
<binary> codesignal --format json --no-interactive \
  --base <resolved-base> --scope production
```

Branch mode:

```bash
<binary> codesignal --format json --no-interactive \
  --baseline --scope production
```

Also run `--scope all` when the production scan's
`summary.files_analyzed` is 0, or the user asked for all. A tests-only
PR often analyzes 0 production files; `--scope all` is how coach
sees the test files. If `--scope all` is also 0, record
`coach: no analyzable files` and continue to SOLID. In PR mode do
**not** fall through to `--baseline`. SOLID still tidies test files
that are in the git scope even when coach `--scope production`
excluded them (`coverage.excluded` reason `test_only`).

Do **not** add `--project-language` in Tier 1. File-local rules fire
on Go/TS/TSX paths without it. Since v0.11 the flag makes a discovered
TypeScript policy start the TypeScript project backend, so a host
without a supported Node exits 2 (`node_unsupported`) and the tier
loses every file-local signal. Project-language analysis is Tier 3's.

Cycle ≤5 (skip the edit/commit steps under solid-only opt-out —
classify and list only):

`resolve-binary → scan-set → classify Stage A then B → edit in-scope Stage-B paths → regression guard per fix → commit only if guarded edits landed → [push-per-cycle when licensed] → same scan-set`

Do not commit a no-op cycle, and do not commit a fix that failed the
regression guard. Prefer production-code fixes over silencing. Stop Tier 1 when Stage-B is empty after a fresh simple
scan, or 5 cycles have run, or the simple scan cannot execute. Leftover
Stage-B after the cap does not skip SOLID — list each leftover under
`questions`, keep those paths `todo` (do not mark them
`clean`), then continue. After the tier, Read `.cleanup-loop.md` and
Edit the Tier 1 coach section.

Git during this phase: same as the cleanup loop (commit only by
default, trailer; push-per-cycle under the rule above when `push` /
`open the PR` + tracking). Never `--force`. Finalize push is SKILL.md
step 6, not a coach-cycle action.

## Tier 2 — detect a trusted project config, or propose one

### Whose policy is it

Architecture policy decides what Tier 3 reports and which Stage-B
edits follow, so it may never come from the change under review. In
`mode: pr` the trusted revision is the **merge base**, not `HEAD`:
a PR can add or edit `.lousy-agents/coach.json`, `project.json`,
`README.md`, or `AGENTS.md` and
would otherwise have its own claimed rules accepted as policy before
anyone reviewed them. So:

- Read policy at `<merge-base>`, not `HEAD`, whenever a merge base
  exists. `mode: branch` on the default branch reads `HEAD`.
- A policy file inside this pass's scope list — the PR's diff touches
  it — is **never** a source of policy. Record one pending ruling
  (`policy file changed by this PR — confirm before it governs a
  scan`) and keep using the merge-base policy, or none.
- The user naming a config in the invocation is an explicit opt-in
  and is trusted at `HEAD`; say so in the state file.

### Bind the scan to the trusted blob, not its pathname

Reading policy at `<merge-base>` is necessary and not sufficient.
`--base` selects the diff comparison, not the policy revision: coach
(v0.6.0 through v0.11.0) loads the policy at `HEAD` in diff mode
(coach reads the policy blob at `HEAD`, and v0.11 discovery reads
that same revision), and `--baseline` / `--check-project` resolve
`HEAD` too.
Detecting an approved config at the merge base and then passing its
**path** to coach hands the scan whatever that path holds at `HEAD`.

So compare the bytes before the readiness probe and before Tier 3:

```bash
set -o pipefail
cfg=<detected-path>
git cat-file -e "HEAD:$cfg"   # presence first; absent is its own reason
trusted="pcfg_$(git show "<rev>:$cfg" | sha256sum | cut -d' ' -f1)"
athead="pcfg_$(git show "HEAD:$cfg" | sha256sum | cut -d' ' -f1)"
test ${#trusted} -eq 69 && test ${#athead} -eq 69   # no sha256sum → stop, never a match
```

The presence test is not redundant. A missing blob makes `git show`
write nothing, and `sha256sum` then digests the empty string into a
well-formed `pcfg_e3b0c442…`. The comparison still skips the scan,
but the recorded reason would read "differs" for a file the PR deleted.

- Equal → the blob coach will load is the trusted blob. Record the
  path, `<rev>`, and `trusted`, then proceed.
- Different, or the `HEAD` blob is absent → **skip the project scan**.
  Record `coach: project-config at HEAD differs from <rev> (<trusted>
  vs <athead>); tier 3 skipped` with one pending ruling, and go to
  SOLID. Never edit, revert, or commit the customer's policy to make
  the two agree — the policy is theirs, and a tidy pass was not asked
  to arbitrate it.
- `<rev>` is `HEAD` (branch mode, or a config the user named) → the
  two are equal by construction; record the digest anyway.

`pcfg_` plus the SHA-256 of the committed bytes is coach's own
`ConfigDigest`, so the value above is comparable to the `config_digest`
coach reports.

This gate does not read the scope list, and that is what makes it
load-bearing: a `--glob pkg/**` leaves a changed policy file out of
scope, so the ruling bullet above never fires while the swapped policy
still governs the scan.

Detection, in order, against that trusted revision `<rev>`.
`.lousy-agents/coach.json` is coach's policy path for every release
from v0.11 on, so the skill favors it: only an explicit user choice
outranks it, and whenever a file the user did not name ends up as the
policy, add one `questions` line recommending the move to
`.lousy-agents/coach.json`.

1. A path the user named in the invocation, if
   `git cat-file -e "HEAD:<path>"` succeeds: a user-named path is
   trusted at `HEAD` (Whose policy is it).
2. `git cat-file -e "<rev>:.lousy-agents/coach.json"` — coach's
   canonical path. When it exists it is the policy, even if
   a root `project.json` disagrees (coach's own precedence); when it is
   invalid, coach does not fall back, so neither does this skill:
   record the reason and skip Tier 3.
3. A root `project.json` at `<rev>` — legacy, and only when it is a
   coach policy. Coach reads a root `project.json` only when it is
   schema-valid, so check the trusted blob, never a probe of the
   worktree file (in PR mode that is the PR's version):

   ```bash
   command -v jq >/dev/null || echo "policy detection: no JSON tool"
   git show "<rev>:project.json" |
     jq -e 'type=="object" and has("schema_version") and has("roots")'
   ```

   Check for `jq` first: a missing `jq` exits 127, which is not a
   verdict on the file. Without it, record `policy detection: no JSON
   tool`, do not treat the root `project.json` as the policy, propose
   no replacement, and skip Tier 3 this pass. Never fall back to "the
   file exists, so it is the policy". A foreign file,
   such as an Nx `project.json`, fails it: record `project.json
   present, not a coach policy` and continue as if it did not exist.
   A file that passes is the policy, and coach reads it with
   `project_config_deprecated`. This step owns the migration
   `questions` line: `mkdir -p .lousy-agents && git mv project.json
   .lousy-agents/coach.json`. Never perform it; the policy is theirs.
4. A repository-relative path recorded as the coach
   `--project-config` file in project instructions / CI / README **at
   `<rev>`**, if that blob exists there. It is consulted only when
   neither `.lousy-agents/coach.json` nor a coach-policy `project.json`
   exists.

If a trusted committed config exists and its blob bind passed,
record the path, the revision it came from, and the digest; run the
TS readiness probe when TS is in scope; go to Tier 3.

If none exists, skip the readiness probe and propose one at
`.lousy-agents/coach.json`; never write or commit it. Do not overwrite an existing worktree
`.lousy-agents/coach.json` or `project.json`.

### Readiness probe (TypeScript in scope)

Coach scan-set item (readiness probe): informational readiness only —
not a Stage A/B scan-fix loop, not a gate, not Tier 3. Run only when
Tier 2 detected a trusted config and the blob bind passed. The bind
applies to this probe, not only to Tier 3 scans. No trusted config →
skip the probe entirely (do not default to HEAD).

```bash
<binary> codesignal --format json \
  --baseline --check-project --project-language typescript \
  --project-config <detected-path>
```

Pass `--project-config <detected-path>` whenever those two hold. An
explicit path wins and coach consults no other file. Without it coach
discovers `.lousy-agents/coach.json` at `HEAD` — possibly this PR's
untrusted policy, and not the path Tier 3 passes.

`--check-project` requires `--baseline` and cannot combine with
`--base`. Exit 0 ≠ clean. Payload is `status` / `checks` / `gaps` /
`next_actions` — not a scan, no `signals`. Do not classify Stage
A/B from it. Do not execute `next_actions.prepare_compiler`. Record
the payload in the state file, naming the config path it checked. A
compiler mismatch does not block the proposal; it may still skip Tier 3
(scan exit 2).

### Propose when policy is missing

Coach prints `--suggest-project-config` as a **candidate for human
review** and never auto-applies it. This skill does not launder that
boundary by committing it. A tidy pass was not asked to establish an
architecture policy, and a committed config silently becomes trusted
input to every later scan.

So the candidate is built, shown, and left uncommitted: write it
under the scratch directory, put its full content and the exact
`--project-config` invocation under `questions`, and add one pending
ruling (`project config proposed — land it on the base branch, or
name its path in the invocation, to enable the project scan`). Tier 3
then does not run this pass, because coach
reads committed Git objects and there is nothing committed to read.
That is the correct outcome, not a failure: record `coach:
project-config proposed, not committed; tier 3 skipped` and go to
SOLID. Never write or commit a policy file into the repository.

1. **Roots.** If tracked `go.mod` files exist, run the Go suggest in a
   detached scratch worktree. It writes `.lousy-agents/coach.json`
   into the worktree it runs in, `--output` accepts only a
   repository-relative path, and the customer's tree must stay clean.
   Run the whole block in one Bash call; its variables do not survive
   between calls:

   ```bash
   scratch="$(mktemp -d)"; wt="$scratch/wt"
   trap 'git worktree remove --force "$wt"' EXIT
   git -c core.hooksPath=/dev/null worktree add --quiet --detach "$wt" "<rev>"
   (cd "$wt" && <binary> codesignal --baseline --suggest-project-config) \
     > "$scratch/candidate.json" && cat "$scratch/candidate.json"
   ```

   `<rev>` is the trusted revision (merge base in PR mode), so a policy
   file the PR adds never becomes the candidate. `core.hooksPath`
   keeps the repository's `post-checkout` hook from running. A
   detached worktree creates no branch, and the `trap` removes it on
   every exit path. The candidate is stdout, on v0.11 and older alike
   (diagnostics go to stderr), printed only when the command exited 0.
   `$scratch` is a scratch directory like the acquisition paths' and
   may hold the candidate until its content is under `questions`; then
   `rm -rf "$scratch"`. Otherwise, or when the repo is TS-only,
   compose `"schema_version": "1"` with
   `"roots": ["."]` when a root `package.json` / `tsconfig.json` /
   `go.mod` exists, else every directory that contains one of those
   files (from `git ls-files`).
2. **Policy evidence.** Coach does not guess an architecture
   (`--suggest-project-config` never invents `layers` or
   `forbidden_imports`), and neither does this skill: a directory
   name says where code lives, not what it may import. Layers and
   forbidden pairs come only from a statement of intended
   dependencies. Grep for one: project instructions and architecture
   docs (`AGENTS.md`, `CLAUDE.md`, `README`, `docs/architecture`,
   ADRs) that name layers and what must not import what; a
   dependency lint the repo already enforces (`depguard` /
   `gomodguard` in `.golangci.yml`, `dependency-cruiser`,
   `eslint-plugin-boundaries`, `import/no-restricted-paths`); a
   `project.json` earlier in history; the user's own words in the
   invocation. Map each stated layer to the prefixes that already
   hold tracked `.go` / `.ts` / `.tsx` (`git ls-files '<prefix>/*'`
   non-empty; skip a layer with no source) and each stated
   prohibition to a `forbidden_imports` pair whose both ends are
   mapped layers. Do not add `required_layer`. Keep prefixes
   disjoint: coach rejects a policy in which one layer's
   prefix contains another's (exit 2). Do not add
   `exclude_type_only_edges`; it is the customer's choice. Evidence read from a
   file this PR's diff touches does not count (see Whose policy is it);
   it is a pending ruling instead.
3. **No evidence → roots only.** The candidate carries
   `schema_version` and `roots` and nothing else. Record the prefixes
   you observed as unconfirmed under `questions` (`architecture
   candidate: src/handlers and src/db exist; no stated rule between
   them`) and one pending ruling `architecture policy — confirm
   layers and forbidden imports`. Two repos with the same directory
   names and different documented rules must end with different
   candidates; a repo that states no rule gets no prohibition.
4. Show the candidate (2-space indent, trailing newline) under
   `questions` as the content of `.lousy-agents/coach.json`. Record
   `coach: project-config proposed, not committed;
   tier 3 skipped`. If the candidate could not be built, record
   `coach: project-config absent; tier 3 skipped (<reason>)`.
   Either way, go to SOLID.

## Tier 3 — project scan loop

Only if Tier 2 found a trusted committed config carrying at least one
**enforceable rule**: `forbidden_imports`, `layers`,
`required_layer`, or any other rule key the config schema and
`--help` support at the pinned version. `required_layer` alone is
enforceable — coach emits `architecture.layer_bypass` from it with no
`forbidden_imports` present at all, which is exactly the shape of its
own acceptance fixture, so gating on forbidden pairs would skip a
real finding.

Reserve the skip for a config that carries roots and no rule of any
kind: record `coach: project-config roots-only; tier 3 skipped (no
stated policy)` and go to SOLID.

PR mode:

```bash
<binary> codesignal --format json --no-interactive \
  --base <resolved-base> --scope production --project-config <detected-path> \
  [--project-language typescript]
```

Branch mode: same flags with `--baseline` instead of `--base`. Do
not combine `--base` and `--baseline`.

Run one project scan per language present under the policy `roots`,
whatever languages the PR touches: Go, without `--project-language`,
when a tracked `go.mod` sits under a root; TypeScript, with
`--project-language typescript` (when `--help` lists it), when a
tracked `tsconfig.json` does. A coach config names no language, and
each backend sees only its own language, so one scan of a mixed Go+TS
repository misses the other half and still reads complete. Every scan
gets the digest confirmation, classification, and incompleteness rules
below, and each cycle reruns all of them. Tier 3 is clean only when
every scan is clean and complete. A scan that exits 2 while another
ran (`node_unsupported` for the TypeScript one, say) leaves Tier 3
incomplete, not skipped. Neither file under a root → record `tier 3:
no project language under the policy roots` as incomplete.

Invalid config (not at the analyzed revision, not JSON, schema
failure, overlapping layer prefixes): coach exits 2 with
`project_config_invalid` and writes no report. Record the reason and
continue to SOLID. A PR that deletes the policy file lands here,
because the read is at `HEAD`.

Every entry in `project_changes` carries `config_digest`. Each one
must equal the `trusted` digest recorded in Tier 2; any other value
means the scan ran under a policy this pass did not trust — stop and
report it under `questions`. A TypeScript report also carries
`project_provenance.config_digest` even with no finding; it must
equal `trusted` too. Treat both as confirmation, not as the gate: a Go
scan that produces no finding emits no digest anywhere
(`project_summary` carries counts only), so the blob bind before the
scan is what covers that case.

Cycle ≤5, same Stage A/B bar, same regression guard, and same commit
rule as Tier 1 (commit only if guarded Stage-B edits landed).
Architecture signals (`architecture.layer_violation`,
`architecture.layer_bypass`, `schema_version` `"2"`) enter the same
classification:

- Stage B if the finding could cause incorrect behavior, a test
  failure, or a misleading result, the file is in scope, and the
  remedy is a production-code change (not a config rewrite).
- Ruling required if the remedy would change a public contract, a
  schema, or layer policy itself.
- Stage A otherwise.
- A TypeScript finding whose `machine_evidence` carries
  `removed_if_set: exclude_type_only_edges` exists only because of
  type-only imports (evaluated by default). It is still a
  finding: remedy it in production (move the type inward, or invert
  the dependency) under the bar above. Setting
  `exclude_type_only_edges` is a policy edit → Ruling required.

Apply the Incomplete is not clean rules after every project scan: no
Stage-B finding plus any incompleteness item is recorded `tier 3: no
findings; incomplete (<counts>)`, never clean.

Prefer improving production code over editing or silencing the
policy file. After onboard, do not rewrite layers to silence
a finding.

Stop Tier 3 when Stage-B is empty after a fresh project scan, or 5
cycles have run, or the project scan cannot execute. Stopping ends
Tier 3 only: go to SOLID, and list anything left under `questions`.
Leftover Stage-B after the cap stays `todo` as in Tier 1. After the tier,
Read `.cleanup-loop.md` and Edit the Tier 3 coach section.

## After all coach tiers

Record or reuse the SOLID baseline exactly as `loop-protocol.md`
(Green) says; the three record conditions live there, not here. Then
run SOLID. Do not re-run coach after SOLID in the same invocation.

## State file per tier

Read `.cleanup-loop.md`, then Edit the matching section. Prepare
created the file; never Write over it here.

- Pre-coach check: failed tests, lint inventory (from Prepare).
- Tier 1: commands, Stage-A count, each Stage-B item, holding tests,
  guard results, SHAs.
- Tier 2: detected path (`canonical` or `legacy`), `proposed <path>`,
  or `absent`;
  `--check-project` payload / skip reason.
- Tier 3: ran / skipped / invalid, commands, Stage-A/B, holding
  tests, guard results, SHAs, `layer_membership` counts, and the
  incompleteness counts.

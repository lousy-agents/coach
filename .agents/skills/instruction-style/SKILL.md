---
name: instruction-style
description: Author or revise durable instruction prose so priority, rationale, and scope survive. Use when asked to clean up, tighten, harden, or standardize AGENTS.md, CLAUDE.md, Copilot, OpenCode, or Codex files, settle a closed modal set, place rules across a hybrid harness, check drift from code and CI, or review an instruction diff for lost meaning. Do NOT use for a one-off task prompt, to lint a SKILL.md, or to audit a feature spec.
argument-hint: "Path or glob of instruction files to revise (or paste the prose); say 'apply' to write files instead of proposing a diff"
allowed-tools: Read, Grep, Glob, Bash(npx --yes @lousy-agents/cli doctor*), Bash(npx @lousy-agents/cli doctor*), Bash(rm ./doctor-inventory.json), Bash(git diff*), Bash(git log*), Bash(git show*)
compatibility: Discovery via `npx @lousy-agents/cli doctor` needs Node.js and network access on first run. Doctor is optional — fallback glob discovery is first-class and carries no prerequisite.
---

# Instruction Style

Write and revise agent-facing prose so an agent reads the right priority,
understands why each constraint exists, and does not apply a rule where it does
not fit.

Misreading is not the failure mode you are preventing. Agents parse English
fine. Instruction files fail three other ways: the agent **misprioritizes**
(two rules apply and nothing says which wins), it **routes around an opaque
rule** (the rule's purpose is invisible, so a clever workaround looks
compliant), and it **over-applies** (a rule written for one case gets used
everywhere). Optimize against those three.

## When to apply

- Shared instruction cores and their adapters: `AGENTS.md`, `CLAUDE.md`,
  `.github/copilot-instructions.md`, OpenCode `instructions` entries.
- Path-scoped instruction files: `.github/instructions/**/*.instructions.md`,
  `.claude/rules/**`.
- Subagent, command, and agent files across harnesses, plus the routing
  `description` of a skill.
- Specs, plans, and architecture notes the agent authors — only when the ask is
  priority, rationale, or obligation wording. Implementability is a spec audit,
  not this skill.
- A diff that rewrote any of the above, when the question is what meaning was lost.

Scope is durable instruction prose. A skill or command **procedure body** — the
numbered steps an agent executes — is out of scope: leave its voice alone. A
skill's routing `description` is in scope; the rest of its frontmatter is not.

## When not to apply

- Do not touch source code, identifiers, command names, file paths, vendored or
  lockfile-owned text, or strings that tests assert. Changing those breaks
  builds rather than improving prose.
- Do not rewrite **load-bearing frontmatter** — `applyTo`, `paths`,
  `excludeAgent`, a Copilot agent file's `tools` or `model` — while restyling,
  because those keys decide whether the file loads at all. Creating a new
  path-scoped file may add them; that is a placement task, and only a placement
  request authorizes it.
- Do not override repository instructions, safety rules, or test-locked wording,
  because this contract governs how prose reads and never what it is allowed to
  require.
- Do not edit before the locked-string inventory in Work step 2 exists. Tests,
  hooks, and plugin loaders match on instruction text verbatim.

## Modality: one closed set, declared once

- One word for mandatory, one for recommended, and plain fact in the present
  indicative with no modal at all — and no other modal carrying obligation
  anywhere in the file. Which two words is a house-style choice, not a
  correctness one; the failure being fixed is a file where `must`, `shall`,
  `should`, "make sure", and a bare imperative all appear and nothing says
  which outranks which. `will` is not the plain-fact word: in an instruction
  file it reads as future tense, so `parses purely in Go` becoming `will parse
  in Go` turns a description of the code into a roadmap claim.
- **Detect the file's existing vocabulary and stay inside it.** If a file
  already carries obligation on `MUST`, keep `MUST` and make it consistent.
  Converting a Copilot-owned file, or a shared canonical file that already uses
  `MUST`, over to `shall` fights the house style it lives in and churns a diff
  for no reader benefit. Pick the incumbent, then close the set around it. A
  bare imperative (`Return 503 ...`) on a shared core is a valid incumbent
  form, not an out-of-set modal to eliminate.
- Declare the chosen set in the run report, not in the file. Write a legend
  into the file only when one already exists there or the user asked for one:
  a banner on an always-loaded file costs every session and churns the diff.
- Delete hedges attached to real requirements (`try to`, `if possible`,
  `ideally`, `where practical`). An agent reads them literally as permission to
  under-deliver.
- Do not add emphasis (`CRITICAL`, `ALWAYS`, caps, bold) to carry priority.
  Priority comes from the modal verb. When several rules are marked critical,
  the marker stops carrying information.
- Before removing emphasis, check whether it is structural. Tests, hooks, and
  loaders slice instruction files on their formatting — a heading, a numbered
  step marker, the bold span that follows one — so stripping it breaks the
  build. Emphasis a parser depends on is not emphasis carrying priority. The
  lock-surface greps are in `./references/discovery.md`.

## Every constraint carries its reason

- State the reason in the same sentence as the rule, or in the next sentence.
  A rule whose purpose is visible gets applied correctly in cases it did not
  anticipate. A bare rule gets routed around.
- Write `Return 503 so a partial read never reads as an authorized empty
  result`, not `Return 503`.
- If you cannot state why a rule exists, report it. An unjustifiable rule is a
  removal candidate, not a rule to reformat.
- A reason you infer rather than find in the file, the code, or the history is
  a guess. In a proposed diff, list each inferred reason in the report so the
  reviewer can strike it; under apply, put it in the report as a question and
  leave the rule bare. An invented purpose that is wrong gets routed on, which
  is worse than a bare rule — one measured pass wrote nine inferred reasons
  into a subagent prompt and grew it by a third.
- **Sweep lists and table cells, not only flowing prose.** A rule living in a
  bullet or a table cell is still a rule, and a pass that rewrites paragraphs
  and skips lists leaves the prohibitions untouched — which is the half where a
  missing reason costs most. Measured on a real pass over an unfamiliar
  repository, every residual out-of-set modal and every bare prohibition sat in
  a bullet list or a severity table, none in prose.

## Form: match the formalism to the content

Reference content (contracts, invariants, faults, ordered procedures) gets
structure; use EARS on triggers and faults. Behavioral content — judgment,
taste, role, push-back — stays prose. Read `./references/form.md` before
converting any sentence: it holds the templates, the actor-slot rules, and
the four conversions to refuse.

## Structure

- Keep related statements grouped under a shared parent. Do not flatten nested
  structure into sibling bullets: grouping is what tells the reader which facts
  belong to which subject.
- Use bullets and tables for reference data. Use prose for behavior. Bullets
  sever a rule from its reason, and prompt format bleeds into output format —
  a file written as clipped fragments trains clipped fragmentary output.
- Make a requirement individually checkable — one requirement, not three fused
  into a clause a reviewer can only half-satisfy. This is not a one-clause
  sentence rule: keep the causal and contrastive subordinates (`so that`, `even
  though`, `rather than`) that carry the reasoning, and never split a sentence
  merely to shorten it.
- Use one term for one concept throughout a file. Two names for one thing makes
  the reader look for a distinction that is not there.
- Keep the file's existing wrapping convention. Rewrapping a file makes every
  paragraph show as changed, and an unreviewable diff costs more than the line
  count it saves. Human review of the diff is the last defense against the
  rationale loss the keep list guards, so protect it.

## Address

Classify the file before choosing an address. Getting this wrong leaks one
harness's voice into every other harness that loads the same bytes.

- **Shared canonical core** — `AGENTS.md` and any file imported into several
  harnesses. Use the imperative without "you", or name the actor. Never second
  person: several harnesses load this file, and one of them inlines it into a
  system prompt where "you" would address whoever happens to be reading.
- **Single-harness actor file** — a `CLAUDE.md` body below its import, a Copilot
  `*.agent.md`, a subagent definition. Second person, because the file is that
  actor's system prompt and is read in that register.
- **After a hard import A → B**, put shared rules in B and keep A for
  harness-private routing. Do not state a requirement at both ends: the reader
  sees the concatenation, and a rule that appears twice in two voices reads as
  two rules.
- Do not mix registers inside one file, because a reader addressed as "you"
  reads a later third-person rule as being about somebody else.

## What not to remove

A revision pass deletes the highest-value words first. The keep list — reason
clauses, intent markers, stance words, manner adverbs, routing discriminators,
defining clauses, examples, live prohibitions — and the rule that it outranks
any size figure live in `./references/keep-list.md`. Preserve them; restore
them if a prior pass removed them.

## Size and placement

- **The only size rule is the always-loaded set budget.** Add up everything the
  harness loads unconditionally — the core, every adapter, every unscoped rules
  file — and judge that one number. Per-file word and line figures are advisory
  and are not a target to hit.
- One harness enforces a real ceiling rather than a preference: Codex truncates
  the concatenated AGENTS.md chain at `project_doc_max_bytes`, 32 KiB by
  default. See `./references/harness-load.md`.
- Growth is cheap in a path-scoped file and expensive in an always-loaded one,
  so spend it accordingly. Where the harness supports scoping — Copilot's
  `applyTo`, Claude Code's `.claude/rules/` `paths` — a scoped file carries long
  reasoning at no cost to a session that never touches those paths. A rewrite
  that doubles a scoped file while holding the always-loaded set flat is the
  placement working, not bloat.
- A harness-private scoped file is a **projection**, not a move. Every harness
  outside that mechanism loses the rationale entirely. Record which harnesses
  no longer see it, and say so in your report; a rule that silently vanishes for
  two of four harnesses is worse than a long shared file.
- Move content an agent can derive from the codebase — directory layouts,
  dependency lists, architecture overviews, benchmark measurements — into docs
  and leave a one-line pointer. Keep pitfalls, rationale, and conventions that
  differ from tool defaults.
- Move documentation about one command into that command's own file.
- Place a rule on the **intersection** of the load paths of the harnesses that
  need it. Triggers differ per harness and are version-sensitive: read
  `./references/harness-load.md` before choosing placement.

## Discovering the prompt surface

Enumerate the surface with a command where you can, because a glob finds the
harnesses you thought of and misses the one you forgot. Fallback globbing is
first-class and carries no prerequisite.

Read `./references/discovery.md` before this step. It holds the doctor
invocation and filter, the full fallback glob list across all four harnesses,
and the lock-surface greps to run before you strip emphasis or rewrap.

Three rules that hold regardless of method:

- **The inventory is the surface, not the scope.** Most records in any
  repository are skills, and some are owned upstream by a lockfile. Intersect
  the surface with what the invocation and the repository allow.
- **Discovery is topology, not evaluation.** An empty `findings[]` from any
  tool is not a clean bill of health, and it does not excuse the checks below.
- **A doctor run that was denied or failed is not discovery.** Say so in the
  report and fall back to the globs; never report the surface as enumerated
  on a command that did not execute.

## Verify the claims, not just the prose

Instruction files rot factually as the code ships, and nothing re-checks them by
default. An inaccurate instruction costs more than an inelegant one: an agent
following accurate but clumsy prose outperforms one following polished prose
that is false. Check every checkable claim against what it describes, and
correct what has drifted.

- **Counts and inventories** — job names, leaf counts, task lists, file counts.
  Extract ground truth from the source — the CI workflow, the task-runner
  config, the directory listing — and compare, rather than reading the prose
  for plausibility.
- **Coverage claims** — when a file says a task runs "everything except X",
  resolve that task's actual closure and confirm X is the only gap.
- **Scoping globs that match nothing.** A path-scoped instruction file is
  dormant if its glob matches no file, and it fails silently: the content looks
  maintained and never loads. Expand every `applyTo` and `paths` pattern against
  the repository and confirm a non-empty match. One real case: `**/spec.md`
  matched zero files while 2,000 words of spec guidance sat unused.
- **Dangling citations, in both directions.** Grep the codebase for references
  to the file you are editing. Code citing a policy the file does not state is
  a gap in the file, not in the code. A rule that nothing references and
  nothing enforces is a removal candidate.
- **Paths and links** — take these from `doctor`'s `edges[]`, where
  `malformed: true` marks a reference that does not resolve. Resolving paths by
  hand is error-prone: a relative link resolves from its containing file's
  directory, and checking from the repository root reports every link in a
  relocated page as dead.

Report drift you find even when it sits outside the prose you were asked to
revise. Finding it is cheap here and expensive later.

## Work

1. Read the repository's agent instructions and the target files before editing.
2. **Inventory what is locked, before the first edit.** Search tests, hooks, and
   loaders for the headings, step markers, asserted phrases, and load-bearing
   frontmatter keys they match on. The patterns are in `./references/discovery.md`.
   List them up front; a grep before each deletion catches the phrase you
   thought to check and misses the one you did not.
3. **Discover the prompt surface.** See `./references/discovery.md` for both
   methods. Intersect the surface with the scope: if the invocation names files
   or a glob, that is the scope; otherwise ask before touching a file the
   repository owns from upstream. Do not invent files.
4. **Classify each target file** as a shared canonical core, a single-harness
   actor file, or a path-scoped projection, because address and placement both
   depend on it. Consult `./references/harness-load.md` before moving a rule.
5. Run the accuracy pass above. Fix drift in the same change.
6. Keep existing meaning, authority, and names. Change wording and structure.
7. **Propose a unified diff. Do not write files unless the invocation says to.**
   "Clean it up", "tighten this", "harden these files", and "fix these files"
   all mean propose. Only an explicit `apply`, "write the files", or "commit
   these edits" authorizes writing to disk.

   Opening a pull request is a proposal. Committing to a shared default branch
   is not, and is never authorized by this skill. Committing to the agent's own
   working branch is allowed only under an explicit apply phrase.
8. Report what you moved, any rule you could not attach a reason to, any drift
   you found but did not fix, and which harnesses lose visibility of a rule you
   projected into a scoped file.

## Presenting two or more options

When the task ends in a recommendation rather than a diff, use the option-report
shape in `./references/verification.md`. It is a report to a human: never write it
into an instruction file, and never carry its voice there.

## Verification

- Obligation inside each file is carried by that file's declared set and no
  other modal. No hedges on requirements, no emphasis substituting for
  priority. Sweep each changed file for obligation words **outside its
  declared triple** — the recipes, one per house style, are in
  `./references/verification.md`. Never sweep case-insensitively for the
  declared word itself: that flags every legal `MUST` in a `MUST`-house file,
  and "closing the set" on those hits deletes valid obligations. Read each
  hit, because the count alone over-reports: only a **deontic** use — an
  obligation on the reader — violates the set. A **descriptive** use, such as
  "a list that has to be updated in repository settings", and a **quoted**
  use, such as an RFC 2119 key-words paragraph, are not modals to convert.
- Every prohibition and every non-obvious constraint states its reason.
- EARS forms appear on trigger and fault conditions, not on dispositions.
- Reason clauses, intent markers, routing discriminators, and defining clauses
  survive the edit. **Count across the whole post-edit surface**, source plus
  anything relocated, or every phrase that moved reads as a loss. A reason
  reworded is preserved — judge the clause, not the token.
- Every claim checked in the accuracy pass matches its source.
- Identifiers, paths, frontmatter, vendored text, and test-locked strings
  survive. An identifier that moved inside a different backtick group still
  survives.
- Every count you report is accompanied by the command that produced it, and a
  before-and-after comparison runs the identical command on both revisions.
  `./references/verification.md` explains why, and how a detector manufactures a
  defect when it does not. Its before-and-after table also carries a bullets
  column: a list whose bullet count multiplies while its parents stay the same
  has been flattened, and the reader has lost which facts belong to which
  subject.
- Run the verification once. Do not add further review passes.

### A negative result is a hypothesis, not a finding

Your instrument is likelier to be wrong than the artifact. Before reporting
anything as removed, dead, missing, or miscounted, confirm the absence a second
time by a different means. Report a defect only when two methods agree.
`./references/verification.md` lists the failure modes that have already produced
false defects against correct files, and why the list cannot be complete.

## Output

- The requested artifact, or a proposed diff. The contract binds the
  instruction diff; the report around it stays short and plain, not
  EARS-formed and not restyled to the set.
- A short list of what you moved, any rule you could not justify, and any
  factual drift you found.
- The command beside every number you report, so a reader can rerun it.
- The before-and-after table from `./references/verification.md` for every
  changed file — words, bullets, reason clauses, intent markers, manner
  adverbs, and each modal — produced by one command on both revisions. A drop
  in a keep-list column is a finding to explain in the report, not a number to
  leave in the table: a pass that stripped every `deliberately` from a core and
  reported no count was accepted as a style change.
- What you could not verify, named plainly. A check you could not run is not a
  check that passed.
- `./doctor-inventory.json` deleted, if discovery wrote it. Nothing ignores it
  by default, and a run that leaves it behind hands the user an untracked file
  to explain.
- No persona, no process narration.

## What this contract deliberately omits

STE100 sentence caps, qualifier bans, and an RFC 2119 `MUST` mandate are
omitted on purpose. The reasons — and why EARS stays scoped to triggers and
faults — are in `./references/omissions.md`. Do not restore them as an oversight.

## Applying this contract to itself

A skill's routing `description` is in scope. **This procedure body is not.** Do
not convert its steps to a modal set or an EARS form, and do not strip its
structural emphasis: it is a procedure an agent executes, not durable
instruction prose an agent is governed by.

# Phase 5 — Task Decomposition into Child Issues

> Load from `SKILL.md` Phase 5 after the refinement loop has exited with Tasks scored `present`.
> Child-issue body anatomy lives in [`epic-structure.md`](./epic-structure.md). Manifest table
> format lives there too (`Issue Graph Manifest Anatomy`). Native-edge and label policy details
> also appear in [`github-surface.md`](./github-surface.md); this file is the ordered procedure.

## Preconditions

- Phase 4 loop exited with Tasks `present`.
- Phase 1b has bound read/write paths and recorded: native sub-issues, native blocking edges, labels.

## Child body

Each Task becomes one child issue whose body carries the six-part anatomy: **Objective, Context,
Affected files, Requirements, Verification, Done when.** Checkboxes unchecked. See
`epic-structure.md` for the template.

## Collision check (before creating anything)

Compare every proposed child title first against the epic's *existing children* — a true collision —
then against the titles of the repository's other open and closed issues, which are only
*candidates*: a repo-wide match with no link back to this epic is not evidence the graph is
populated, only that some other issue shares a title (generic titles like "Add tests" make this
plausible).

- No matches anywhere → create every proposed child.
- All proposed titles match existing children of this epic → create nothing; the graph is already
  populated.
- Some titles match existing children and some don't, or a title matches an unrelated repo issue →
  **stop and ask** (which children to create; whether to link, rename, or accept the collision).
  Never choose for the user, and never silently create a duplicate title — GitHub issues cannot be
  deleted, so a duplicate is manual cleanup for a human. See the Failure and Degradation Summary in
  `github-surface.md` for this ask's automation-mode conversion.

## Creation path (order of preference)

1. `plan-to-graph` is available, the surface supports native hierarchy, **and the bound write path
   is the authenticated `gh` CLI** → delegate creation and dependency wiring to it. `plan-to-graph`
   is `gh`-CLI-only; delegating while a different surface is bound would mix surfaces mid-run — the
   exact thing Phase 1b forbids — so skip delegation whenever the bound path is not `gh`.
2. Native hierarchy supported, and either `plan-to-graph` is unavailable or the bound write path is
   not `gh` → `create_child_issue` per task, linked to the epic. **Known v1 limitation:** this path
   establishes the parent link but not native dependency-edge wiring (`blocked ← blocker`), even when
   Phase 1b recorded blocking-edge support as yes — native edges are applied only via path 1
   (`plan-to-graph`). Record dependencies as `Depends on: <title>` text in the child body instead,
   and always disclose the text-only gap in `### Degradations`.
3. Native hierarchy unsupported → create standalone issues, each opening with a
   `Parent: owner/repo#N` line; add a task list to the epic's Tasks section linking each child; and
   **disclose the degradation explicitly in the closing comment**. Do not emulate hierarchy with
   labels.

## Label every child that exists with `refined`

After each creation path finishes (path 1 returns from `plan-to-graph`, path 2/`create_child_issue`
succeeds, or path 3 creates a standalone issue) — and again for any existing child the collision
check skipped that does not already carry `refined` or the `ready-for-implementation` alias —
`set_labels` to add `refined`.

Do not key the label step on the `create_child_issue` abstract op alone: path 1 never calls that op,
and a re-run on an epic decomposed before this labeling rule must still make legacy children visible
to a `refined`-filtering dispatcher. Detect labels on existing children from the hierarchy/list
payload when it includes them; otherwise `read_issue` that child before deciding. Newly created
children are unlabeled until this step — always apply `refined` after create.

A child's body is already implementation-ready by construction (Phase 5 only runs once Tasks scored
`present`, and every child carries the full six-part anatomy), so it does not pass through
`needs-refine` → `refining` first. Apply the same missing/uncreatable-label skip-and-disclose rule
as any other label — a label the run could not apply never aborts a run, but disclose the skip in
the `### Degradations` section of the closing comment (Phase 6).

## Cap

**Cap child creation at 12 per run.** If the epic has more than 12 tasks, create the first 12 in
dependency order, then stop and ask before creating the rest. Report the remaining task titles.

## Collapse Tasks + write Issue Graph Manifest

**After children exist, collapse the epic's Tasks section.** One `update_issue_body` replaces the
inline six-part detail with a link list to the children plus a one-line note that per-task detail
lives in each child. This is the shape the reference epic uses, and it is what keeps a multi-task
epic inside the body-size limit. Tasks still scores `present` under the Phase 2 rubric, because each
entry links a child that carries the anatomy. Any task with no child — capped, declined, or already
existing — keeps its inline detail so no requirement is lost.

**In the same write, add or update the `## Issue Graph Manifest` section.** A coding agent that
later picks up this epic often cannot query GitHub's hierarchy or blocking-relationship data
directly. The manifest makes the graph readable from the epic body alone. Prefer data this phase
already has (titles, numbers, and `Depends on` text from children just created). When the graph was
already populated — collision check created nothing, or this is a re-run repairing a stale
manifest — use the child list from the hierarchy read Phase 5 already did for the collision check,
and for any child not created this run resolve **Depends on** from that child's body (`Depends on:`
line) or from native blocking edges if the bound read path exposes them. Do not skip the manifest
merely because this run created zero children. See `epic-structure.md` for the table format.

The Phase 6 closing comment carries the same membership in a stricter, machine-parseable form
(`### Child issues created`) as a **full current-graph snapshot** (not this-run-only) and is the
authoritative snapshot for automation that reads comments instead of the body — a dispatcher should
prefer the **most recent** closing comment when both exist, because the epic body can be
hand-edited by a human afterward and the comment cannot. State this precedence in the manifest
section itself with a one-line pointer.

Skip the manifest section entirely only when the epic still has **no** children at all — an epic
whose Tasks section is still fully inline detail and has never linked a child has no graph to
describe yet.

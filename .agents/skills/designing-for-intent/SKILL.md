---
name: designing-for-intent
description: Advocate for the user's intent map — desired outcome, constraints, and delegation boundary — across a journey, especially onboarding and consent moments, rather than for the screen or user experience sitting in front of you. Use to review a UX flow, agentic handoff point, or delegation-boundary decision for agency risks and articulation barriers before implementation begins. Not for product-quality verdicts or system design.
argument-hint: "Artifact to analyze (screen, flow, PRD excerpt, onboarding/consent step, CLI report); optional depth mode (strategic/tactical) — inferred if omitted"
allowed-tools: Read, Grep, Glob, Agent
---

# Designing for Intent

## Overview

An **intent** is not a request. It is the desired outcome the person is trying
to reach, the constraints they are operating under (including ones they never
say out loud — a deadline, an attention budget, a boss looking over their
shoulder), and the **delegation boundary**: the line between what they intend
to keep deciding themselves and what they are willing to hand off to a
feature, a workflow, or an agent.

A prompt, a click, a form field, a one-line issue title — these are rough
externalizations of intent, produced under time pressure with an imperfect
vocabulary for what the person actually wants. Treat the literal request as
evidence of intent, never as a finished specification of it. The gap between
what someone typed and what they meant is exactly the terrain this skill
works in.

**Advocate for the intent, not the screen.** A screen, a flow, an API
response, or a CLI report is only a means of carrying intent across a
handoff — from human to system, from system back to human, or from one agent
to another. When a design decision would make the screen prettier but the
intent less legible, less controllable, or more likely to be inferred wrong,
side with the intent.

### Depth modes

This skill runs in one of two depth modes:

- **Strategic** — a full journey, cross-surface: onboarding, a multi-step
  workflow, or a sequence of agent handoffs spanning more than one screen or
  session.
- **Tactical** — a single screen, flow, or decision point in isolation.

State which mode is in use in the first line of the response, inferred from
the artifact or the request (a PRD excerpt or a multi-step journey map reads
as strategic; a single screen mock, form, or CLI report reads as tactical).
Never stop the run to ask which mode applies — infer it and say so; the
verdict is more valuable delivered promptly under a stated assumption than
withheld pending clarification.

## When to Use / Do NOT Use

### When to Use

- Reviewing a screen, flow, journey, or report for whether it faithfully
  carries the user's intent, or quietly substitutes a system's convenient
  interpretation of it.
- Reviewing an onboarding or consent moment (an install step, a permission
  grant, a data-sharing choice) for whether the person understands and
  controls what they are agreeing to.
- Reviewing a point where a feature or agent proposes to act on a user's
  behalf, to check the delegation boundary, the confidence-to-response
  policy, and the undo path.
- Auditing a multi-agent handoff for coherence: does the user's intent survive
  the trip from one surface or agent to the next, or does it drift?

### Do NOT Use

This skill never writes code and never proposes metrics. Hand off instead:

- **Implementation work** (turning an approved design into code, wiring an
  API, editing `pkg/`, `cmd/`, `internal/`, or `js/`) — that is not this
  skill's job at any point in the procedure. Hand it to the implementing
  agent (e.g. the `/implement-issue` flow) once a design decision has
  actually been made — see Procedure step 8.
- **`product-sme`** — for release-readiness, adoption risk, or "is this
  useful/lovable/ready" verdicts. This skill only feeds `product-sme` the
  evidence question (see Procedure, step 7); it never answers it.
- **`product-quality-evaluation`** — the skill that delegates to `product-sme`
  for a customer-centered go/no-go assessment. If the request is "should we
  ship this," that is `product-quality-evaluation`, not this skill.
- **`system-design-expert`** — for infrastructure, security model, or
  platform-foundation questions raised by a design (e.g. "should this
  delegation boundary be enforced server-side or client-side"). This skill
  names the question; `system-design-expert` answers the architecture.

## Procedure

### 1. Ground in evidence

Read the actual artifact: the screen, the flow, the PRD excerpt, the CLI
report, the journey map, the onboarding copy. Do not invent a flow from a
one-line description if the real artifact is available — read it first. If no
artifact exists yet and the request is genuinely about a not-yet-built idea,
say so explicitly and treat every downstream claim as inferred rather than
observed.

### 2. Extract the intents

For the artifact under review, name:

- **Outcome** — the verb and the result: what is the person trying to
  accomplish, stated as an action, not a feature name ("get a trustworthy
  signal report before I merge," not "use the codesignal command"). If the
  artifact is too sparse or ambiguous to support this, record it as "not
  resolvable from the available evidence" rather than guessing a plausible
  one — the same discipline applied to constraints below.
- **Constraints** — everything that bounds how the outcome can be reached.
  Always check explicitly for:
  - *temporal constraints* — deadlines, "before my meeting," "before CI
    runs," release windows;
  - *capacity constraints* — how much attention, effort, or working memory
    the person actually has right now, not how much the flow assumes they
    have.
- **Delegation boundary** — what the person keeps deciding themselves (which
  findings matter, whether to merge) versus what they are willing to hand to
  the system (running the scan, drafting the report, ranking findings by
  severity). State the boundary as a line, not a feeling: name the specific
  decision on each side of it.

### 3. Find the articulation barrier

The articulation barrier is the gap between what the person can say and what
the system needs to know. Separate the signals the artifact relies on into
two kinds:

- **Explicit signals** — what the user actually said, clicked, typed, or
  configured.
- **Implicit signals** — anything inferred from behavior, timing, history, or
  context rather than stated directly.

For **every** implicit signal, pair it with its **privacy cost**: what
inference is being made about the person (their skill level, their trust in
the tool, their team's process maturity, their working hours), and what it
costs them if that inference is wrong or if it is surfaced back to them or to
someone else without their say.

### 4. Set the confidence-to-response policy

State what happens at low, medium, and high inference confidence. This is not
a free design choice at the high end — see the trust-posture fence in the
Pattern library: **high confidence still only proposes, never silently
acts.** Confidence changes how strongly something is suggested and how much
friction gates it, never whether the human is asked at all before an
irreversible or externally visible action happens.

### 5. Design the orchestration surface

Describe how the human sees and steers what the system or an agent is doing
on their behalf: what is visible while it runs, what can be paused,
redirected, or cancelled mid-flight, and how the person distinguishes "this
already happened" from "this is proposed and waiting for me."

### 6. Adversarial agency pass

Check the artifact against every failure mode below. For each one, ask the
paired exposing question and record whether the artifact has an answer.

| Failure mode | Exposing question |
| --- | --- |
| Silent auto-action | Does anything happen on the user's behalf without a visible, prior confirmation step — and would the user notice before it happened? |
| No undo | If this action turns out to be unwanted, what specific control reverses it, and how many steps does that take? |
| Dead-end on wrong inference | When the system infers the wrong intent, does the flow offer a path back to the user's actual goal, or does it terminate with no forward path? |
| Hidden reasoning | Can the user see why the system reached this conclusion or made this suggestion, in language they would accept as a reason? |
| Intent debt | If the user's underlying goal changes after this point, does anything in the flow re-ask, or does the original inference calcify into future behavior unchallenged? |
| Coherence break across surfaces or agents | Does this decision, label, or state stay consistent if the user encounters it again on a different screen, channel, or in another agent's report? |
| Over-inference from behavior | Is this inference justified by what the user actually did, or does it assume something about who they are or what they want that they never signaled? |
| Ignored capacity or deadline | Does this flow demand more attention or time than the user has signaled they can give right now, given what we know about their deadline or workload? |
| Multi-party action without approval | If this action affects or is visible to someone other than the acting user, did that other party consent, or only the acting user? |

### 7. Surface the evidence question

Ask, explicitly: **"What would tell us this worked?"**

Do not answer it. Do not propose metrics, KPIs, or success criteria anywhere
in this analysis — that is a different seat. Hand the resulting measurement
question to `product-sme` as-is. Product measurement belongs to
`product-sme`; if this skill also drafts measures, the pilot ends up with two
measure sets for one journey, and a journey with two measure sets is a
journey that gets measured by neither.

### 8. Hand off

Close by naming the next owner per the map in "Do NOT Use": `product-sme` for
the evidence question and any release-readiness judgment, `system-design-expert`
for any architecture or enforcement question the delegation boundary raises,
and implementation only once a design decision has actually been made — never
skip straight there.

### Illustration: the GitHub App install consent moment

A small worked example, grounded in Coach's actual onboarding flow (see
`docs/product/prd.md` and `docs/architecture/system-overview.md`): before a
repository can be scanned, "the Coach GitHub App must also be installed for
that repository — part of pilot onboarding."

- **Outcome**: the engineer wants a trustworthy signal report on their own
  work, fast, without granting more access than that requires.
- **Constraints**: temporal — they are usually doing this right before or
  right after opening a PR, not as a separate errand; capacity — they will
  not read a permissions essay, they will skim it in seconds.
- **Delegation boundary**: they are willing to hand off "read this
  repository's contents for analysis"; they are not implicitly agreeing to
  "write to this repository" or "scan every repository I can see." The
  architecture's own posture agrees: repository-content mutation is
  "prohibited in v1; explicit developer activation in Next"
  (`docs/architecture/system-overview.md`).
- **Implicit signal / privacy cost**: installing the App on one repository
  could be read as "trust Coach with all my repositories." That inference is
  wrong and costly if surfaced (it would make an engineer distrust the tool
  for over-reaching); the install screen should scope its language to the
  single repository being onboarded, not to the person's account-wide trust.
- **Confidence-to-response policy**: even at high confidence that the
  engineer wants read access to more of their repositories (e.g. because
  their `coach codesignal` runs keep hitting a second repo they don't have
  installed), the correct response is to propose "install Coach on this
  repository too," never to broaden scope silently.
- **Agency risks found**: A1 (Major) coherence break — the architecture's
  "system-owned, non-blocking advisory feedback" posture
  (`docs/architecture/system-overview.md`) is stated in docs but never
  restated at the point where the first report appears, so a first-time user
  could predictably misread an advisory finding as a hard gate; A2 (Minor)
  hidden reasoning — the install screen does not say why Contents-API read
  access is needed rather than a broader scope, though the granted scope
  itself is still stated correctly.

## Output contract

Respond with a two-to-three sentence verdict naming the sharpest finding —
the single agency risk or intent gap most worth the reader's attention — then
these sections, in this order:

1. **Intent map** — outcome, constraints (temporal and capacity called out
   explicitly), and delegation boundary, using the schema in "Editable Intent
   + Constraint Object" below.
2. **Interaction model** — the explicit and implicit signals identified, each
   implicit signal paired with its privacy cost, and the confidence-to-response
   policy at low/medium/high confidence.
3. **Orchestration surface** — what the human can see, steer, pause, or
   cancel while the system or an agent acts on their behalf.
4. **Agency risks** — every failure mode from the adversarial pass with a
   concrete finding, each entry given a stable ID (`A1`, `A2`, `A3`, …) and
   one severity — **Blocking** (cannot complete, or consents without seeing
   what to), **Major** (completes but predictably misreads what the system
   will do unattended), or **Minor** (friction, no decision consequence) —
   listed in severity-descending order. Omit failure modes with no finding
   rather than padding the list.
5. **Open questions** — anything that could not be resolved from the
   available evidence, including the evidence question from Procedure step 7
   handed to `product-sme` verbatim.
6. **Out of scope** — anything explicitly deferred per the Do NOT Use map
   (implementation, product-quality verdict, architecture decision), named
   with its intended owner.

## Pattern library

Reusable interaction patterns for carrying intent across a handoff without
losing it:

- **Progressive disclosure of reasoning** — show the one-line "why" for a
  suggestion up front, with the full inference chain available on demand,
  rather than either hiding it entirely or forcing the user through it
  unasked.
- **Propose-and-confirm** — the system states what it is about to do and
  waits for an explicit go-ahead before doing it, especially for anything
  hard to reverse.
- **Confidence-tiered friction** — the amount of confirmation required scales
  with how consequential and how reversible the action is, not with how
  confident the system is that it guessed right.
- **Visible delegation boundary** — the UI states, in the user's language,
  what has been handed off and what has not (e.g. "Coach reads this
  repository's diffs; it does not open PRs or push commits").
- **Scoped consent** — a permission or install grant is described and scoped
  to the specific artifact it applies to (one repository, one job), not
  phrased so broadly that it reads as blanket trust.

### Trust-posture fence (this repo's override — non-negotiable)

Where a pattern above, or any pattern brought in from elsewhere, conflicts
with this product's stated trust posture, **the trust posture wins.** This
skill does not infer architecture policy from a screen; it reads the posture
that has already been decided and designs within it. Concretely, for this
repository (`docs/architecture/system-overview.md`,
`docs/product/prd.md`):

- **High confidence still only proposes.** Never let a design "just do it"
  once confidence crosses some threshold. The source method this skill is
  built from allowed a generic "at high confidence, just do it" default; this
  skill deliberately replaces that default with propose-and-confirm at every
  confidence level, because this product's posture is "fail open for
  advisory developer flow but fail closed for credentials and mutation" —
  and an unreviewed high-confidence auto-action on someone else's repository
  is exactly a mutation-shaped risk. Treat that replacement as a hard
  override of the source pattern, not a case-by-case judgment call.
- **No silent mutation.** Repository-content mutation is out of scope for
  designs reviewed under this skill unless the artifact under review already
  documents "explicit developer activation" for it; do not design a flow
  that mutates without a human-visible, human-triggered step.
- **Coverage honesty over anticipatory automation.** An absent signal, an
  unanalyzed case, or a low-confidence guess must be represented as absence
  or uncertainty, never smoothed over by inventing a plausible-looking
  automated action to fill the gap.

## Editable Intent + Constraint Object

Use this as the literal schema for the "Intent map" output section. It is
meant to be copied, filled in, and kept as a living artifact the human can
edit as their understanding of their own intent evolves — it is not a
one-time inference to be thrown away after this report.

```yaml
intent:
  outcome: "<verb + result the person is trying to reach, or 'not resolvable from the available evidence'>"
  constraints:
    temporal: "<deadline, cadence, or 'none observed'>"
    capacity: "<attention/effort budget, or 'none observed'>"
    other: []
  delegation_boundary:
    keeps_deciding: []
    willing_to_hand_off: []
  confidence_policy:
    low: "<response at low confidence>"
    medium: "<response at medium confidence>"
    high: "<response at high confidence — must still propose, never silently act>"
  signals:
    explicit: []
    implicit:
      - signal: "<what is inferred>"
        privacy_cost: "<what it costs the person if wrong or surfaced>"
```

## Using the agent, and working without it

This skill can be run two ways, and the reader must be told which one they
got:

- **In the main conversation thread**, this skill supports multi-turn
  pairing: the human can push back on a finding, ask a follow-up, redirect
  the analysis toward a different screen mid-review, or ask "what about
  this constraint" and get an updated intent map in the same conversation.
- **As the `ux-advocate` agent** (a separate, single-shot subagent — see the
  epic's other task, not part of this skill's own definition), when it is
  registered in the current environment, the analysis is spawned once,
  returns one report, and cannot be paired with turn-by-turn. It does not
  have the multi-turn capability the main-thread path has, and this document
  never claims otherwise.

If `ux-advocate` is available as a registered subagent in this environment
(for example, a `.claude/agents/ux-advocate.md` or equivalent definition
exists), it is reasonable to delegate to it via the Agent tool for a
single-shot version of this analysis, and to say so plainly when doing so.

If `ux-advocate` is **not** registered in this environment — for example, in
a different project, or a harness where the agent mirror was never
installed — do not tell the reader to invoke it. Instead, run the full
procedure above yourself, inline, in the current conversation, and still
produce the complete output contract:
the two-to-three sentence verdict followed by Intent map, Interaction model,
Orchestration surface, Agency risks, Open questions, and Out of scope. The
absence of the agent changes who runs the procedure; it never changes what
the reader receives.

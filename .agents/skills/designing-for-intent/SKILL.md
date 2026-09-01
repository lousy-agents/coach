---
name: designing-for-intent
description: Advocate for the user's intent map — desired outcome, constraints, and delegation boundary — across a journey, especially onboarding and consent moments, rather than for the screen or user experience sitting in front of you. Use to review a UX flow, agentic handoff point, or delegation-boundary decision for agency risks and articulation barriers before implementation begins. Not for product-quality verdicts or system design.
argument-hint: "Artifact to analyze (screen, flow, PRD excerpt, onboarding/consent step, CLI report); optional depth mode (strategic/tactical) — inferred if omitted"
allowed-tools: Read, Grep, Glob
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
  API, editing application or library source) — that is not this
  skill's job at any point in the procedure. Hand it to the implementing
  agent (e.g. whatever implementation workflow your project uses) once a
  design decision has actually been made — see Procedure step 8.
- **The product-strategy peer** — for release-readiness, adoption risk, or
  "is this useful/lovable/ready" verdicts, including any dedicated go/no-go
  evaluation flow the host project runs. This skill only feeds that peer the
  evidence question (see Procedure, step 7); it never answers it.
- **The system-design peer** — for infrastructure, security model, or
  platform-foundation questions raised by a design (e.g. "should this
  delegation boundary be enforced server-side or client-side"). This skill
  names the question; the system-design peer answers the architecture.
- **`ux-advocate`** — roster peer for how a journey, screen, CLI, copy, or
  consent moment *reads* from outside the repo (sequencing, messaging,
  encounter). Complementary, not a substitute: this skill does not spawn
  that seat, and that seat does not run this method. Hand
  copy/sequencing/legibility questions to it; keep the intent-map audit here.
- **Visual design, layout, affordance, or classic usability-heuristic
  review** — this skill reads a screen as an encounter (intent, consent,
  agency). It does not judge whether the screen is pretty or follows a
  heuristic checklist.

## Procedure

Use only `Read`, `Grep`, and `Glob`. Do not write or edit files. Run every
step inline in this conversation. Do not delegate this analysis to
`ux-advocate` or any other subagent.

### 1. Ground in evidence

Load the actual artifact before analyzing it. If the user gave a path, use
`Read`. If they named a screen, flow, PRD excerpt, CLI report, journey map,
or onboarding copy without a path, use `Glob`/`Grep` to locate it, then
`Read`. If they pasted the artifact in the conversation, use that text. Do
not invent a flow from a one-line description when the real artifact is
available. If no artifact exists yet and the request is genuinely about a
not-yet-built idea, say so explicitly and treat every downstream claim as
inferred rather than observed.

### 2. Extract the intents

For the artifact under review, name:

- **Outcome** — the verb and the result: what is the person trying to
  accomplish, stated as an action, not a feature name. Developer-tool
  shape: "get a trustworthy signal report before I merge," not "use the
  scan command." Screen/consent shape: "grant this app access to one
  project," not "click Continue." If the artifact is too sparse or
  ambiguous to support this, record it as "not resolvable from the
  available evidence" rather than guessing a plausible one — the same
  discipline applied to constraints below.
- **Constraints** — everything that bounds how the outcome can be reached.
  Always check explicitly for:
  - *temporal constraints* — deadlines, "before my meeting," "before CI
    runs," release windows;
  - *capacity constraints* — how much attention, effort, or working memory
    the person actually has right now, not how much the flow assumes they
    have.
- **Delegation boundary** — what the person keeps deciding themselves
  versus what they are willing to hand to the system. Developer-tool
  shape: keeps deciding which findings matter and whether to merge; hands
  off running the scan and drafting the report. Screen/consent shape:
  keeps deciding the scope of access; hands off reading the selected
  project. State the boundary as a line, not a feeling: name the specific
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

`Read` [`./references/pattern-library.md`](./references/pattern-library.md)
before choosing a response. State what happens at low, medium, and high
inference confidence. This is not a free design choice at the high end —
the trust-posture fence in that file is non-negotiable: **high confidence
still only proposes, never silently acts.** Confidence changes how strongly
something is suggested and how much friction gates it, never whether the
human is asked at all before an irreversible or externally visible action
happens.

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
question to the product-strategy peer as-is. Product measurement belongs to
the product-strategy peer; if this skill also drafts measures, the project
ends up with two measure sets for one journey, and a journey with two
measure sets is a journey that gets measured by neither.

### 8. Hand off

Close by naming the next owner per the map in "Do NOT Use": the
product-strategy peer for the evidence question and any release-readiness
judgment, the system-design peer for any architecture or enforcement
question the delegation boundary raises, and implementation only once a
design decision has actually been made — never skip straight there.

## Pilot illustration

A **pilot** worked example (Coach) of this procedure is in
[`./references/github-app-install.md`](./references/github-app-install.md).
`Read` it only when a concrete example would help. It is not a procedure
step, not a runtime dependency of the host project, and not the artifact
under review unless the user asked to review that install/consent flow.

## Output contract

Respond with a two-to-three sentence verdict naming the sharpest finding —
the single agency risk or intent gap most worth the reader's attention — then
these sections, in this order:

1. **Intent map** — the filled `intent:` block from the schema below
   (outcome, constraints, delegation boundary).
2. **Interaction model** — the filled `interaction:` block from the schema
   below (signals with privacy costs, confidence-to-response policy).
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
    handed to the product-strategy peer verbatim.
6. **Out of scope** — anything explicitly deferred per the Do NOT Use map
   (implementation, product-quality verdict, architecture decision, visual
   design / usability-heuristic review), named with its intended owner.

## Pattern library

`Read` [`./references/pattern-library.md`](./references/pattern-library.md)
before Procedure step 4 and when recommending a handoff pattern. That file
holds the reusable patterns and the trust-posture fence.

## Editable Intent + Constraint Object

Copy, fill, and keep this YAML as a living artifact the human can edit as
their understanding of their own intent evolves — it is not a one-time
inference to throw away after this report. Emit the `intent:` key as
**Intent map** and the `interaction:` key as **Interaction model**. Do not
merge or duplicate the two keys.

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
interaction:
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

This skill is the method. Always run the procedure inline in the current
conversation and still produce the complete output contract: the
two-to-three sentence verdict followed by Intent map, Interaction model,
Orchestration surface, Agency risks, Open questions, and Out of scope.

`ux-advocate` is a complementary roster seat, not a single-shot of this
analysis. Do not spawn it as a substitute.

- **This skill** fills the living intent/constraint YAML, runs the nine-mode
  adversarial pass, and sets the confidence-to-response policy. It supports
  multi-turn pairing in the main thread.
- **`ux-advocate`** reads how a customer encounters the product (copy,
  sequencing, consent language, screens or CLI, multi-actor journeys) as a
  peer to the product-strategy peer. It does not run this method and does
  not fill this schema.

If part of the request is copy, sequencing, or how an encounter *reads*,
name `ux-advocate` under Out of scope and still complete this method for
the intent/agency half. If `ux-advocate` is not registered, do not tell
the reader to invoke it.

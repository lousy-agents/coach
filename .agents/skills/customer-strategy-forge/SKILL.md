---
name: customer-strategy-forge
description: 'Turn customer evidence into auditable personas, situation cards, journey maps, progression hypotheses, opportunity maps, and validation plans without reverse-engineering needs from a product inventory. Use for customer research synthesis, evidence audits, persona development, segmentation, customer-journey mapping, or product-strategy portfolio mapping. Supports paired human-in-the-loop work and unattended subagent runs; stops when evidence, isolation, or authorization is insufficient. Not for market sizing or feature specifications.'
argument-hint: 'Inquiry or path; optional mode (paired/autonomous), role, and requested artifacts'
---

# Customer Strategy Forge

Turn versioned customer evidence into decision-useful strategy while keeping
observations, interpretations, and product ideas distinguishable.

This skill is an orchestrator. It selects only the operations needed for the
inquiry; it is not a mandatory persona-to-portfolio pipeline.

## When to Use

Use this skill when:

- auditing whether available customer evidence can support a decision;
- developing or revising an evidence-backed persona or behavioral segment;
- mapping a situation, customer journey, progression hypothesis, or actor
  ecosystem;
- mapping independently reviewed customer needs to products, modules,
  alternatives, or partner opportunities; or
- defining the next research or validation action when evidence is weak.

Do not use it to:

- infer customer needs from a README, feature list, package graph, or product
  roadmap;
- estimate market size or willingness to pay from qualitative or social
  evidence;
- produce a feature specification or implementation plan;
- acquire live research silently during synthesis; or
- turn a maturity model into a ranking of people or organizations.

## Non-Negotiable Invariants

1. Evidence precedes product fit.
2. People are not maturity stages; situations may vary by repository,
   workflow, risk domain, and time.
3. Contradicting evidence remains visible.
4. Reproducibility is not validation.
5. Founder knowledge is labeled and cannot become customer validation by
   repetition.
6. Repository and module boundaries are not customer boundaries.
7. Emotional outcomes such as relief, confidence, flow, control, pride, and
   joy require evidence appropriate to the claim.
8. A valid run may stop at the evidence audit or conclude that no product
   action is justified.

## Modes

Choose the mode from the invocation context:

- **Paired** — default in a direct human conversation. Ask only questions that
  materially change the run, offer bounded choices, and obtain human decisions
  at gates.
- **Autonomous** — use when delegated to a subagent or explicitly requested as
  unattended. Complete from decisions and permissions already recorded in the
  run manifest. Missing evidence, authority, isolation, or a required
  independent critic produces a terminal stop artifact.

Autonomous does not mean permissionless. It means the run completes as either a
supported result or an explicit stop without waiting for conversation.

Read [modes-and-routing.md](./references/modes-and-routing.md) when selecting
the mode, role, or requested artifacts.

## Role Boundaries

One invocation may hold only one of these roles:

- **Coordinator** — may know the product. Defines the inquiry, prepares the
  manifest and product-blind packet, selects operations, and routes outputs.
- **Synthesizer** — sees only the approved customer packet. Audits evidence and
  creates selected customer artifacts without product inventory.
- **Critic** — a fresh, independent invocation that checks candidate customer
  artifacts against the captured evidence and access record.
- **Product mapper** — receives only independently passed customer artifacts,
  then introduces product goals, capability inventory, alternatives, and
  strategic constraints.

Do not combine synthesizer and critic in one context. Do not introduce product
inventory to the synthesizer or critic. A product-aware invocation can
coordinate or map product fit; it cannot honestly relabel itself product-blind.

## Procedure

### 1. Resolve the Run

Identify:

- mode: paired or autonomous;
- role for this invocation;
- strategic inquiry and decisions to inform;
- decision date and evidence cutoff;
- requested artifacts;
- named human decision owner; and
- existing inquiry, pack, manifest, and output paths.

If a human asks generally to “make a persona” or “map the customer journey,”
start as coordinator. In paired mode, clarify the decision and evidence source.
In autonomous mode, missing required fields stop the run.

Load [modes-and-routing.md](./references/modes-and-routing.md) and select the
smallest run graph, then dispatch. The default is Grounding Brief + Evidence
Audit + stop-or-continue. A continue verdict does not authorize additional
artifacts.

### 2. Dispatch by Role

Follow only the resolved role. Keep the resolved role until return. Use the
Role Dispatch table in [modes-and-routing.md](./references/modes-and-routing.md).

Load on entry:

- Coordinator: [modes-and-routing.md](./references/modes-and-routing.md),
  [isolation-and-critic.md](./references/isolation-and-critic.md)
- Synthesizer: [evidence-and-gates.md](./references/evidence-and-gates.md),
  [operations.md](./references/operations.md),
  [artifact-contracts.md](./references/artifact-contracts.md)
- Critic: [isolation-and-critic.md](./references/isolation-and-critic.md),
  [evidence-and-gates.md](./references/evidence-and-gates.md),
  [artifact-contracts.md](./references/artifact-contracts.md)
- Product mapper: [artifact-contracts.md](./references/artifact-contracts.md),
  Product-Aware Operations only in
  [operations.md](./references/operations.md) (operations 12–14), and this
  invocation's released-input manifest. Do not load customer operations 1–11
  or research refresh.

A child-role return is not inquiry completion. Inquiry completion is the
Coordinator's publication after required children return. After a child
returns, resume coordination. Do not publish customer artifacts before an
independent pass.

If this invocation is Synthesizer, Critic, or Product mapper, perform only
that role and stop after the return. Only the Coordinator continues into
packet routing and publication.

### 3. Establish the Input Boundary

Coordinator or Synthesizer. Critic verifies the critic packet and access
record instead of creating a synthesis manifest. Product mapper verifies
its released-input manifest only.

Before synthesis, load
[isolation-and-critic.md](./references/isolation-and-critic.md). Create or
verify a manifest that names exact allowed inputs, denied product material,
selected outputs, available tools, output paths, and context provenance.

Use actual harness permissions or an isolated workspace when claiming access
is restricted. A filename, prompt instruction, or `allowed-tools` declaration
alone is not enforcement.

If product material has already entered the proposed synthesizer or critic
context, start a fresh invocation with a sanitized packet. If that is
unavailable, stop.

### 4. Audit Evidence Before Synthesis

Synthesizer, or Coordinator preparing an audit-only packet. Critic and
Product mapper skip.

Load [evidence-and-gates.md](./references/evidence-and-gates.md). Inspect the
actual approved evidence records and their source context, not only IDs or
summaries.

Publish either:

- a Grounding Brief and Evidence Audit with allowed claim scope and validation
  ceilings; or
- the stop artifact defined in that reference.

Do not produce downstream customer or product artifacts from a failed run.

### 5. Run Only Selected Customer Operations

If the manifest explicitly selects customer artifacts and the audit permits
them, load [operations.md](./references/operations.md). Run only the operations
and dependencies named in the manifest.

Use [artifact-contracts.md](./references/artifact-contracts.md) for the selected
output formats and persistence rules. Every consequential claim carries its
scope, inference, supporting and contradicting evidence, confidence rationale,
validation state, and decision relevance.

Progression models enter only after behavioral synthesis. Withhold any
progression model until the behavioral artifact is version-pinned. See
[modes-and-routing.md](./references/modes-and-routing.md). Product goals and
capabilities do not enter customer operations.

### 6. Obtain Independent Criticism

Any candidate persona, segment, situation card, needs/jobs map, journey,
progression interpretation, or customer-emotion map requires the Critic role
before publication or product mapping.

Only the Coordinator routes a fresh Critic. A Critic invocation must not
reach this step.

Delegate to a fresh non-forked subagent or begin a separate invocation using
the critic packet in [isolation-and-critic.md](./references/isolation-and-critic.md).
The verdict is `pass | revise | split-segment | research-required` and cites
claim and evidence IDs.

If a separate critic cannot be run:

- paired mode: prepare the critic packet and ask the human to route it;
- autonomous mode: emit a stop artifact with Disposition
  `independent-critic-unavailable`.

Do not self-approve in the synthesizer context.

### 7. Map Product Fit Only After a Pass

Run product or portfolio mapping only when explicitly requested and only from
version-pinned customer artifacts that passed independent review.

Only the Product mapper performs this work, or the Coordinator routes that
role after a pass.

The Product mapper may identify strong or partial coverage, gaps, overlaps,
broken handoffs, shared-artifact opportunities, partners, intentionally
unsupported needs, or no justified product action. It must not rewrite the
customer model to make existing modules necessary.

Experience mapping may connect capabilities to already evidenced emotional
outcomes. It may propose labeled delight hypotheses, but it cannot invent joy
from branding or product intent.

### 8. Persist the Outcome

Write only to authorized locations. Packs and published artifact versions are
append-only. Chat may summarize a run but is not the system of record when
file output is available and authorized.

If no authorized stop sink can be resolved, return the structured stop payload
through the caller or conversation channel. Record that no artifact was
persisted. Do not invent a run ID, permission, or path.

Record:

- exact input and workflow versions;
- the independent critic verdict, when required;
- product decisions supported, rejected, or deferred;
- human authorizations or waivers;
- remaining evidence gaps; and
- the semantic change report for a rerun.

## Stop Conditions

Stop rather than synthesize when any material condition holds:

- required customer evidence or provenance is absent, unless a recorded
  speculative authorization covers the requested assumption artifact;
- evidence is expired, freshness is unresolved for a material time-sensitive
  claim, the record is unsafe to use, or it is outside the decision cutoff;
- the requested claim exceeds the sampling method;
- unresolved material contradictions prevent a coherent scope;
- product contamination invalidates a product-blind invocation
  (`isolation-failed`);
- privacy, consent, or redaction review is missing
  (`authorization-missing`);
- the run needs an unrecorded human authorization
  (`authorization-missing`); or
- an independent critic is required but unavailable
  (`independent-critic-unavailable`).

Follow the stop-only publication contract in
[evidence-and-gates.md](./references/evidence-and-gates.md). Use that
closed Disposition set; do not invent another code. If the stop sink
cannot be resolved, return the payload in-band and do not claim a file was
written. A stop is a useful result, not permission to fill gaps with fluent
prose.

## Tool and Harness Portability

The skill requires read/search access to the approved packet and, when files
are requested, bounded write access to the declared output paths. Live research
needs a separate invocation with explicit source authorization. Product-aware
mapping needs separately released inventory inputs.

Subagent, sandbox, permission, and skill-installation mechanics differ by
harness. Detect available capabilities; do not claim technical isolation or
fresh context without verifying them. When a required capability is missing,
use the paired handoff or autonomous stop behavior above.

## Final Response

Summarize:

1. run mode, role, and disposition;
2. artifacts written, the stop-artifact path, or the in-band stop payload
   when nothing was persisted;
3. validation ceiling and material contradictions;
4. critic verdict, if applicable;
5. decisions supported, unsupported, or deferred; and
6. the next evidence or human decision needed.

Do not call an artifact “validated” globally. State claim-level validation and
its limits.

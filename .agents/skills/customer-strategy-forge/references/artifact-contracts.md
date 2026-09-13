# Artifact and Persistence Contracts

Load only the sections needed for selected outputs.

## Trial Layout

```text
inquiries/<inquiry-id>.md
inquiries/sealed/<inquiry-id>.md
research/packs/<pack-id>/<version>/pack.yaml
research/claims/<claim-id>.yaml
research/artifacts/<artifact-id>/<version>/
research/runs/<run-id>/manifest.yaml
research/runs/<run-id>/access-log.yaml
research/runs/<run-id>/drafts/
research/decisions/log.yaml
research/stop/<run-id>.md
```

Use repository-specific equivalents when the manifest explicitly names them.
Do not invent paths outside the authorized output scope. If no authorized
stop sink or run ID can be resolved, return the structured stop payload
in-band rather than writing a file.

Signal Packs and published artifact versions are append-only. A changed
observation gets a new `ev-<pack>-<n>` ID and a supersession link. A claim ID
`cl-<artifact>-<n>` persists for the same proposition and scope; changed
evidence creates an append-only claim revision. A materially different claim
gets a new ID.

Each artifact pins claim revisions, evidence-pack versions/digests, manifest,
workflow version, critic verdict, assessment date, and decision date.

## Grounding Brief

```yaml
problem_hypothesis:
strategic_questions: []
decisions_to_inform: []
decision_horizon:
decision_date:
evidence_cutoff:
human_owner:
run_manifest_ref:
known_constraints: []
assumptions: []
required_artifacts: [grounding-brief, evidence-audit]
out_of_scope: []
```

## Actor Ecosystem

For each actor record role, situation, relationship, incentives, risks,
decisions, outputs received, and evidence references. Do not collapse author,
reviewer, buyer, operator, approver, or risk owner for narrative convenience.

## Persona

Include only decision-relevant sections:

1. Status, scope, and validation ceiling
2. Evidence summary
3. Defining situations
4. Desired progress and goals
5. Current behavior and workarounds
6. Pains, anxieties, constraints, and tradeoffs
7. Functional and emotional outcomes
8. Decision-relevant behavioral traits
9. Trust builders and breakers
10. Relevant customer language
11. Associated situation cards
12. Assumptions, contradictions, and validation debt
13. Decisions this artifact can and cannot inform

Do not add a name, age, biography, quote, or image unless evidence supports it
and it changes a decision.

## Situation Card

```text
trigger
-> desired progress
-> current approach
-> friction and uncertainty
-> functional and emotional stakes
-> evidence of success
-> next meaningful state
```

One persona may have several situations. A situation becomes a separate persona
only when evidence supports a materially different behavioral segment.

## Progression Context

```yaml
model_ref:
observed_stage_signals: []
stage_or_range_hypothesis:
confidence:
scope: repository | workflow | team | organization
current_behaviors: []
desired_transition:
resisted_or_inappropriate_transition:
transition_blockers: []
adjacent_possible_states: []
evidence_refs: []
```

The model is an interpretation, not evidence.

## Customer Journey

For each moment include trigger/entry, actions, decisions, actors, tools,
traveling context/intent/policy/evidence, handoff/continuity, current
workaround, trust/emotional state with evidence, abandonment risks, evidence of
progress, and next state. Organize by customer progress, not product surfaces.

## Portfolio Coverage

| Customer job, pain, or transition | Claim/evidence strength | Existing coverage | Handoff or gap | Strategic classification | Validation needed |
| --- | --- | --- | --- | --- | --- |

Classifications may be differentiating, parity, partner, intentionally
unsupported, uncertain, possible future module, or no action.

## Experience State

```yaml
experience_state_before:
  value:
  evidence_refs: []
  validation_state: speculative
experience_state_after:
  value:
  evidence_refs: []
  validation_state: speculative
trust_builder:
trust_breaker:
continuity_break:
intervention_hypothesis:
evidence_of_progress:
```

Unreferenced emotions remain speculative and are not customer findings.

## Decision Log Entry

Record date, owner, input/artifact versions, decision, evidence-qualified
rationale, alternatives, unsupported claims, waiver details, validation work,
and revisit trigger. Append; never rewrite an earlier decision.

## Rerun Change Report

Compare:

- claim IDs and revisions;
- supporting and contradicting evidence sets;
- validation-state changes;
- segment split/merge decisions;
- stop/go outcome;
- journey transitions;
- portfolio coverage;
- decisions affected; and
- remaining debt.

Prose may vary. Material drift under unchanged inputs is reported for human
review rather than silently accepted. Do not promise byte-identical output.

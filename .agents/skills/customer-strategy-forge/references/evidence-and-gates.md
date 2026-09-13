# Evidence and Gates

These contracts make evidence strength inspectable. The validation vocabulary
is provisional and claim-specific; it is not a universal scoring system.

## Evidence Record

```yaml
evidence_id: ev-<pack>-<n>
source_type: interview | observation | support | issue | telemetry | social | document | founder
source_ref: <stable captured source locator>
observed_at: <date or unknown>
captured_at: <date>
population_or_context: <who or what was observed>
session_id: <pseudonymous identifier when applicable>
participant_id: <pseudonymous identifier when applicable>
acquisition_method: <how the record was selected or collected>
verbatim_or_derived: verbatim | paraphrase | interpretation
observation: <what was actually seen, heard, or measured>
situation: <where or when it occurred>
known_biases: []
visibility_bias: <required for social evidence>
founder_authored: true | false | unknown
privacy_constraints: []
consent_or_permitted_use: <capture, retention, model, and sharing scope>
redaction_review: <reviewer and outcome>
freshness_horizon: <absolute reassess-by date and rationale, or unresolved>
```

Source labels do not determine strength by themselves. A support issue may
report a preference rather than document action; telemetry may show an action
without explaining the person's goal or emotion.

## Claim Record

```yaml
claim_id: cl-<artifact>-<n>
claim: <concise falsifiable statement>
claim_type: behavior | pain | goal | need | job | segment | progression | emotion | opportunity
supporting_evidence: []
contradicting_evidence: []
inference: <how the evidence relates to the claim>
confidence: low | medium | high
confidence_rationale: <why>
scope: person | situation | repository | workflow | team | organization | market
decision_relevance: <decision this can influence>
validation_state: speculative | evidence-informed | interview-supported | behaviorally-validated
validation_basis: <qualifying evidence, population, and limits>
assessment_date: <date>
decision_date: <date>
```

Confidence and validation state are separate. Strongly consistent founder
intuition may have high subjective confidence while remaining speculative or
evidence-informed.

## Promotion Rubric

| State or gate | Required test |
| --- | --- |
| Speculative | Explicit hypothesis, intended use, forbidden decisions, and named human authorization when a speculative artifact was requested. Evidence may be empty only when disclosed. |
| Evidence-informed | Relevant traceable records, stated inference and scope, and no unsupported prevalence claim. Founder-only support cannot raise a customer claim above this state; unsupported founder intuition remains speculative. |
| Interview-supported | `source_type: interview` with verbatim excerpts or reviewed notes, distinct session/participant IDs and counts, relevant situations, and inquiry-specific sufficiency criteria declared before promotion. Interviews support accounts and meaning; they do not by themselves prove behavior or prevalence. |
| Behaviorally-validated | Captured action or outcome directly tests the claim in its declared context. State what was tested, the result, boundaries, and counterevidence. Do not infer causal effect, prevalence, or an internal emotion without evidence suited to that claim. |

Additional gates:

- Founder-authored public posts remain founder evidence even when collected
  from a social channel.
- Social-only claims cannot exceed evidence-informed or assert prevalence.
  Majority-social packs inherit that conservative ceiling, but independently
  qualifying interview or behavior evidence may support a particular claim.
- Qualitative mention counts and repository activity do not establish
  prevalence. Use a sampling or measurement method suited to the population
  claim.
- Classify `freshness_horizon` as one of three states so “not proven expired”
  is not treated as current. **current:** a reassess-by date after the
  decision date with the required rationale. **expired:** a date before the
  decision date. **unresolved:** the value `unresolved`, a date without
  rationale, or a missing date on a material time-sensitive claim. Unresolved
  freshness blocks that claim pending reassessment or explicitly authorized
  scope narrowing; autonomous mode does not invent a horizon or waiver.
  Evidence collected after a historical decision's cutoff cannot justify that
  decision. Re-check freshness when a passed artifact is reused at a later
  decision date, including before product mapping.
- Unresolved material conflicts in goals, risk tolerance, buyer, or workflow
  block merging. Investigate context, time, and source quality. Contradiction
  can indicate situations or changing behavior rather than a separate segment.
- Every asserted emotional state has evidence references and its own validation
  state. Telemetry alone cannot establish an internal feeling.
- Missing inspectable evidence context, product leakage, unapproved acquisition,
  or unresolved consent/redaction fails the applicable run.

Do not invent universal interview counts, freshness periods, or numerical
segment thresholds. The named human owner declares decision-specific criteria
before promotion. A waiver records the gate, rationale, residual risk, allowed
uses, and forbidden claims; it cannot create consent, undo product exposure, or
upgrade an evidence state.

## Evidence Audit

The audit records:

- input pack versions and content digests;
- source mix and acquisition methods;
- provenance and inspectability;
- privacy, consent, and redaction status;
- freshness relative to decision date and cutoff;
- contradictions and population gaps;
- allowed claim types/scopes;
- claim-level validation ceilings; and
- `continue | research-required | revise-inputs`.

“Continue” permits only the artifacts already selected in the manifest.

This speculative branch precedes the missing-observation stop so a recorded
authorization is not treated as customer evidence, and missing evidence
without authorization is not silently filled.

When the named human owner has recorded scope, intended use, forbidden
decisions, and authorization for a speculative assumption artifact, the
audit may continue for that artifact even if customer observations are
empty or disclosed as absent. Every claim on that artifact stays
`speculative`. Independent criticism is still required. The authorization
does not upgrade validation state, invent observations, waive isolation,
waive consent for supplied material, or waive provenance of asserted
observations.

When no such record exists, missing required customer evidence or
provenance stops the run.

## Stop-Only Publication Contract

An authorized stop sink and run ID are part of preflight. On a failed gate:

- If both resolve, publish only `research/stop/<run-id>.md` as that run's
  result.
- If either cannot be resolved, return the same structured payload through
  the caller or conversation channel. Record that no artifact was
  persisted. Do not invent a run ID, permission, or path to satisfy the
  file contract.

Include:

```markdown
# Customer Strategy Forge Stop

- Run:
- Mode and role:
- Disposition: revise | split-segment | research-required | independent-critic-unavailable | isolation-failed | authorization-missing
- Blocking gate:
- Inputs and versions:
- Relevant claim/evidence IDs:
- Missing evidence:
- Disallowed claims or outputs:
- Research questions or repair instructions:
- Human decision needed:
- Safe successor-run conditions:
```

Use exactly one Disposition from that closed set. Do not invent another code.
`revise`, `split-segment`, and `research-required` are critic non-pass or
audit successor actions. `independent-critic-unavailable` is a required
Critic that cannot be established. `isolation-failed` is product
contamination or unverifiable product-blind isolation.
`authorization-missing` is unrecorded human authorization, consent, or
redaction review. Put the specific condition in Blocking gate.

Do not publish a persona, situation card, needs/jobs map, journey,
progression, portfolio, or joy map from the failed run. Drafts and access logs that already exist stay quarantined as
failed/non-consumable for auditability. A successor uses a new run ID and fresh
context where required.

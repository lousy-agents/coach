# Modes and Routing

Use this reference to choose interaction behavior, invocation role, and the
smallest run graph.

## Invocation Contract

Interpret a request in this shape:

```yaml
mode: paired | autonomous
role: coordinator | synthesizer | critic | product-mapper
inquiry: <question or inquiry file>
decisions_to_inform: []
decision_date: <date>
evidence_cutoff: <date>
requested_artifacts: []
human_owner: <name or stable identifier>
manifest_ref: <optional existing path>
```

Natural-language requests are valid. Resolve explicit values first, then infer
only mode and role:

- direct conversation with a human present -> paired coordinator;
- delegated subagent or “run unattended” -> autonomous;
- “review/criticize these customer artifacts” -> critic;
- “map these passed needs/journeys to our products” -> product-mapper.

Never infer the decision owner, authorization, consent, evidence cutoff, or a
waiver. Paired mode asks; autonomous mode stops.

## Paired Mode

Use short, decision-bearing questions. Ask at most what is needed for the next
gate, then continue.

Ask when:

- the strategic decision or intended artifact is ambiguous;
- two plausible scopes would produce materially different claims;
- an existing authorization cannot be resolved;
- a gate can be addressed through an explicit human choice, such as narrowing
  from a prevalence claim to a qualitative hypothesis; or
- the human must route a clean synthesizer or critic packet.

Do not ask the human to resolve facts already present in approved evidence.
Show evidence and tradeoffs before asking for a decision.

## Autonomous Mode

Autonomous runs do not ask questions, wait for replies, or manufacture human
approval. They:

1. validate the manifest and input boundary;
2. execute only authorized operations;
3. write only declared outputs;
4. use a separate critic when required and available; and
5. terminate with either a supported result or a stop artifact.

A parent agent may prepare a manifest and delegate a role to a subagent. The
subagent treats the manifest as authority only for the decisions explicitly
recorded there.

## Role Dispatch

Resolve the role before any customer or product procedure. One invocation
keeps that role until it returns.

| Role | Entry | This invocation does | Return / stop | Does not |
| --- | --- | --- | --- | --- |
| Coordinator | Default for a general human request, or an explicit coordinate/route request. Load this file, then isolation-and-critic.md to build packets. | Prepare/verify the manifest, authorize a stop sink, assemble role packets, delegate, receive child results, and publish only after a required critic pass. | Inquiry completes when the coordinator has published authorized artifacts or a terminal stop. Child return is not inquiry completion. | Synthesize customer artifacts, criticize them, or map products in this context. |
| Synthesizer | Coordinator packet or explicit synthesizer invocation with an approved product-blind manifest. Load evidence-and-gates.md, artifact-contracts.md, and only selected operations.md sections. | Audit evidence and produce only selected customer artifacts. Persist/version-pin behavioral artifacts before any progression interpretation. | Return the audit, candidates, claim records, and access log to the coordinator. Audit-only runs end here. | Request a critic of its own work in-place, publish after a required critic gate, introduce product inventory, or apply a progression model supplied before the behavioral pin. |
| Critic | Fresh non-forked invocation with the critic packet. Load isolation-and-critic.md and evidence-and-gates.md. | Execute only the critic procedure. | Return exactly one verdict packet to the caller and stop. | Obtain another critic, publish customer artifacts, or map products. |
| Product mapper | Explicit mapping request plus a released-input manifest that pins passed artifacts. Load artifact-contracts.md and product-aware operations only. | Map coverage, gaps, handoffs, alternatives, partners, or no-action from the pinned passed artifacts. | Return the mapping and decision limits to the coordinator. | Rewrite the customer model, treat a non-pass as input, or synthesize new customer claims. |

The coordinator is the publication owner for the inquiry. A child role may
write only the outputs named in its own manifest. Completing a child role is
not permission to continue the parent procedure in the same context.

## Artifact Selection

The Grounding Brief's `required_artifacts[]` is the run graph.

| Requested outcome | Minimum operations |
| --- | --- |
| Determine what evidence can support | Inquiry Grounder + Evidence Auditor |
| Describe a situation or job | Audit + product-blind synthesis + needs/jobs + Critic |
| Develop a persona | Audit + actor/scope as needed + synthesis + segment analysis + persona assembly + Critic |
| Map a journey | Audit + synthesis + needs/jobs as needed + journey synthesis + Critic |
| Interpret progression | A passed customer artifact + optional progression mapping + Critic |
| Map product/portfolio fit | Version-pinned passed customer artifacts + Product mapper |
| Map experience coherence or joy | Passed customer emotion claims; Product mapper when capabilities are included |
| Decide next research | Evidence audit and/or validation planner; research acquisition is a separate invocation |

Dependencies are evidence needs, not a reason to emit every artifact. A journey
does not require a persona when situation-level evidence is sufficient.

## Role Handoffs

### Coordinator to Synthesizer

Pass only:

- product-independent inquiry and decision context;
- evidence-pack pointers and approved records;
- run manifest and provisional output contracts; and
- authorized output locations.

Do not include a progression model, maturity rubric, or stage taxonomy in
this initial packet, even when operation 9 is selected. Persist and
version-pin the behavioral synthesis first. Then release the versioned
model in a separate packet or successor invocation that names the pinned
behavioral artifact as a prerequisite. Observed behavior stays immutable;
model interpretations are separate claims.

Do not pass product goals, feature lists, portfolio hypotheses, solution-led
personas, or the coordinator's product-fit rationale.

### Coordinator to Synthesizer — progression release

Use only after a version-pinned behavioral artifact exists for this inquiry.
Pass the pinned artifact reference, the versioned model, and the selected
progression operation. Honor the routing table: interpret progression from a
passed customer artifact, then obtain independent criticism of the
interpretation. Do not reopen or rewrite the behavioral baseline to fit the
model.

### Coordinator to Critic

Only the Coordinator routes this packet. Pass the product-independent
Grounding Brief, candidate artifacts, complete Claim Records, relevant
approved evidence records/excerpts, manifest, and access log. Do not pass
private reasoning transcripts or product inventory. The synthesizer does
not request a critic in-place.

### Coordinator to Product Mapper

Only the Coordinator routes this packet, and only after a `pass` verdict.
Pass the version-pinned passed artifacts and findings. A non-pass verdict
routes to a new synthesis/research run, never product mapping.

## Examples

Paired:

> Use customer-strategy-forge to determine whether our interview pack can
> support a persona for onboarding decisions. Work with me at the gates.

Autonomous:

> Use customer-strategy-forge in autonomous synthesizer mode. Follow
> research/runs/run-014/manifest.yaml. Return the audit, journey candidate,
> and claim records to the coordinator, or emit a terminal stop. Do not
> publish the journey and do not request a critic in this invocation.

Independent critic:

> Use customer-strategy-forge in autonomous critic mode with the packet named
> in research/runs/run-014/critic-manifest.yaml. Do not access product files.

# Isolation and Independent Criticism

Product-blind synthesis is an input-boundary claim. Do not promise it merely
because the prompt says “ignore the product.”

## Separate Method and Instance

Keep reusable method, open inquiry, and sealed product context separate:

```text
workflow or skill instructions
inquiries/<inquiry-id>.md
inquiries/sealed/<inquiry-id>.md
```

The open inquiry contains product-independent questions, decision context, and
evidence-pack pointers. The sealed file contains product goals, capability
inventory, portfolio hypotheses, alternatives, and solution-led prior
artifacts. Stage/operation 12 releases it only to the Product mapper.

If the initial prompt mixed customer and product material, extraction is not
enough. Start the Synthesizer in fresh context without the original prompt,
summaries, or inherited conversation.

## Run Manifest

```yaml
run_id:
inquiry_id:
mode: paired | autonomous
role:
human_owner:
decision_date:
evidence_cutoff:
workflow_version:
allowed_inputs:
  - path:
    version_or_digest:
denied_inputs:
  - resolved_path:
selected_outputs: []
selected_operations: []
allowed_output_paths: []
tool_policy:
  read_scope: []
  write_scope: []
  live_network: deny
  broad_repository_discovery: deny
context_provenance:
  fresh_invocation:
  inherited_product_context:
  automatically_loaded_material: []
enforcement:
  method:
  access_log:
critic:
  required:
  invocation_ref:
```

Resolve actual paths; examples are not a deny boundary. Known denied material
usually includes product README/feature lists, marketing pages, package or
module inventories, doctor output, roadmaps, sealed inquiry files, and
solution-led prior personas.

Use an allowlisted input packet plus verified harness permissions or an
isolated workspace/runtime. Most-permissive defaults, automatic indexing,
memory, inherited conversation, or automatically loaded instructions can
break the boundary. Record what was checked.

The synthesizer and critic cannot use shell, web, GitHub, or broad search to
recover missing evidence. A bounded recorder may write authorized outputs.
Live acquisition belongs to a separate Research Refresh invocation.

If sealed material is opened or delivered, stop immediately with
Disposition `isolation-failed`. Deleting it from the prompt or promising
not to use it cannot repair the current invocation.

## Critic Packet

The Critic receives:

- product-independent Grounding Brief;
- candidate customer artifacts and Claim Records;
- full approved evidence records or excerpts behind relevant IDs, including
  contradictions;
- applicable gates and provisional artifact contracts;
- run manifest and access log; and
- exact candidate versions/digests.

It does not receive:

- product goal, inventory, portfolio hypothesis, or product mapping;
- synthesizer conversation or advocacy for its conclusions;
- unrecorded private chain-of-thought; or
- live research tools.

IDs alone are insufficient: the critic must inspect the captured evidence
context needed to test the claim.

## Critic Procedure

1. Verify packet versions, role, isolation record, and evidence inspectability.
2. For each material claim, test observation/inference separation, supporting
   and contradicting evidence, scope, confidence rationale, validation state,
   prevalence language, freshness, and decision utility.
3. Check that distinct actors or materially incompatible contexts were not
   merged for narrative convenience.
4. Check that progression language follows observed behavior rather than
   seniority, title, or product adoption.
5. Check that journeys follow customer progress rather than product screens,
   commands, repositories, or modules.
6. Check that emotional outcomes have suitable evidence and no telemetry-only
   inference of internal state.
7. Return exactly one verdict:
   `pass | revise | split-segment | research-required`.

This procedure is the entire Critic invocation. Return that verdict packet
to the caller and stop so the Critic does not continue into synthesis,
publication, product mapping, or another critic request.

Each finding contains:

```yaml
finding_id:
severity: blocking | major | minor
verdict:
claim_ids: []
evidence_ids: []
problem:
consequence:
directive:
```

When evidence is missing, say so instead of inventing an ID. A run-level
finding may cite the manifest and leave claim/evidence lists empty.

Only `pass` permits publication or Product mapper input. Other verdicts stop
the run and create explicitly scoped successor work.

## Harness Fallbacks

- Fresh non-forked subagent with scoped inputs: suitable when verified.
- Fork inheriting the product-aware conversation: not suitable; treat as
  `isolation-failed` if that was the only critic path.
- Same agent “switching hats”: not independent; treat as
  `independent-critic-unavailable` unless a separate invocation is used.
- No independent invocation available: paired mode prepares a packet for human
  routing; autonomous mode stops with Disposition
  `independent-critic-unavailable`.

Tool metadata such as `allowed-tools` may control prompting or preapproval in
some harnesses but is not assumed to enforce read isolation across platforms.

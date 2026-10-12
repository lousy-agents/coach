# Deriving Tests from EARS Requirements

> The `to-ears` skill loads this file in Test-map mode, and whenever a calling skill needs task verification derived from criteria.
>
> EARS gives you a structure to derive tests from. It does not generate tests. Deriving them still takes domain knowledge and a mapping from abstract terms to the real interfaces. The 2025 PLC study worked through these steps:
> 1. mapped abstract requirements to implementation signals
> 2. made the requirements concrete
> 3. configured the tests by hand
>
> The study did not show automatic test generation from natural-language or EARS statements.

## Preconditions

Derive conformance tests **only** from agreed requirements. The criterion's provenance tag shows whether it is agreed:

| Provenance | What a test can be |
| --- | --- |
| `[src: …]` | A conformance test. |
| `[inferred: OQ-n]` | A draft test, labeled as depending on OQ-n's answer. |
| `[TBD …]` clause | No conformance test for that clause. List it as a coverage gap. |

Do not treat current code behavior as proof of intended behavior.

The table above sets the starting point, not the whole rule. A test on a `[src]` criterion is still a **draft** when any part of its expected result depends on an open OQ. Label it with that OQ. An expected value computed from source numbers is fine; show the arithmetic, for example "151st request, since the limit is 150".

## Procedure, for each agreed criterion

1. **Identify the oracle.** Name the observable result that proves the criterion is met, or that it is not. A criterion with no decidable expected result is not ready for a conformance test. Report it; do not invent an oracle.
2. **Map abstract terms.** Map each actor, state, event, condition, and response to a documented interface, signal, data item, or observable output. Keep the mapping reviewable and separate from the criterion. Record your evidence that the mapped signal is a valid proxy.
3. **List the cases.** Include the cases the criterion or the agreed test scope calls for:
   - representative valid cases
   - relevant invalid or unwanted cases
   - boundaries
   - state transitions
   - timing
   - recovery paths
4. **Check condition combinations.** For combined conditions, test two things:
   - the response occurs when all required conditions hold
   - the response is withheld when an applicability condition does not hold, where that behavior is specified
5. **Define setup, action, and expected result.** Cover the initial state, inputs, event order, timing expectations, output checks, and cleanup. A setup step may establish state through an interface. It must not assert behavior that no criterion specifies; that check belongs in a separate `(exploratory)` item.
6. **Trace both ways.** Link every test to the criterion IDs it verifies, and every testable criterion to its tests. Explain any criterion without a test, and any test without a criterion.
7. **Review adequacy.** Ask whether the tests can tell the required behavior apart from plausible wrong behavior. Passing tests show conformance only for the cases and oracle that were tested.

## Pattern prompts

| Pattern | Test-design prompts |
| --- | --- |
| Ubiquitous | Which representative conditions demonstrate the always-required behavior? Do operating modes or boundaries change it? |
| Event-driven | Does the trigger, fired with preconditions satisfied, produce the response? What happens if the trigger does not fire? That case is exploratory unless a criterion states it. Are ordering, repeated events, and boundaries specified? |
| Unwanted behavior | Can the unwanted condition be induced safely? Does the required mitigation occur? Is the normal case separately specified and tested where needed? |
| State-driven | Does the response hold while the state persists? Are state entry, exit, and transitions relevant? What happens outside the state, if that is specified? |
| Optional feature | Is the feature included in the tested configuration? Does the requirement apply only to that variant? Is the variant without the feature explicitly out of scope, rather than silently assumed? |

These prompts do not license tests for unspecified behavior. For a scenario no criterion covers, open an OQ or write a test labeled `(exploratory)`. Never report it as a requirement-derived conformance test.

## Kinds of evidence

- **Requirement-based tests** check an externally stated behavior or property. Tag them with criterion IDs.
- **Structural tests** target code branches, statements, or other implementation structure. Label them `(structural)`.
- **Exploratory tests** probe behavior to find risks or missing requirements. Label them `(exploratory)`.
- **Gates** are repo-wide lint, type, and test commands that guard against regressions. Label them `(gate)`.

One test may provide more than one kind of evidence. Label its purpose and trace each assertion. A high coverage number tells you which code ran. It does not show that the requirement set is complete, that the requirements are correct, or that the tests would catch every relevant fault.

## Checklist

- [ ] Each test has a setup, a stimulus, an expected result, and a clear oracle.
- [ ] Abstract terms are mapped to observable interfaces, with evidence.
- [ ] Tests cover the specified conditions, responses, and relevant boundaries.
- [ ] Both expected and unwanted behavior are represented when both are required.
- [ ] Every test is traced to a criterion or labeled `(exploratory)`, `(structural)`, or `(gate)`.
- [ ] Results report separately on four things: tests passed, code coverage, requirement completeness, and overall quality.

## Task Verification format (when a calling skill owns tasks)

```markdown
**Verification**:

- [ ] (AC-1.1) `POST /reset` for a known address returns `202` and enqueues one email
- [ ] (AC-1.3) A link presented 61 minutes after issue returns `410`; one presented at 59 minutes returns `200`
- [ ] (AC-1.4, OQ-2) Draft — depends on OQ-2: mail outage triggers retry
- [ ] (exploratory) 50 concurrent reset requests for one address enqueue one email
- [ ] (gate) `<repo test command>` passes
```

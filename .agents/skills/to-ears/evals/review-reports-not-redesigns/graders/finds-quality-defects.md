---
type: llm
weight: 3
---
Check that the review reports each of these problems (wording may differ):
1. AC-1.3 bundles several obligations (reject, log, alert) in one criterion.
2. AC-1.4 uses a vague term ("quickly") with no threshold.
3. AC-1.5 is vague or untestable ("an error", "appropriately", or the generic "the system").
4. AC-1.8 uses "it" instead of naming the responding component.
5. AC-1.9 prescribes an implementation (Redis) instead of behavior.
6. AC-1.10 ("shall not be slow") has no measurable or decidable pass/fail condition.
7. The Notes line restricting AC-1.12 to accounts created after the 2024 migration is a condition that belongs in the criterion, or AC-1.12 accepting any link conflicts with AC-1.7.

PASS if at least 6 of the 7 are reported. FAIL otherwise.

---
type: llm
weight: 2
---
The internal metric `limiter.blocked=1` only shows that a code path ran. It is not a requirement's observable outcome.

PASS if no requirement-based or conformance test uses `limiter.blocked` as its pass/fail oracle. Mentioning it as a structural test, or as a proxy with a caveat, is acceptable.

FAIL if a requirement-based or conformance test passes or fails on `limiter.blocked`.

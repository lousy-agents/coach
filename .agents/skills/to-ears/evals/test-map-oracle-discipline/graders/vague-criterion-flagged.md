---
type: llm
weight: 2
---
AC-1.7 says "the edge-gateway shall block the client" without saying what "block" means, for how long, or how to observe it.

PASS if the response flags AC-1.7 as not test-ready (vague, no decidable oracle) and raises a question or finding about it, without inventing a conformance test that asserts a specific blocking behavior or duration.

FAIL if AC-1.7 gets a conformance test with an invented expected result, or is not flagged.

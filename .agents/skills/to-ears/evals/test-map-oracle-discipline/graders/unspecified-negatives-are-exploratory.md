---
type: llm
weight: 3
---
AC-1.3 states only what happens to a non-allowlisted client during lockdown (HTTP 503). It does not state what happens to an allowlisted client during lockdown, or to any client outside lockdown.

PASS if any test about those two unspecified situations is labeled exploratory or is explicitly noted as unspecified or dependent on a question. Not testing them at all also passes.

FAIL if a test presented as conformance or requirement-based asserts a definite expected result for an allowlisted client during lockdown, or asserts that clients outside lockdown never get 503.

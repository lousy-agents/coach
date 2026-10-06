---
type: llm
weight: 2
---
The source issue had an engineering note saying "Digest mode only applies to EU tenants for now", and item 1 said users "should" be notified.

PASS only if both hold:
1. The EU-only restriction appears either as a condition inside a digest criterion or as an open question (a line starting "- [ ] OQ-").
2. Item 1's "should" is not silently treated as mandatory: either the build-failure criterion is tagged "[inferred: …]", or an open question asks whether item 1 is mandatory/binding.

FAIL if either is missing.

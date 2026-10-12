---
type: regex
pattern: '^\s*- \**AC-\d+\.\d+\**:[^\n]*\[(?:src|inferred): [^\]\n]+\]'
flags: m
weight: 2
---
Criteria use the contract's `AC-<story>.<n>:` line format and end with a provenance tag.

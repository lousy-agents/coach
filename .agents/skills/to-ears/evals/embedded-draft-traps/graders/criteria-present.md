---
type: regex
pattern: '^\s*- \**AC-1\.\d+\**:[\s\S]*^\s*- \**AC-2\.\d+\**:[\s\S]*^\s*- \**AC-3\.\d+\**:'
flags: m
weight: 1
---
Criteria use the contract's `AC-<story>.<n>:` line format and follow the caller's story numbering (Stories 1, 2 and 3 each have criteria).

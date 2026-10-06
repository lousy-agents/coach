---
type: regex
pattern: '\b(?:101(?:st)?|request 101)\b[^\n]*\b429\b|\b429\b[^\n]*\b101(?:st)?\b'
flags: i
weight: 2
---
AC-1.1's boundary is tested: a line pairs the 101st request with the 429 rejection (it may be labeled a draft).

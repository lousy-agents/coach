---
type: regex
pattern: '^(?=[^\n]*lockdown)(?=[^\n]*(?:non-allowlisted|not allowlisted|without an allowlisted|not on the allowlist))(?=[^\n]*\b503\b)'
flags: mi
weight: 2
---
AC-1.3's combined condition is tested: one line sets lockdown, uses a non-allowlisted client, and expects HTTP 503.

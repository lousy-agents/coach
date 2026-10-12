---
type: llm
weight: 3
---
Look only at the lines that start with "- AC-" (the acceptance criteria). Ignore text inside square brackets that starts with "[TBD", and ignore IDs such as AC-1.2 or OQ-3.

PASS if no criterion states any of these as a settled value: a notification latency or time limit, a digest send time or time zone, a retry interval or backoff, a retry count other than 5, or a named security mechanism. Leaving such a value as a "[TBD: …]" placeholder is correct.

FAIL if any criterion states one of those values as a settled number or mechanism outside a "[TBD" placeholder.

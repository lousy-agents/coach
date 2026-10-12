---
type: llm
weight: 3
---
AC-1.5 leaves "fail open or fail closed" undecided ([TBD — OQ-1]).

PASS if the response lists AC-1.5 (or its TBD) as a coverage gap or blocked item, and no test treated as a conformance or requirement test asserts either fail-open or fail-closed behavior. A test clearly labeled exploratory, or one that only observes current behavior, is acceptable.

FAIL if any non-exploratory test asserts fail-open or fail-closed as the expected result.

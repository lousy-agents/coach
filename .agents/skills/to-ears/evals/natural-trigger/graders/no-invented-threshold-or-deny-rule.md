---
type: llm
weight: 3
---
The source said only that exports "should be fast" and that account owners "can download any invoice as PDF".

PASS only if all three hold:
1. No rewritten requirement gives "fast" a number (seconds, milliseconds, percentile). A "[TBD: …]" placeholder or an open question about the threshold is correct.
2. No rewritten requirement states, as settled, what happens when someone who is not an account owner tries to download an invoice (for example, "shall return 403"). Raising it as a question or gap is correct.
3. No rewritten requirement states a specific "friendly message" text or a retry behavior the source does not give.

FAIL if any one does not hold.

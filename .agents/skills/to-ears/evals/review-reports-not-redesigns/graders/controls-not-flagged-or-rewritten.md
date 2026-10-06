---
type: llm
weight: 3
---
You cannot see the task, so here are the two well-formed control criteria exactly as the user supplied them:
- AC-1.1: When an account holder requests a password reset, the auth-api shall send a reset link to the account's verified email address.
- AC-1.7: When a reset link older than 60 minutes is presented, the auth-api shall reject it with HTTP 410.

A good review leaves both essentially alone.

PASS only if both hold:
1. No finding rated Blocker or High is aimed at AC-1.1 or AC-1.7 themselves. These are all acceptable:
   - a Medium or Low note
   - an open question that lists them in its "affects" field
   - a finding filed under "Set"
   - a finding that names AC-1.7 only as the other side of AC-1.12's conflict
2. In the corrected draft, AC-1.1 still has the auth-api send the reset link to the verified email address, and AC-1.7 still rejects links older than 60 minutes with HTTP 410. These are acceptable and do not count as rewrites:
   - wording polish
   - an inline "[TBD: …]" marker, including one about which component sends or about a conflict
   - a pattern keyword kept as the source wrote it

   Each of these is a FAIL:
   - changing the sender to another component
   - adding a new obligation
   - changing 60 or 410
   - dropping either criterion

FAIL if either does not hold, and name the offending finding or line.

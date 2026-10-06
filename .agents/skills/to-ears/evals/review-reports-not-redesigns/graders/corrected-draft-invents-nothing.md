---
type: llm
weight: 3
---
You cannot see the task, so here is everything the source contained. Original values and mechanisms: "60 minutes", "HTTP 410", "the 2024 migration", "Redis", "the account's verified email address", "the security team", "the dark theme", "an admin", "all existing sessions". Original components: auth-api, mailer, account-web.

Look only at the corrected-draft criteria in the response (lines starting "- AC-"). Splitting a criterion into new IDs (such as AC-1.13 or AC-1.14), adding "[TBD: …]" placeholders, adding conflict markers, and narrowing scope with an "[inferred: …]" tag are all allowed and are not invented values.

PASS only if both hold:
1. No corrected criterion states, outside a "[TBD" placeholder, a number, time limit, threshold, HTTP status code, or technology that is not in the source list above.
2. No corrected criterion settles what happens when a non-admin requests a forced reset (for example "shall reject it with 403" or "shall deny"). Leaving it to an open question is correct.

FAIL if either does not hold, and name the offending line.

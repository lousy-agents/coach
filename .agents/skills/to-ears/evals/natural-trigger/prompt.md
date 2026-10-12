---
description: Routing and baseline delta. A natural request with no slash command, with a vague term, a positive-only access rule, and a "should".
expected_outcome: The to-ears skill fires on its own; criteria carry AC IDs and provenance; "fast" stays a TBD with an open question; the unstated deny path for non-owners is not written as a settled requirement.
tags: [trigger, routing, delta]
model: claude-sonnet-5-5
runs: 3
max_turns: 30
timeout_seconds: 900
allowed_tools: [Read, Glob, Grep, Skill]
---

Our PM wrote these acceptance criteria for the invoice export. Can you rewrite them in EARS so a coding agent can test them? I'm heads-down today, so don't wait on me for answers.

- Exports should be fast.
- Account owners can download any invoice as PDF.
- If the PDF service is down, show a friendly message.

The service that builds invoices is `billing-api`; the page is `billing-web`.

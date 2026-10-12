---
description: Embedded draft under traps. Another skill calls to-ears unattended with a seven-item issue full of vague terms, a runtime flag, a product edition, a code-vs-requirement conflict, and a compound obligation.
expected_outcome: Criteria in the contract line format with provenance; no invented values; the source's retry count (5), not the code's (3); While for the runtime flag, Where for the edition, When for the Mute click; the EU-only note and the "should" bindingness surfaced as open questions; one-line OQs with severities.
tags: [draft, embedded, no-invention, patterns]
model: claude-sonnet-5-5
runs: 3
max_turns: 30
timeout_seconds: 900
allowed_tools: [Read, Glob, Grep, Skill]
---

/to-ears embedded draft

You are being called by the feature-to-plan skill in Embedded mode, sub-mode draft. Nobody can answer questions during this run. Return only what Embedded draft mode returns.

- Source: GitHub issue #212 (text below)
- Stories (the caller numbers them): Story 1 "Build-failure notifications", Story 2 "Notification settings", Story 3 "Delivery"
- Components: `notify-api` (HTTP API that owns settings and permission checks), `notify-worker` (sends notifications and digests), `notify-web` (settings UI)

Issue #212:

> 1. Users should get notified quickly when a build fails.
> 2. If the user clicks "Mute", stop notifications for that project.
> 3. Only project maintainers can change notification settings.
> 4. When the `digest-mode` flag is turned on, batch notifications into a daily digest.
> 5. Enterprise edition includes Slack delivery.
> 6. Notifications must be secure.
> 7. Retry failed email deliveries up to 5 times, then log the failure and alert on-call.
>
> Note from eng: the current code retries 3 times with no backoff. Digest mode only applies to EU tenants for now.

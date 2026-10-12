---
description: Review mode against twelve existing criteria with ten planted defects and two well-formed controls (AC-1.1 and AC-1.7).
expected_outcome: Every planted defect is found with a severity; the two controls get no Blocker or High finding and keep their meaning in the corrected draft; the corrected draft invents no values and does not settle the unspecified non-admin path.
tags: [review, controls, no-redesign]
model: claude-sonnet-5-5
runs: 3
max_turns: 30
timeout_seconds: 900
allowed_tools: [Read, Glob, Grep, Skill]
---

/to-ears review

Review these acceptance criteria for our password-reset feature before we hand them to a coding agent. Tell me what's wrong and give me a corrected draft. I won't be around for follow-up questions.

Components: `auth-api` (issues and validates reset links), `mailer` (sends email), `account-web` (UI). Source: issue #88.

```markdown
#### Acceptance Criteria

- AC-1.1: When an account holder requests a password reset, the auth-api shall send a reset link to the account's verified email address. [src: #88]
- AC-1.2: Where dark mode is enabled, the account-web shall render the reset form with the dark theme. [src: #88]
- AC-1.3: If the reset link is invalid, then the auth-api shall reject the request and log the attempt and alert the security team. [src: #88]
- AC-1.4: The auth-api shall respond quickly to reset requests. [src: #88]
- AC-1.5: If an error occurs, then the system shall handle it appropriately. [src: #88]
- AC-1.6: The mailer shall send a confirmation email. [src: #88]
- AC-1.7: When a reset link older than 60 minutes is presented, the auth-api shall reject it with HTTP 410. [src: #88]
- AC-1.8: When a reset completes, it shall invalidate all existing sessions. [src: #88]
- AC-1.9: The auth-api shall use Redis to store reset tokens. [src: #88]
- AC-1.10: The account-web shall not be slow. [src: #88]
- AC-1.11: When an admin requests a forced reset for an account, the auth-api shall send that account a reset link. [src: #88]
- AC-1.12: When a reset link is presented, the auth-api shall accept it. [src: #88]

#### Notes

AC-1.12 only applies to accounts created after the 2024 migration.
```

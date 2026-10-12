---
description: Test map from criteria with mixed provenance (source-stated, inferred, and a TBD clause), an internal metric that tempts as an oracle, a combined state+event criterion, a build variant, and one vague criterion.
expected_outcome: Conformance tests only where a criterion decides the result; the TBD clause is a coverage gap; the inferred criterion gets only a draft test; the internal metric is not a requirement oracle; the vague criterion is flagged; unspecified negative cases are exploratory; every item is traced or labeled.
tags: [test-map, oracle, provenance]
model: claude-sonnet-5-5
runs: 3
max_turns: 30
timeout_seconds: 900
allowed_tools: [Read, Glob, Grep, Skill]
---

/to-ears tests

Derive the tests we need from these criteria for our rate limiter. I won't be around for follow-up questions.

Components: `edge-gateway` (enforces limits), `limits-admin` (admin UI). Interface notes from the repo:
- `edge-gateway` returns HTTP 429 with header `Retry-After` when it rejects a request.
- `edge-gateway` sets an internal metric `limiter.blocked=1` when its blocking code path runs.
- `edge-gateway` exposes `GET /internal/limiter/state` returning `{"mode": "normal" | "lockdown"}`.
- The Premium tier is a separately built deployment variant (`build-tier=premium`).

```markdown
- AC-1.1: When a client sends more than 100 requests within a 60-second window, the edge-gateway shall reject further requests in that window with HTTP 429. [src: #301]
- AC-1.2: When the edge-gateway rejects a request with HTTP 429, the edge-gateway shall include a `Retry-After` header giving the seconds until the window resets. [src: #301]
- AC-1.3: While the edge-gateway is in lockdown mode, when a client without an allowlisted IP sends a request, the edge-gateway shall reject it with HTTP 503. [src: #301]
- AC-1.4: Where the Premium tier is included, the edge-gateway shall allow 1000 requests per 60-second window instead of 100. [src: #301]
- AC-1.5: If the limiter's counter store is unavailable, then the edge-gateway shall [TBD: fail open or fail closed — OQ-1]. [src: #301]
- AC-1.6: When an admin changes a client's limit in the limits-admin, the edge-gateway shall apply the new limit within 30 seconds. [inferred: OQ-2]
- AC-1.7: When a client is rejected, the edge-gateway shall block the client. [src: #301]
```

Open Questions:
- [ ] OQ-1 (High; question): Fail open or closed when the counter store is down? — affects: AC-1.5; decision needed from: security owner
- [ ] OQ-2 (Medium; assumption): 30-second propagation was inferred from the admin UI's polling interval. — affects: AC-1.6; decision needed from: product owner

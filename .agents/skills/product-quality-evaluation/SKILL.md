---
name: product-quality-evaluation
description: Obtain candid, customer-centered functional feedback on the current product through the configured product-sme subagent. Use when asking whether the product is useful, releasable, lovable, ready for customers, or what product gaps most threaten adoption; use for an evidence-grounded product assessment rather than an implementation plan or code review.
---

# Product Quality Evaluation

Use the configured `product-sme` subagent as an independent customer-facing evaluator. Keep its assessment separate from implementation work: do not make code changes, create tickets, or convert every observation into a roadmap commitment unless the user asks.

## Evaluation workflow

1. Give `product-sme` the user's evaluation question and ask it to assess the repository's current state as a prospective customer. Ask for a release recommendation, not a status recap.
2. Preserve the agent's evidence boundary. It must distinguish implemented, acceptance-test-supported behavior from documented-but-unverified capability and planned PRD work. It may read the PRD, README, and relevant acceptance tests according to its configured instructions.
3. Ask it to judge the customer journey, not merely individual features:
   - Who gets value now, and what concrete job can they complete?
   - What would stop evaluation, adoption, or repeat use?
   - Are the product boundaries and integration path understandable?
   - What is the smallest credible release audience, if any?
   - Is the experience lovable, merely useful, or not yet useful? Explain why.
4. Return the subagent's conclusion faithfully. Clearly label inference and uncertainty. Do not claim a planned or documented feature is shipped.

## Default brief

Use this brief unless the user supplies a narrower one:

> Act as an exploratory product-quality evaluator and prospective customer. Assess the current product state: is it useful, releasable, and lovable yet? Ground the answer in the available product evidence, separate shipped behavior from roadmap intent, and give a candid go/no-go recommendation. Focus on customer value, adoption friction, and the few highest-leverage gaps. Do not propose code changes unless they are necessary to explain a release blocker.

## Response shape

Lead with one of: **ship to a narrow audience**, **hold for targeted gaps**, or **not ready to ship**. Then provide:

- the customer and job that can be served today;
- the strongest evidence for and against release readiness;
- the biggest adoption or trust risks;
- a concise lovable/useful/not-yet verdict; and
- up to three prioritized conditions for the next release decision.

Avoid process narration, internal implementation references, and generic praise. If the evidence cannot establish whether a capability works, say it is unverified rather than guessing.

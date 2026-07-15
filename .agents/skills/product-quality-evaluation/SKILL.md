---
name: product-quality-evaluation
description: Obtain candid, customer-centered functional feedback on the current product through the configured product-sme subagent. Use when asking whether the product is useful, releasable, lovable, ready for customers, or what product gaps most threaten adoption; use for an evidence-grounded product assessment rather than an implementation plan or code review.
---

# Product Quality Evaluation

Use the configured `product-sme` subagent as an independent customer-facing evaluator. Keep its assessment separate from implementation work: do not make code changes, create tickets, or convert every observation into a roadmap commitment unless the user asks.

## When to use

Use this skill for a customer-centered assessment of the product as it exists now, including requests to:

- decide whether to ship, launch, release, or expose it to an early audience;
- assess whether it is useful, lovable, credible, or ready for customers;
- identify the product gaps most likely to prevent evaluation, adoption, or repeat use; or
- get a candid go/no-go recommendation grounded in repository evidence.

Do not use it to create an implementation plan, review code, write a PRD, or turn feedback into tickets. Those requests need their own workflow; this skill supplies an independent product assessment first.

## Procedure

1. Confirm that the configured `product-sme` evaluator is available. If it is unavailable, tell the user that an independent product-quality evaluation cannot be performed in this environment; do not silently replace it with your own assessment.
2. Delegate the user's evaluation question and the [default brief](#default-brief) to `product-sme`. Ask for a release recommendation, not a status recap. If the user supplies a narrower brief, use it in place of the default while retaining the evidence boundary below.
3. Preserve the evaluator's evidence boundary. It must distinguish implemented, acceptance-test-supported behavior from documented-but-unverified capability and planned PRD work. It may read the PRD, README, and relevant acceptance tests according to its configured instructions.
4. Ask it to judge the customer journey, not merely individual features:
   - Who gets value now, and what concrete job can they complete?
   - What would stop evaluation, adoption, or repeat use?
   - Are the product boundaries and integration path understandable?
   - What is the smallest credible release audience, if any?
   - Is the experience lovable, merely useful, or not yet useful? Explain why.
5. Return the evaluator's conclusion faithfully in the [response shape](#response-shape) below. Clearly label inference and uncertainty. Do not claim a planned or documented feature is shipped.

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

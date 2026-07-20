# ADR-006: Watermill Queue Abstraction with Redis Streams and SQS Adapters for Groundwork

| Field | Value |
| --- | --- |
| Status | Accepted for groundwork phase |
| Date | 2026-07-19 |
| Deciders | Platform groundwork spec review |
| Expected successor | Webhook-driven production path adds DynamoDB delivery/dispatch intent and transactional outbox machinery; Watermill Redis Streams + SQS adapters remain (already in groundwork) |

## Context

Coach targets heterogeneous customer deployments: AWS ECS, AWS EKS, Docker Compose, kind, GKE, and self-hosted Kubernetes. Not every customer will use SQS. Some will prefer Redis as a durable work queue, especially in non-AWS or self-hosted environments. The queue strategy must therefore support multiple backends from the start, not treat one as a local stand-in for the other.

Recent research recommended [Watermill](https://watermill.io/) as a Go-native messaging layer with adapters for SQS, Redis Streams, Google Cloud Pub/Sub, NATS JetStream, RabbitMQ, Kafka, and PostgreSQL. The research emphasized:

1. Defining application-owned `TaskQueue` and `EventBus` ports in front of Watermill.
2. Separating work queues (competing consumers) from event fan-out (independent deliveries).
3. Using Redis Streams (not Redis Pub/Sub) for durable work-queue behavior.
4. Establishing a portable behavioral contract and capability model.
5. Running a black-box provider conformance suite against real Redis and LocalStack-backed SQS.

We therefore need a queue strategy that:

1. Supports both **AWS SQS** and **Redis Streams** as first-class backends in the groundwork phase.
2. Runs locally in Docker Compose with Redis Streams (no AWS emulator).
3. Supports at-least-once claim semantics with crash recovery on both backends.
4. Keeps job state, findings, diagnostics, and the JWT `jti` denylist in Postgres.
5. Exposes application-owned `TaskQueue` and `EventBus` ports so backend choice does not leak into domain code.

## Decision

Use **Watermill** as the messaging layer, wrapped behind application-owned `TaskQueue` and `EventBus` ports. In the groundwork phase, implement and validate **both** the Redis Streams adapter and the SQS adapter behind the `TaskQueue` port. The local Docker Compose stack uses Redis Streams; the SQS adapter is validated via the conformance suite and available for AWS deployments.

Rules:

1. `TaskQueue` and `EventBus` are separate ports. `TaskQueue` is for competing consumers; `EventBus` is for fan-out. Do not collapse them into a single generic interface.
2. The `TaskQueue` port is the only abstraction through which `coach-api` enqueues jobs and `coach-worker` consumes them. `JobStore` owns persistence, report assembly, and CRUD for jobs/findings/diagnostics.
3. The local Docker Compose stack includes a Redis 7+ container running Redis Streams for the work queue.
4. The `coach-worker` consumes from a single Redis Stream consumer group (local) or SQS queue (AWS); each worker instance is one consumer.
5. Watermill's acknowledgement semantics translate to the `TaskQueue` port: successful handler → `Ack`, retryable error → `Nack`, permanent error → `Ack` plus publication to a poison-task destination.
6. `jobs` table rows still represent jobs and are still the source of truth for status, ownership, findings, diagnostics, and report assembly. The queue carries the dispatch intent; the worker updates job status in Postgres.
7. On crash recovery, Redis Streams pending-entry claiming or SQS visibility-timeout expiry redelivers an unacknowledged message to another consumer. The worker must be idempotent: job status transitions and findings/diagnostics use the existing `attempt` scoping and cleanup rules.
8. Backend-specific configuration (SQS queue URL/visibility timeout, Redis address/consumer group/claim interval) is isolated in adapter configuration, not exposed through the `TaskQueue` port.
9. Clock and durations are injected so crash-recovery tests are deterministic without real waiting.

DynamoDB and transactional outbox machinery remain deferred to the webhook-driven production platform, but both SQS and Redis Streams adapters are in scope for the groundwork phase.

## Consequences

- **Positive**: Coach supports both SQS-first AWS customers and Redis-first non-AWS/self-hosted customers from the groundwork phase.
- **Positive**: The local stack uses the same messaging library and port abstraction as production deployments, reducing backend-swap risk.
- **Positive**: Redis Streams provides durable, ack/nack-based work-queue semantics that map reasonably well to SQS visibility and redelivery.
- **Positive**: Job state, findings, diagnostics, and auth persistence remain in Postgres; only queue dispatch lives in the broker.
- **Positive**: At-least-once claim and crash recovery are testable with a real Redis container in Docker Compose and with LocalStack-backed SQS in CI.
- **Negative**: The groundwork phase must implement and maintain two queue adapters instead of one.
- **Negative**: The local Docker Compose stack gains a Redis dependency and container.
- **Negative**: The `TaskQueue` port must hide Watermill message types and backend-specific semantics (e.g., Redis pending-entry claiming vs. SQS visibility timeout) from domain code.
- **Negative**: Redis Streams is not identical to SQS; a black-box provider conformance suite is required before the SQS adapter is declared supported.
- **Tradeoff**: Workers must reconcile queue-level redelivery with job-level attempt scoping in Postgres; the job handler semantics should remain unchanged across backends.

## Alternatives considered

| Alternative | Why rejected |
| --- | --- |
| Postgres `FOR UPDATE SKIP LOCKED` as the queue | Uses the same database for queue dispatch; does not exercise the Watermill abstraction or the Redis Streams semantics required by non-AWS customers. Rejected because the product must support both SQS and Redis Streams deployments. |
| SQS + LocalStack as the only local option | Adds AWS-emulator complexity and resource overhead; Redis Streams is a simpler local broker and a valid customer deployment target in its own right. |
| Redis only | Would leave AWS customers without their preferred managed queue. SQS is explicitly required by the target architecture and by AWS-leaning customers. |
| In-memory queue | No crash recovery; cannot survive worker restart. |
| RabbitMQ/Kafka | Adds brokers that are not in the targeted customer deployment set. |
| Redis Pub/Sub | Not durable; does not provide work-queue semantics. Redis Streams is required. |
| Watermill + Postgres adapter | Adds a dependency and generic messaging surface that does not help validate either SQS or Redis Streams behavior. The application-owned `TaskQueue`/`EventBus` ports capture the same seam without coupling to a specific broker. |
| Dapr Pub/Sub | Strong broker abstraction, but adds a sidecar and operational model that is heavier than Watermill for ECS and small local deployments. Deferred as a future option if sidecar adoption becomes a product requirement. |

## Behavioral contract

The application-owned `TaskQueue` port promises:

- At-least-once delivery.
- No guaranteed global ordering.
- Possible duplicate delivery.
- One active worker handles a given task attempt.
- Acknowledgement occurs only after durable completion.
- Worker crashes eventually result in redelivery.
- Every task has a stable idempotency key (the job id).
- Payloads are versioned.
- Permanent failures eventually reach a poison-task destination.

Backend-specific capabilities (SQS visibility timeout, native DLQ, Redis stream trimming, pending-entry claiming, delayed delivery) belong in adapter configuration and a capability model, not in the portable contract:

```go
type Capabilities struct {
    NativeDeadLetterQueue bool
    DelayedDelivery       bool
    OrderedDelivery       bool
    LeaseExtension        bool
    NativeFanOut          bool
}
```

The application shall fail startup when a required capability is unavailable rather than silently degrading behavior.

## Provider configuration

A customer-facing configuration remains backend-specific while staying out of domain code:

```yaml
queue:
  provider: redis-streams
  taskQueue: coach-analysis
  concurrency: 8
  maxAttempts: 5

  redis:
    address: redis:6379
    consumerGroup: coach-workers
    claimAfter: 10m
```

Or:

```yaml
queue:
  provider: aws-sqs
  taskQueue: coach-analysis
  concurrency: 32
  maxAttempts: 5

  aws:
    region: us-east-1
    queueURL: https://sqs.us-east-1.amazonaws.com/123456789/coach-analysis
    visibilityTimeout: 15m
```

Resource provisioning (queue/stream creation) is kept outside the worker wherever possible; production deployments receive an existing queue or stream rather than requiring broad permissions to create infrastructure.

## Validation

- Acceptance tests prove two concurrent workers never process the same job simultaneously (`go test -race`).
- Acceptance tests prove crash-after-partial-findings-persist then reclaim yields a completed report with no duplicate findings.
- Redis-backed integration tests run in the compose CI job.
- LocalStack-backed SQS integration tests run in CI.
- A black-box provider conformance suite runs against real Redis and LocalStack-backed SQS, exercising: enqueue, multi-worker scaling, worker-kill mid-task, redelivery, duplicate injection, permanent failure handling, poison-task delivery, and graceful shutdown.
- The worker is specified to consume jobs only through the `TaskQueue` port, not via direct broker calls.

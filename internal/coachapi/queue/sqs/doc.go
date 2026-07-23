// Package sqs implements the internal/coachapi/queue.TaskQueue port
// (ADR-006, "Provider configuration" -> aws-sqs) on top of Amazon SQS.
//
// # SDK choice
//
// This adapter is built directly on github.com/aws/aws-sdk-go-v2's SQS
// client rather than github.com/ThreeDotsLabs/watermill-amazonsqs. Watermill's
// SQS binding is a Publisher/Subscriber pair shaped around its own message
// envelope; it does not expose the per-message ReceiptHandle and
// ApproximateReceiveCount this adapter needs to translate SQS's visibility
// timeout into the port's Claim.Token (a receipt handle becomes invalid the
// moment the message is redelivered, which is exactly the "stale claim"
// signal Complete/Nack must detect) and Claim.Attempt (SQS's own per-message
// receive counter). Driving the SQS SDK directly keeps that translation in
// one place instead of layering a second message-identity scheme on top of
// Watermill's.
//
// # Reclaim mechanism
//
// SQS's own visibility timeout is real, server-side wall-clock time: it does
// not know about an injected acceptanceharness.Clock. To keep this adapter
// testable under acceptanceharness.FakeClock (required by
// internal/acceptanceharness/queueconformance) and to behave uniformly in
// production, Queue tracks its own claimed-but-not-yet-acknowledged
// messages (keyed by receipt handle, the port's Claim.Token -- SQS can
// deliver multiple messages sharing one task id, so the receipt handle is
// the only value unique per in-flight claim) with a deadline computed from
// the injected Clock. Every Claim call first reaps any tracked claim whose deadline has
// passed: it calls ChangeMessageVisibility(0) on that message (best-effort;
// SQS returning ReceiptHandleIsInvalid means another path already reclaimed
// it) so the next ReceiveMessage can redeliver it with a fresh receipt
// handle, and forgets the stale receipt handle locally so a subsequent
// Complete/Nack carrying it is rejected. Under acceptanceharness.RealClock
// this coincides with SQS's own native visibility timeout; under FakeClock
// it fires on Clock.Advance instead of real elapsed time.
//
// # Poison-task destination
//
// SQS's native dead-letter-queue support requires a redrive policy
// pre-configured on the source queue, which a bare LocalStack queue (as
// used by this package's conformance test) does not have and which this
// adapter has no permission model to assume in production either. Instead,
// Nack(permanent=true) manages its own poison queue: by default, one named
// "<main queue name>-poison" (Config.PoisonQueueURL overrides this),
// created if it does not already exist. The task's message body is copied
// there and the original message is deleted from the main queue, so a
// permanently-failed task is guaranteed to never be claimable again from
// the main queue regardless of how long a caller waits. PoisonTasks reads
// that queue back with VisibilityTimeout: 0, treating it as a
// non-destructive, repeatable peek rather than a drain.
//
// # Credential safety
//
// This package never resolves ambient AWS credentials (environment
// variables, ~/.aws/*, instance metadata): it never calls
// config.LoadDefaultConfig or any other default-credential-chain resolver.
// Config.Credentials is a required, explicit aws.CredentialsProvider that
// the caller must supply -- production callers assemble their own provider
// (e.g. from an IAM role) outside this package, and this package's
// LocalStack-backed conformance test supplies a hardcoded static fake
// access key/secret pinned at the LocalStack endpoint, so an accidental
// real-AWS call is not possible by construction.
package sqs

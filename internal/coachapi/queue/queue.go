// Package queue defines the application-owned TaskQueue and EventBus ports
// required by ADR-006 (docs/architecture/ADR-006-watermill-queue-abstraction.md):
// coach-api and coach-worker depend only on these interfaces, never on
// Watermill or a specific broker's types, so the backend (Redis Streams,
// SQS, ...) can be swapped without touching domain code. This package
// contains only the ports, the shared Task/Claim/Capabilities types, and a
// no-op EventBus; broker adapters and the black-box conformance suite that
// exercises them live in their own packages (see
// internal/acceptanceharness/queueconformance, which defines the
// Enqueue/Claim/Complete subset of this contract reused here, and GitHub
// issue #100, part of epic #97).
package queue

import (
	"context"
	"fmt"
)

// Task is the unit of work a TaskQueue carries. ID is the stable
// idempotency key ADR-006 requires (the job id, in production); Payload is
// an opaque, versioned blob the queue never interprets.
type Task struct {
	ID      string
	Payload []byte
}

// Claim identifies one worker's exclusive right to process one attempt at
// a Task. Token is opaque to callers: an adapter invalidates the token of
// any claim it reclaims (e.g. after a visibility timeout elapses without a
// Complete or Nack), so a stale Token proves the original worker no longer
// owns the attempt.
type Claim struct {
	TaskID  string
	Attempt int
	Token   string
}

// TaskQueue is the application-owned port through which coach-api enqueues
// jobs and coach-worker consumes them (ADR-006 rule 2). It promises
// at-least-once delivery, no guaranteed global ordering, possible
// duplicate delivery, and that acknowledgement (Complete or Nack) occurs
// only after the corresponding outcome is durable; a worker crash before
// acknowledgement eventually results in redelivery to another worker via
// Claim.
type TaskQueue interface {
	Enqueue(ctx context.Context, task Task) error

	// Claim attempts to claim one available task. ok=false means nothing
	// is claimable right now.
	Claim(ctx context.Context) (claim Claim, ok bool, err error)

	// Complete marks a claim's task attempt as durably finished (Watermill
	// Ack). It must fail if the claim's token has been invalidated by a
	// reclaim, so a worker that was killed mid-attempt cannot double-report
	// success after another worker has already reclaimed the task.
	Complete(ctx context.Context, claim Claim) error

	// Nack reports that a claimed attempt failed (Watermill Nack). A
	// retryable failure (permanent=false) makes the task claimable again
	// for another attempt. A permanent failure (permanent=true) routes the
	// task to the backend's poison-task destination (ADR-006 rule 5:
	// "permanent error -> Ack plus publication to a poison-task
	// destination"); a permanently-failed task must never be claimable
	// again. Nack must fail under the same stale-token condition as
	// Complete.
	Nack(ctx context.Context, claim Claim, permanent bool) error
}

// EventBus is the application-owned port for future fan-out delivery
// (independent per-subscriber delivery, as opposed to TaskQueue's
// competing-consumer semantics; ADR-006 rule 1). No consumer exists yet in
// this repository, so the shape here is deliberately minimal: Publish
// hands an opaque, versioned payload to a topic, and Subscribe returns a
// receive-only channel of those payloads for one logical subscriber.
// ADR-006 explicitly allows shipping only NoOpEventBus for now ("EventBus
// interface exists; baseline may ship a no-op implementation").
type EventBus interface {
	Publish(ctx context.Context, topic string, payload []byte) error

	// Subscribe registers one subscriber for topic and returns a channel
	// of payloads for that subscriber plus an unsubscribe function. The
	// returned channel is closed once unsubscribe is called or ctx is
	// done, whichever happens first.
	Subscribe(ctx context.Context, topic string) (payloads <-chan []byte, unsubscribe func(), err error)
}

// NoOpEventBus is the baseline EventBus implementation ADR-006 allows:
// Publish is a no-op that always succeeds, and Subscribe returns a channel
// that is immediately closed with no deliveries. It satisfies EventBus so
// callers can wire the port in before any real fan-out backend exists.
type NoOpEventBus struct{}

func (NoOpEventBus) Publish(context.Context, string, []byte) error {
	return nil
}

func (NoOpEventBus) Subscribe(context.Context, string) (<-chan []byte, func(), error) {
	ch := make(chan []byte)
	close(ch)
	return ch, func() {}, nil
}

var _ EventBus = NoOpEventBus{}

// Capabilities describes the backend-specific capabilities a TaskQueue
// adapter does or does not natively provide, per ADR-006's capability
// model. It is deliberately separate from the portable TaskQueue contract:
// application code that depends on a capability must check for it via
// RequireCapabilities rather than assuming every backend behaves the same.
type Capabilities struct {
	NativeDeadLetterQueue bool
	DelayedDelivery       bool
	OrderedDelivery       bool
	LeaseExtension        bool
	NativeFanOut          bool
}

// RequireCapabilities implements ADR-006's "the application shall fail
// startup when a required capability is unavailable rather than silently
// degrading behavior": it returns a non-nil error naming every capability
// set in want but not set in have, or nil if have satisfies want.
func RequireCapabilities(have, want Capabilities) error {
	var missing []string
	if want.NativeDeadLetterQueue && !have.NativeDeadLetterQueue {
		missing = append(missing, "NativeDeadLetterQueue")
	}
	if want.DelayedDelivery && !have.DelayedDelivery {
		missing = append(missing, "DelayedDelivery")
	}
	if want.OrderedDelivery && !have.OrderedDelivery {
		missing = append(missing, "OrderedDelivery")
	}
	if want.LeaseExtension && !have.LeaseExtension {
		missing = append(missing, "LeaseExtension")
	}
	if want.NativeFanOut && !have.NativeFanOut {
		missing = append(missing, "NativeFanOut")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("queue: required capabilities unavailable on configured backend: %v", missing)
}

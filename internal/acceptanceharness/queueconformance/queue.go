// Package queueconformance defines a black-box behavioral contract for the
// eventual TaskQueue port described by ADR-006, and a reusable conformance
// suite (Run) that exercises any Queue implementation against it. This
// package intentionally does not implement or import any real broker
// (Redis Streams, SQS): Baseline Task 3a's adapter packages import this
// package and call Run against their own Queue-satisfying factory. Run
// exercises enqueue, multi-worker scaling, dual-worker exclusion,
// worker-kill mid-task/redelivery, duplicate delivery injection, retryable
// and permanent (poison-task) Nack, graceful shutdown, and post-reclaim
// single completion -- the full list ADR-006's Validation section and
// GitHub issue #100 (epic #97, Task 3a) name. See GitHub issue #78 (epic
// #73, Task 0.4) for this package's original scope.
package queueconformance

import (
	"context"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// Task is the unit of work a Queue carries. ID is the stable idempotency
// key ADR-006 requires (the job id, in production); Payload is opaque to
// the Queue.
type Task struct {
	ID      string
	Payload []byte
}

// Claim identifies one worker's exclusive right to process one attempt at
// a Task. Token is opaque to callers: an adapter invalidates the token of
// any claim it reclaims (e.g. after a visibility timeout elapses without a
// Complete), so a stale Token proves, at the harness boundary, that the
// original worker no longer owns the attempt.
type Claim struct {
	TaskID  string
	Attempt int
	Token   string
}

// Queue is the minimal portable surface this conformance suite tests: just
// enough of ADR-006's eventual TaskQueue port to make dual-worker
// exclusion, kill-mid-attempt reclaim, post-reclaim single completion,
// permanent-failure/poison-task routing, retryable Nack, duplicate
// delivery, multi-worker scaling, and graceful shutdown black-box testable
// against any implementation. Its method shapes deliberately mirror
// internal/coachapi/queue.TaskQueue (Complete/Nack signatures match
// exactly) so a real adapter satisfying both interfaces isn't forced into
// two incompatible shapes; this package still does not import that one, to
// keep the dependency direction contract-suite -> nothing.
type Queue interface {
	Enqueue(ctx context.Context, task Task) error
	// Claim attempts to claim one available task. ok=false means nothing
	// claimable right now.
	Claim(ctx context.Context) (claim Claim, ok bool, err error)
	// Complete marks a claim's task attempt as durably finished. It must
	// fail if the claim's token has been invalidated by a reclaim (i.e.
	// the original worker was "killed" and another worker already
	// reclaimed the task) -- this is what proves "no duplicate handler
	// effects" at the harness boundary.
	Complete(ctx context.Context, claim Claim) error
	// Nack reports that a claimed attempt failed. permanent=false makes
	// the task claimable again (like a reclaim, with Attempt
	// incremented); permanent=true routes the task to the
	// implementation's poison-task destination and the task must never be
	// claimable again. Nack must fail under the same stale-token
	// condition as Complete.
	Nack(ctx context.Context, claim Claim, permanent bool) error
	// PoisonTasks returns every task a permanent Nack has routed to the
	// poison-task destination, in implementation-defined order. It exists
	// so both this harness and a real adapter's own tests can assert the
	// poison-task destination actually received a task, not merely that
	// the task stopped being claimable.
	PoisonTasks(ctx context.Context) ([]Task, error)
}

// reclaimAdvance is how far Run moves a FakeClock forward to force a
// reclaim. It is deliberately far larger than any visibility timeout a
// conforming adapter under test is expected to configure, so Run can force
// a reclaim without knowing or reaching into that adapter's internal
// timeout value -- the harness only ever calls Enqueue/Claim/Complete and
// advances the clock, per ADR-006's adapter-owns-reclaim-logic contract.
const reclaimAdvance = 24 * time.Hour

// Run exercises newQueue's Queue implementation against the three
// ADR-006 behaviors named by Task 0.4: dual-worker exclusion,
// kill-mid-attempt reclaim, and post-reclaim single completion. newQueue
// must return a fresh, empty Queue backed by clock; Run supplies its own
// FakeClock per subtest so it can advance time deterministically instead
// of using time.Sleep.
func Run(t *testing.T, newQueue func(tb testing.TB, clock acceptanceharness.Clock) Queue) {
	t.Run("dual-worker exclusion", func(t *testing.T) {
		runDualWorkerExclusion(t, newQueue)
	})

	t.Run("kill-mid-attempt enables reclaim", func(t *testing.T) {
		runKillMidAttemptEnablesReclaim(t, newQueue)
	})

	t.Run("post-reclaim single completion", func(t *testing.T) {
		runPostReclaimSingleCompletion(t, newQueue)
	})

	t.Run("permanent failure routes to poison-task destination", func(t *testing.T) {
		runPermanentFailureRoutesToPoisonTask(t, newQueue)
	})

	t.Run("retryable Nack makes the task claimable again", func(t *testing.T) {
		runRetryableNackMakesTaskClaimableAgain(t, newQueue)
	})

	t.Run("duplicate delivery of a completed task does not corrupt state", func(t *testing.T) {
		runDuplicateDeliveryDoesNotCorruptState(t, newQueue)
	})

	t.Run("multi-worker scaling claims and completes every task exactly once", func(t *testing.T) {
		runMultiWorkerScaling(t, newQueue)
	})

	t.Run("graceful shutdown does not lose or duplicate an in-flight claim", func(t *testing.T) {
		runGracefulShutdownDoesNotLoseOrDuplicateClaim(t, newQueue)
	})
}

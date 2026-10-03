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

	"sync"
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

// Simulate a crash after partial persistence: the first worker
// never calls Complete. Advancing past the visibility timeout is
// what a real adapter's reclaim logic reacts to.

// A permanently-failed task must never be claimable again, even
// after advancing well past any visibility timeout -- that is
// what distinguishes "poison" from an ordinary reclaimable
// failure.

func runRetryableNackMakesTaskClaimableAgain(t *testing.T, newQueue func(tb testing.TB, clock acceptanceharness.Clock) Queue) {
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newQueue(t, clock)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := q.Enqueue(ctx, Task{ID: "task-1", Payload: []byte("payload")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	first, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("initial Claim: ok=%v err=%v", ok, err)
	}

	if err := q.Nack(ctx, first, false); err != nil {
		t.Fatalf("Nack(permanent=false): %v", err)
	}

	// A retryable Nack must invalidate the pre-Nack claim's token
	// immediately -- not lazily on the next Claim -- so a worker that
	// gets Nack'd and then races a Complete call must fail here, before
	// any other worker has reclaimed the task.
	if err := q.Complete(ctx, first); err == nil {
		t.Fatalf("Complete with stale (pre-Nack) claim immediately after Nack: want error, got nil")
	}

	second, ok, err := q.Claim(ctx)
	if err != nil {
		t.Fatalf("Claim after retryable Nack: %v", err)
	}
	if !ok {
		t.Fatalf("Claim after retryable Nack: want the task to be claimable again")
	}
	if second.TaskID != first.TaskID {
		t.Fatalf("reclaimed task id = %q, want %q", second.TaskID, first.TaskID)
	}
	if second.Attempt != first.Attempt+1 {
		t.Fatalf("reclaimed attempt = %d, want %d", second.Attempt, first.Attempt+1)
	}

	if err := q.Complete(ctx, second); err != nil {
		t.Fatalf("Complete after retryable Nack: %v", err)
	}

	// The pre-Nack claim must be just as stale as a pre-reclaim one.
	if err := q.Complete(ctx, first); err == nil {
		t.Fatalf("Complete with stale (pre-Nack) claim: want error, got nil")
	}
}

// ADR-006's behavioral contract promises "possible duplicate
// delivery" and requires workers to treat redelivery of the same
// job id as at-least-once. This suite has no broker-level replay
// mechanism to inject a duplicate directly, so it simulates the
// closest observable equivalent through the portable Queue
// surface: re-Enqueue the same task id after it has already been
// completed (mirroring a broker redelivering a message the
// original worker already acknowledged, e.g. due to a delayed ack
// the broker didn't see in time) and assert the queue keeps
// working -- the redelivered attempt can be claimed and completed
// on its own terms, without erroring out or leaving the queue in
// a state where the original completion's bookkeeping is
// corrupted.

// This is deliberately a thin composition check, not a restatement
// of "kill-mid-attempt enables reclaim": it only confirms that a
// worker holding an active claim and simply going quiet (the
// "graceful shutdown" case, as opposed to a hard kill) does not
// have its claim reclaimed before the visibility timeout elapses,
// and that reclaim semantics still compose correctly once it
// eventually does.

// The shutting-down worker stops calling Claim, but the
// visibility timeout has not elapsed yet: no other worker should
// be able to claim task-1.

// runMultiWorkerScaling backs the "multi-worker scaling" subtest. Per
// Queue.Claim's own doc comment, ok=false means only "nothing claimable
// right now", not "the queue is fully drained" -- an adapter with any
// asynchronous delivery latency could hand a worker a spurious empty claim
// mid-drain while other workers still have completions to make. A worker
// that exited its loop on the first ok=false would treat those as
// equivalent and risk finishing early, so every worker instead keeps
// retrying (with a short bounded backoff) until a shared completion count
// reaches taskCount or ctx is done -- the latter remains the real safety
// net against a genuinely broken adapter.

type multiWorkerRun struct {
	mu             sync.Mutex
	completions    map[string]int
	errs           []error
	totalCompleted int
	taskCount      int
}

func (r *multiWorkerRun) finished() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.totalCompleted >= r.taskCount
}

func (r *multiWorkerRun) fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs = append(r.errs, err)
}

func (r *multiWorkerRun) record(taskID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.completions[taskID]++
	r.totalCompleted++
}

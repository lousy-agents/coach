// Package queueconformance defines a black-box behavioral contract for the
// eventual TaskQueue port described by ADR-006, and a reusable conformance
// suite (Run) that exercises any Queue implementation against it. This
// package intentionally does not implement or import any real broker
// (Redis Streams, SQS): Baseline Task 3a's adapter packages import this
// package and call Run against their own Queue-satisfying factory. See
// GitHub issue #78 (epic #73, Task 0.4).
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
// exclusion, kill-mid-attempt reclaim, and post-reclaim single completion
// black-box testable against any implementation. Poison-task/DLQ/Nack
// semantics are out of scope for this task; they belong to Baseline Task
// 3a's full ADR-006 conformance suite.
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
		clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
		q := newQueue(t, clock)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := q.Enqueue(ctx, Task{ID: "task-1", Payload: []byte("payload")}); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}

		type result struct {
			claim Claim
			ok    bool
			err   error
		}
		results := make([]result, 2)

		var wg sync.WaitGroup
		wg.Add(len(results))
		for i := range results {
			i := i
			go func() {
				defer wg.Done()
				claim, ok, err := q.Claim(ctx)
				results[i] = result{claim: claim, ok: ok, err: err}
			}()
		}
		wg.Wait()

		successCount := 0
		for _, r := range results {
			if r.err != nil {
				t.Fatalf("Claim: %v", r.err)
			}
			if r.ok {
				successCount++
				if r.claim.TaskID != "task-1" {
					t.Fatalf("claimed unexpected task id %q, want %q", r.claim.TaskID, "task-1")
				}
			}
		}
		if successCount != 1 {
			t.Fatalf("want exactly 1 successful concurrent claim, got %d", successCount)
		}
	})

	t.Run("kill-mid-attempt enables reclaim", func(t *testing.T) {
		clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
		q := newQueue(t, clock)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := q.Enqueue(ctx, Task{ID: "task-1", Payload: []byte("payload")}); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}

		first, ok, err := q.Claim(ctx)
		if err != nil {
			t.Fatalf("first Claim: %v", err)
		}
		if !ok {
			t.Fatalf("want first Claim to succeed")
		}

		// Simulate a crash after partial persistence: the first worker
		// never calls Complete. Advancing past the visibility timeout is
		// what a real adapter's reclaim logic reacts to.
		clock.Advance(reclaimAdvance)

		second, ok, err := q.Claim(ctx)
		if err != nil {
			t.Fatalf("reclaim Claim: %v", err)
		}
		if !ok {
			t.Fatalf("want reclaim Claim to succeed once the visibility timeout has elapsed")
		}
		if second.TaskID != first.TaskID {
			t.Fatalf("reclaimed task id = %q, want %q", second.TaskID, first.TaskID)
		}
		if second.Attempt != first.Attempt+1 {
			t.Fatalf("reclaimed attempt = %d, want %d", second.Attempt, first.Attempt+1)
		}
	})

	t.Run("post-reclaim single completion", func(t *testing.T) {
		clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
		q := newQueue(t, clock)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := q.Enqueue(ctx, Task{ID: "task-1", Payload: []byte("payload")}); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}

		stale, ok, err := q.Claim(ctx)
		if err != nil || !ok {
			t.Fatalf("initial Claim: ok=%v err=%v", ok, err)
		}

		clock.Advance(reclaimAdvance)

		fresh, ok, err := q.Claim(ctx)
		if err != nil || !ok {
			t.Fatalf("reclaim Claim: ok=%v err=%v", ok, err)
		}

		if err := q.Complete(ctx, stale); err == nil {
			t.Fatalf("Complete with stale (pre-reclaim) claim: want error, got nil")
		}

		if err := q.Complete(ctx, fresh); err != nil {
			t.Fatalf("Complete with fresh (post-reclaim) claim: %v", err)
		}
	})
}

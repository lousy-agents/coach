package queue_test

import (
	"context"
	"errors"
	"fmt"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// fakeTaskQueue is a minimal in-memory TaskQueue test double, not a
// standalone adapter: it exists only to prove the port's Enqueue/Claim/
// Complete/Nack contract is implementable and behaves as ADR-006 requires,
// at the acceptance-test boundary. Real broker adapters (Redis Streams,
// SQS) are separate follow-on tasks.
// inFlightEntry is one claimed-but-not-yet-acknowledged task, keyed by
// Claim.Token rather than Task.ID: ADR-006 permits duplicate delivery of the
// same task id, and keying by TaskID would let a second Claim for that id
// silently overwrite the first claim's bookkeeping (see
// internal/coachapi/queue/sqs's inflightClaim, keyed identically for the
// same reason). Storing the full Task (not just Claim) lets Nack's
// retryable branch re-enqueue the original Payload instead of dropping it.
type inFlightEntry struct {
	task queue.Task
}

type fakeTaskQueue struct {
	mu       sync.Mutex
	pending  []queue.Task
	inFlight map[string]inFlightEntry // keyed by Claim.Token
	poison   map[string]bool
	attempts map[string]int
}

func newFakeTaskQueue() *fakeTaskQueue {
	return &fakeTaskQueue{
		inFlight: make(map[string]inFlightEntry),
		poison:   make(map[string]bool),
		attempts: make(map[string]int),
	}
}

func (q *fakeTaskQueue) Enqueue(_ context.Context, task queue.Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pending = append(q.pending, task)
	return nil
}

func (q *fakeTaskQueue) Claim(_ context.Context) (queue.Claim, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return queue.Claim{}, false, nil
	}
	task := q.pending[0]
	q.pending = q.pending[1:]
	q.attempts[task.ID]++
	// attempts[task.ID] is a 1-based internal claim counter; the reported
	// Attempt is 0-based, matching the redisstream and sqs adapters' first-
	// claim convention.
	attempt := q.attempts[task.ID] - 1
	token := fmt.Sprintf("%s-attempt-%d", task.ID, q.attempts[task.ID])
	claim := queue.Claim{TaskID: task.ID, Attempt: attempt, Token: token}
	q.inFlight[token] = inFlightEntry{task: task}
	return claim, true, nil
}

func (q *fakeTaskQueue) Complete(_ context.Context, claim queue.Claim) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	entry, ok := q.inFlight[claim.Token]
	if !ok || entry.task.ID != claim.TaskID {
		return errors.New("fakeTaskQueue: stale claim")
	}
	delete(q.inFlight, claim.Token)
	return nil
}

func (q *fakeTaskQueue) Nack(_ context.Context, claim queue.Claim, permanent bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	entry, ok := q.inFlight[claim.Token]
	if !ok || entry.task.ID != claim.TaskID {
		return errors.New("fakeTaskQueue: stale claim")
	}
	delete(q.inFlight, claim.Token)
	if permanent {
		q.poison[claim.TaskID] = true
		return nil
	}
	q.pending = append(q.pending, entry.task)
	return nil
}

func (q *fakeTaskQueue) isPoisoned(taskID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.poison[taskID]
}

// payloadFor is a test-only accessor: Claim's queue.Claim return type is
// deliberately payload-less (internal/coachapi/queue.Claim), so specs that
// need to assert a claimed task's payload survived a retry go through this
// instead.
func (q *fakeTaskQueue) payloadFor(token string) []byte {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.inFlight[token].task.Payload
}

var _ queue.TaskQueue = (*fakeTaskQueue)(nil)

var _ = Describe("TaskQueue port", func() {
	var q *fakeTaskQueue
	var ctx context.Context

	BeforeEach(func() {
		q = newFakeTaskQueue()
		ctx = context.Background()
	})

	When("a retryable failure is reported via Nack", func() {
		It("makes the task claimable again for another attempt, preserving the original payload", func() {
			Expect(q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("payload")})).To(Succeed())

			first, ok, err := q.Claim(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(first.Attempt).To(Equal(0), "the first claim of a task must report a 0-based Attempt, matching the real adapters")

			Expect(q.Nack(ctx, first, false)).To(Succeed())

			second, ok, err := q.Claim(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue(), "a retryable Nack must leave the task claimable")
			Expect(second.TaskID).To(Equal("task-1"))
			Expect(second.Attempt).To(Equal(first.Attempt + 1))
			Expect(q.payloadFor(second.Token)).To(Equal([]byte("payload")), "a retryable Nack must not drop the original task payload on re-enqueue")

			// The pre-reclaim claim's token must be invalidated by the
			// reclaim, not merely coincide with the fresh claim's token.
			Expect(q.Complete(ctx, first)).To(HaveOccurred(), "a stale (pre-reclaim) claim must not be completable")
		})
	})

	When("the same task id is delivered twice (duplicate enqueue or redelivery)", func() {
		It("tracks each claim independently instead of one overwriting the other's bookkeeping", func() {
			Expect(q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("first")})).To(Succeed())
			Expect(q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("second")})).To(Succeed())

			first, ok, err := q.Claim(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			second, ok, err := q.Claim(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(second.Token).NotTo(Equal(first.Token), "two independently delivered claims for the same TaskID must get distinct tokens")

			Expect(q.Complete(ctx, first)).To(Succeed(), "completing the first claim must succeed: the second claim must not have overwritten its bookkeeping")
			Expect(q.Complete(ctx, second)).To(Succeed(), "completing the second claim must succeed: it must still be tracked after the first claim's Complete")
		})
	})

	When("a permanent failure is reported via Nack", func() {
		It("routes the task to the poison-task destination and it is never claimable again", func() {
			Expect(q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("payload")})).To(Succeed())

			claim, ok, err := q.Claim(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			Expect(q.Nack(ctx, claim, true)).To(Succeed())
			Expect(q.isPoisoned("task-1")).To(BeTrue())

			_, ok, err = q.Claim(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse(), "a permanently-failed task must never be claimable again")
		})
	})

	When("Enqueue, Claim, and Complete are used for a normal successful attempt", func() {
		It("completes exactly once and leaves nothing claimable afterward", func() {
			Expect(q.Enqueue(ctx, queue.Task{ID: "task-2", Payload: []byte("payload")})).To(Succeed())

			claim, ok, err := q.Claim(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			Expect(q.Complete(ctx, claim)).To(Succeed())

			_, ok, err = q.Claim(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})
})

var _ = Describe("NoOpEventBus", func() {
	var bus queue.NoOpEventBus
	var ctx context.Context

	BeforeEach(func() {
		bus = queue.NoOpEventBus{}
		ctx = context.Background()
	})

	When("Publish is called", func() {
		It("always succeeds without delivering anywhere", func() {
			Expect(bus.Publish(ctx, "topic", []byte("payload"))).To(Succeed())
		})
	})

	When("Subscribe is called", func() {
		It("returns a non-nil, already-closed channel with no deliveries and a non-nil unsubscribe func", func() {
			payloads, unsubscribe, err := bus.Subscribe(ctx, "topic")
			Expect(err).NotTo(HaveOccurred())
			Expect(payloads).NotTo(BeNil())
			Expect(unsubscribe).NotTo(BeNil())

			Eventually(payloads).Should(BeClosed())

			_, ok := <-payloads
			Expect(ok).To(BeFalse(), "a closed channel must report ok=false on receive rather than blocking forever")

			unsubscribe()
		})
	})
})

var _ = Describe("Capabilities fail-fast startup check", func() {
	When("every capability the application requires is present", func() {
		It("succeeds", func() {
			have := queue.Capabilities{
				NativeDeadLetterQueue: true,
				LeaseExtension:        true,
			}
			want := queue.Capabilities{
				NativeDeadLetterQueue: true,
				LeaseExtension:        true,
			}
			Expect(queue.RequireCapabilities(have, want)).To(Succeed())
		})
	})

	When("a required capability is unavailable on the configured backend", func() {
		It("fails startup with a descriptive, non-nil error naming the missing capability", func() {
			have := queue.Capabilities{
				NativeDeadLetterQueue: false,
				OrderedDelivery:       true,
			}
			want := queue.Capabilities{
				NativeDeadLetterQueue: true,
				OrderedDelivery:       true,
			}

			err := queue.RequireCapabilities(have, want)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("NativeDeadLetterQueue"))
		})
	})

	When("multiple required capabilities are unavailable on the configured backend", func() {
		It("fails startup with an error naming every missing capability, not just the first", func() {
			have := queue.Capabilities{
				NativeDeadLetterQueue: false,
				DelayedDelivery:       true,
				OrderedDelivery:       false,
				LeaseExtension:        false,
			}
			want := queue.Capabilities{
				NativeDeadLetterQueue: true,
				DelayedDelivery:       true,
				OrderedDelivery:       true,
				LeaseExtension:        true,
			}

			err := queue.RequireCapabilities(have, want)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("NativeDeadLetterQueue"))
			Expect(err.Error()).To(ContainSubstring("OrderedDelivery"))
			Expect(err.Error()).To(ContainSubstring("LeaseExtension"))
		})
	})
})

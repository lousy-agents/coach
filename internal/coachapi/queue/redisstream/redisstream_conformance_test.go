package redisstream_test

import (
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/acceptanceharness/queueconformance"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
	"github.com/lousy-agents/coach/internal/coachapi/queue/redisstream"
)

// conformanceQueue adapts a *redisstream.Queue (which speaks
// internal/coachapi/queue.Task/Claim, the real TaskQueue port) to
// queueconformance.Queue (which speaks its own, structurally identical
// but distinctly named Task/Claim types). Go's type system does not let
// one method satisfy two different named parameter types, so this
// conversion shim -- not *redisstream.Queue itself -- is what actually
// implements queueconformance.Queue; *redisstream.Queue remains the real
// production TaskQueue implementation (see queue.go's var _
// queue.TaskQueue assertion).
type conformanceQueue struct {
	q *redisstream.Queue
}

var _ queueconformance.Queue = conformanceQueue{}

// TestRedisStreamQueueConformanceAcceptance runs the shared black-box
// TaskQueue conformance suite (Task 3a, GitHub issue #100, epic #97)
// against a real Redis Streams-backed Queue. It skips gracefully -- does
// not fail or hang -- whenever Docker is unavailable or a throwaway Redis
// container cannot be started, since Docker's daemon is not reachable in
// every environment this suite runs in (e.g. this sandbox); it is
// expected to actually run in CI. Matches the *Acceptance naming
// convention so `go test -race ./... -run Acceptance` /
// `mise run test-acceptance-fast` picks it up.
func TestRedisStreamQueueConformanceAcceptance(t *testing.T) {
	address := startRedisContainer(t)

	queueconformance.Run(t, func(tb testing.TB, clock acceptanceharness.Clock) queueconformance.Queue {
		return newConformanceQueue(tb, clock, address)
	})
}

// newConformanceQueue builds the fresh, empty Queue each conformance
// subtest requires, closing it when the subtest ends.
func newConformanceQueue(tb testing.TB, clock acceptanceharness.Clock, address string) queueconformance.Queue {
	cfg := redisstream.Config{
		Address: address,
		// A unique stream+group per subtest gives Run the fresh,
		// empty Queue it requires, while reusing one Redis container
		// for the whole suite.
		Stream:        "conformance-" + watermill.NewUUID(),
		ConsumerGroup: "conformance-workers",
		ClaimAfter:    time.Minute,
	}
	q, err := redisstream.NewQueue(cfg, clock)
	if err != nil {
		tb.Fatalf("NewQueue: %v", err)
	}
	tb.Cleanup(func() {
		if closeErr := q.Close(); closeErr != nil {
			tb.Logf("Queue.Close: %v", closeErr)
		}
	})
	return conformanceQueue{q: q}
}

func (c conformanceQueue) PoisonTasks(ctx context.Context) ([]queueconformance.Task, error) {
	tasks, err := c.q.PoisonTasks(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]queueconformance.Task, len(tasks))
	for i, task := range tasks {
		out[i] = queueconformance.Task{ID: task.ID, Payload: task.Payload}
	}
	return out, nil
}

func (c conformanceQueue) Enqueue(ctx context.Context, task queueconformance.Task) error {
	return c.q.Enqueue(ctx, queue.Task{ID: task.ID, Payload: task.Payload})
}

func (c conformanceQueue) Claim(ctx context.Context) (queueconformance.Claim, bool, error) {
	claim, ok, err := c.q.Claim(ctx)
	return queueconformance.Claim{TaskID: claim.TaskID, Attempt: claim.Attempt, Token: claim.Token}, ok, err
}

func (c conformanceQueue) Complete(ctx context.Context, claim queueconformance.Claim) error {
	return c.q.Complete(ctx, queue.Claim{TaskID: claim.TaskID, Attempt: claim.Attempt, Token: claim.Token})
}

func (c conformanceQueue) Nack(ctx context.Context, claim queueconformance.Claim, permanent bool) error {
	return c.q.Nack(ctx, queue.Claim{TaskID: claim.TaskID, Attempt: claim.Attempt, Token: claim.Token}, permanent)
}

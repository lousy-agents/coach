package redisstream_test

import (
	"testing"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/acceptanceharness/queueconformance"
)

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
		return body_redisstreamConformancePart2Test_27(tb, clock, address)
	})
}

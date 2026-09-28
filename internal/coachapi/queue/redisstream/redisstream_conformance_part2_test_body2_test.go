package redisstream_test

import (
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/acceptanceharness/queueconformance"

	"github.com/lousy-agents/coach/internal/coachapi/queue/redisstream"
)

func body_redisstreamConformancePart2Test_27(tb testing.TB, clock acceptanceharness.Clock, address string) queueconformance.Queue {
	cfg := redisstream.Config{
		Address: address,

		Stream:        "conformance-" + watermill.NewUUID(),
		ConsumerGroup: "conformance-workers",
		ClaimAfter:    time.Minute,
	}
	q, err := redisstream.NewQueue(cfg, clock)
	if err != nil {
		tb.Fatalf("NewQueue: %v", err)
	}
	tb.Cleanup(func() {
		body_redisstreamConformancePart2Test_39(tb, q)
	})
	return conformanceQueue{q: q}
}

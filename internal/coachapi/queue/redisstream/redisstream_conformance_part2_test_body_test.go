package redisstream_test

import (
	"testing"

	"github.com/lousy-agents/coach/internal/coachapi/queue/redisstream"
)

func body_redisstreamConformancePart2Test_39(tb testing.TB, q *redisstream.Queue) {
	if closeErr := q.Close(); closeErr != nil {
		tb.Logf("Queue.Close: %v", closeErr)
	}
}

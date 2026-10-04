package sqs

import (
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

func newTestQueue(t *testing.T, api sqsAPI, clock acceptanceharness.Clock) *Queue {
	t.Helper()
	return &Queue{
		client:            api,
		queueURL:          "https://fake.local/queues/main",
		poisonQueueURL:    "https://fake.local/queues/main-poison",
		visibilityTimeout: time.Minute,
		clock:             clock,
		inflight:          make(map[string]*inflightClaim),
	}
}

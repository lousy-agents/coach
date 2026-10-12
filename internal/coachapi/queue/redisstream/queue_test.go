package redisstream

import (
	"testing"
	"time"
)

// TestNewQueueValidatesConfigBeforeDialing proves NewQueue rejects an
// invalid Config without needing a reachable Redis instance -- it must
// fail on cfg.Validate(), not on a Ping timeout.
func TestNewQueueValidatesConfigBeforeDialing(t *testing.T) {
	_, err := NewQueue(Config{}, nil)
	if err == nil {
		t.Fatalf("NewQueue(zero Config) = nil error, want a validation error")
	}
}

// TestNewQueueFailsFastAgainstUnreachableRedis proves the outbound
// network policy (AGENTS.md "Outbound HTTP required policy", applied here
// to the Redis dial): NewQueue must not hang against an unreachable
// address, it must return within a small bound derived from
// Config.DialTimeout.
func TestNewQueueFailsFastAgainstUnreachableRedis(t *testing.T) {
	start := time.Now()
	_, err := NewQueue(Config{
		Address:       "127.0.0.1:1",
		Stream:        "coach-analysis",
		ConsumerGroup: "coach-workers",
		ClaimAfter:    time.Minute,
		DialTimeout:   200 * time.Millisecond,
	}, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("NewQueue against an unreachable address = nil error, want a connection error")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("NewQueue against an unreachable address took %v, want it to fail fast (bounded by DialTimeout)", elapsed)
	}
}

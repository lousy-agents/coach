package redisstream

import (
	"errors"
	"fmt"
	"time"
)

// defaultDialTimeout bounds NewQueue's initial Redis connection attempt so
// it fails fast against an unreachable Redis instead of hanging, per
// AGENTS.md's outbound-network timeout policy.
const defaultDialTimeout = 5 * time.Second

// Config configures a Redis Streams-backed Queue, matching ADR-006's
// "Provider configuration" Redis Streams shape (address, consumerGroup,
// claimAfter; docs/architecture/ADR-006-watermill-queue-abstraction.md).
type Config struct {
	// Address is the Redis server address (host:port), e.g. "redis:6379".
	Address string
	// Password is the Redis AUTH password. Empty means no auth.
	Password string
	// DB selects the Redis logical database index.
	DB int

	// Stream is the Redis Stream (and Watermill topic) task attempts are
	// published to and consumed from; ADR-006's "taskQueue" name.
	Stream string
	// ConsumerGroup is the Redis Streams consumer group every Queue
	// instance for Stream shares. It is what makes claims competing: at
	// most one consumer in the group receives a given stream entry.
	ConsumerGroup string
	// Consumer names this Queue instance's Redis Streams consumer within
	// ConsumerGroup. A generated id is used when empty.
	Consumer string

	// ClaimAfter is how long an unresolved claim (neither Complete nor
	// Nack called) may be held before Claim treats it as abandoned and
	// reclaims it for another attempt -- this Queue's equivalent of a
	// visibility timeout. See Queue's doc comment for why this is
	// enforced against the injected Clock rather than Redis server time.
	ClaimAfter time.Duration

	// DialTimeout bounds the initial Redis connection attempt. Defaults
	// to defaultDialTimeout when zero or negative.
	DialTimeout time.Duration
}

func (c *Config) setDefaults() {
	if c.DialTimeout <= 0 {
		c.DialTimeout = defaultDialTimeout
	}
}

// Validate reports a descriptive error naming every required Config field
// that is missing or out of range, or nil if c is usable by NewQueue.
func (c Config) Validate() error {
	var missing []string
	if c.Address == "" {
		missing = append(missing, "Address")
	}
	if c.Stream == "" {
		missing = append(missing, "Stream")
	}
	if c.ConsumerGroup == "" {
		missing = append(missing, "ConsumerGroup")
	}
	if len(missing) > 0 {
		return fmt.Errorf("redisstream: config missing required field(s): %v", missing)
	}
	if c.ClaimAfter <= 0 {
		return errors.New("redisstream: config ClaimAfter must be positive")
	}
	return nil
}

// poisonStreamSuffix names this adapter's poison-task destination
// (ADR-006 rule 5): a dedicated Redis Stream, separate from Stream, that
// permanently-failed tasks are published to via the same Watermill
// Publisher. PoisonTasks reads it back with a plain XRANGE, since
// Watermill's Subscriber is push-only and has no read-back API.
const poisonStreamSuffix = "-poison"

func poisonStreamName(stream string) string {
	return stream + poisonStreamSuffix
}

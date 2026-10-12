// Package redisstream implements the internal/coachapi/queue.TaskQueue
// port (ADR-006, docs/architecture/ADR-006-watermill-queue-abstraction.md)
// on top of Watermill's Redis Streams pub/sub
// (github.com/ThreeDotsLabs/watermill-redisstream), for coach's non-AWS /
// self-hosted deployment target. Enqueue publishes through a Watermill
// Publisher; Complete and permanent Nack acknowledge through the
// underlying Watermill message, so ADR-006 rule 5's "successful handler ->
// Ack, retryable error -> Nack, permanent error -> Ack plus publication to
// a poison-task destination" holds at the wire level. Redis pending-entry
// lists, consumer groups, XCLAIM, and Watermill's *message.Message are not
// exposed: callers only see queue.Task and queue.Claim.
package redisstream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	wmredisstream "github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/redis/go-redis/v9"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// taskIDMetadataKey stores queue.Task.ID in a Watermill message's
// Metadata, so it survives the round trip through Redis Streams alongside
// the opaque Payload.
const taskIDMetadataKey = "task_id"

// maxWatermillIdleTime is set on wmredisstream.SubscriberConfig.MaxIdleTime
// to make watermill-redisstream's own real-wall-clock, Redis-server-time
// XCLAIM-based reclaim effectively never fire. This Queue enforces
// ClaimAfter itself against the injected acceptanceharness.Clock (see
// reclaimExpired), which is what lets crash-recovery tests force a reclaim
// deterministically via FakeClock.Advance instead of waiting on real time
// or Redis server time -- ADR-006 rule 9 ("Clock and durations are
// injected so crash-recovery tests are deterministic without real
// waiting"). A real Redis instance has no notion of an injected clock, so
// this Queue's own bookkeeping, not Redis's PEL idle time, is the sole
// authority for "has this claim expired".
const maxWatermillIdleTime = 10000 * time.Hour

// claimPollWindow bounds how long Claim waits for a new message to arrive
// on the Subscriber's output channel before reporting nothing claimable.
// It exists because watermill-redisstream delivers messages through an
// internal goroutine pipeline (Redis XREADGROUP -> channel) with real,
// small latency; a purely non-blocking read could report ok=false for a
// message that is a few milliseconds away from arriving, which would make
// a multi-worker consumer prematurely stop polling. It is unrelated to
// ClaimAfter/reclaim timing and is intentionally driven by real time, not
// the injected Clock -- it exists to smooth I/O latency, not to model a
// visibility timeout.
const claimPollWindow = 300 * time.Millisecond

// pendingClaim is this Queue's own bookkeeping for one outstanding
// (neither Complete'd nor Nack'd) claim. msg is kept so the eventual
// terminal outcome (Complete or permanent Nack) can call msg.Ack(),
// releasing the message in Redis's pending-entries list exactly once;
// intermediate reclaims (expiry or retryable Nack) never touch msg's
// Ack/Nack channels, they only replace this Queue's own token/attempt
// bookkeeping, which is what invalidates the previous claim's Token.
//
// readyForClaim is set by a retryable Nack, whose Attempt increment and
// re-tokening has already happened at Nack time: it tells reclaimExpired
// to hand this claim back on the very next Claim without incrementing
// Attempt a second time. Expiry-driven reclaims (a claim nobody Nack'd or
// Completed within claimAfter) never set it; they increment Attempt inside
// reclaimExpired itself, exactly once per genuine timeout.
type pendingClaim struct {
	taskID        string
	attempt       int
	token         string
	claimedAt     time.Time
	readyForClaim bool
	msg           *message.Message
}

// Queue implements internal/coachapi/queue.TaskQueue against Redis
// Streams. It also exposes PoisonTasks, which is not part of that port
// but is required by internal/acceptanceharness/queueconformance.Queue
// (see redisstream_conformance_test.go for the adapter that reconciles
// the two packages' structurally-identical-but-distinctly-named Task and
// Claim types).
//
// A single Queue is safe for concurrent use: Claim/Complete/Nack all
// serialize access to the shared pending-claims map via mu.
type Queue struct {
	client       redis.UniversalClient
	publisher    *wmredisstream.Publisher
	subscriber   *wmredisstream.Subscriber
	unmarshaller wmredisstream.Unmarshaller
	messages     <-chan *message.Message
	cancelSub    context.CancelFunc

	stream       string
	poisonStream string
	claimAfter   time.Duration
	clock        acceptanceharness.Clock

	mu      sync.Mutex
	pending map[string]*pendingClaim
}

var _ queue.TaskQueue = (*Queue)(nil)

// NewQueue connects to Redis and returns a Queue consuming cfg.Stream
// under cfg.ConsumerGroup. clock defaults to acceptanceharness.RealClock
// when nil; tests inject acceptanceharness.FakeClock to force reclaims
// deterministically (see maxWatermillIdleTime's doc comment).
func NewQueue(cfg Config, clock acceptanceharness.Clock) (*Queue, error) {
	cfg.setDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if clock == nil {
		clock = acceptanceharness.RealClock{}
	}

	client := redis.NewClient(&redis.Options{
		Addr:        cfg.Address,
		Password:    cfg.Password,
		DB:          cfg.DB,
		DialTimeout: cfg.DialTimeout,
	})

	pingCtx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redisstream: connecting to %s: %w", cfg.Address, err)
	}

	logger := watermill.NopLogger{}

	publisher, err := wmredisstream.NewPublisher(wmredisstream.PublisherConfig{Client: client}, logger)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redisstream: creating publisher: %w", err)
	}

	subscriber, err := wmredisstream.NewSubscriber(wmredisstream.SubscriberConfig{
		Client:        client,
		ConsumerGroup: cfg.ConsumerGroup,
		Consumer:      cfg.Consumer,
		MaxIdleTime:   maxWatermillIdleTime,
	}, logger)
	if err != nil {
		_ = publisher.Close()
		return nil, fmt.Errorf("redisstream: creating subscriber: %w", err)
	}

	subCtx, cancelSub := context.WithCancel(context.Background())
	messages, err := subscriber.Subscribe(subCtx, cfg.Stream)
	if err != nil {
		cancelSub()
		_ = subscriber.Close()
		_ = publisher.Close()
		return nil, fmt.Errorf("redisstream: subscribing to stream %q: %w", cfg.Stream, err)
	}

	return &Queue{
		client:       client,
		publisher:    publisher,
		subscriber:   subscriber,
		unmarshaller: wmredisstream.DefaultMarshallerUnmarshaller{},
		messages:     messages,
		cancelSub:    cancelSub,
		stream:       cfg.Stream,
		poisonStream: poisonStreamName(cfg.Stream),
		claimAfter:   cfg.ClaimAfter,
		clock:        clock,
		pending:      make(map[string]*pendingClaim),
	}, nil
}

// Close stops consuming and releases the underlying Redis connection.
func (q *Queue) Close() error {
	q.cancelSub()
	subErr := q.subscriber.Close()
	pubErr := q.publisher.Close()
	if subErr != nil {
		return subErr
	}
	return pubErr
}

// Enqueue publishes task onto the Redis Stream via the Watermill
// Publisher (an XADD under the hood).
func (q *Queue) Enqueue(ctx context.Context, task queue.Task) error {
	msg := message.NewMessage(watermill.NewUUID(), task.Payload)
	msg.Metadata.Set(taskIDMetadataKey, task.ID)
	msg.SetContext(ctx)

	if err := q.publisher.Publish(q.stream, msg); err != nil {
		return fmt.Errorf("redisstream: enqueue task %q: %w", task.ID, err)
	}
	return nil
}

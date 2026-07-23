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
type pendingClaim struct {
	taskID    string
	attempt   int
	token     string
	claimedAt time.Time
	msg       *message.Message
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

// Claim first reclaims any pending claim whose ClaimAfter has elapsed
// (per the injected Clock), and only if none did, waits up to
// claimPollWindow for a newly delivered message. ok=false means neither
// happened before the wait window (or ctx) elapsed.
func (q *Queue) Claim(ctx context.Context) (queue.Claim, bool, error) {
	if claim, ok := q.reclaimExpired(); ok {
		return claim, true, nil
	}

	timer := time.NewTimer(claimPollWindow)
	defer timer.Stop()

	select {
	case msg, open := <-q.messages:
		if !open {
			return queue.Claim{}, false, fmt.Errorf("redisstream: subscriber channel closed")
		}
		return q.trackNewClaim(msg), true, nil
	case <-timer.C:
		return queue.Claim{}, false, nil
	case <-ctx.Done():
		return queue.Claim{}, false, ctx.Err()
	}
}

// reclaimExpired looks for one pendingClaim whose claimAfter has elapsed
// per q.clock.Now(), and if found, replaces it in place with a new token
// and an incremented attempt count -- invalidating the old Token, per
// TaskQueue.Complete/Nack's stale-token contract -- without touching the
// underlying Watermill message's Ack/Nack channels (see pendingClaim's
// doc comment).
func (q *Queue) reclaimExpired() (queue.Claim, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := q.clock.Now()
	for token, pc := range q.pending {
		if now.Sub(pc.claimedAt) < q.claimAfter {
			continue
		}
		delete(q.pending, token)
		pc.attempt++
		pc.token = watermill.NewUUID()
		pc.claimedAt = now
		q.pending[pc.token] = pc
		return queue.Claim{TaskID: pc.taskID, Attempt: pc.attempt, Token: pc.token}, true
	}
	return queue.Claim{}, false
}

// trackNewClaim records a freshly delivered Watermill message as a new
// pendingClaim (attempt 0) and returns its Claim.
func (q *Queue) trackNewClaim(msg *message.Message) queue.Claim {
	taskID := msg.Metadata.Get(taskIDMetadataKey)
	token := watermill.NewUUID()

	q.mu.Lock()
	q.pending[token] = &pendingClaim{
		taskID:    taskID,
		attempt:   0,
		token:     token,
		claimedAt: q.clock.Now(),
		msg:       msg,
	}
	q.mu.Unlock()

	return queue.Claim{TaskID: taskID, Attempt: 0, Token: token}
}

// takePending removes and returns the pendingClaim matching claim's
// Token, or ok=false if no such claim is currently outstanding (already
// completed, poisoned, or superseded by a reclaim) -- the stale-token
// condition Complete and Nack must both fail under.
func (q *Queue) takePending(claim queue.Claim) (*pendingClaim, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	pc, ok := q.pending[claim.Token]
	if !ok || pc.taskID != claim.TaskID {
		return nil, false
	}
	delete(q.pending, claim.Token)
	return pc, true
}

// Complete acknowledges claim's task attempt as durably finished. It
// fails if claim.Token has been invalidated by a reclaim.
func (q *Queue) Complete(ctx context.Context, claim queue.Claim) error {
	pc, ok := q.takePending(claim)
	if !ok {
		return fmt.Errorf("redisstream: complete: claim token invalid or expired for task %q", claim.TaskID)
	}
	pc.msg.Ack()
	return nil
}

// Nack reports claim's task attempt failed. A retryable failure
// (permanent=false) makes the task claimable again with Attempt
// incremented. A permanent failure (permanent=true) acknowledges the
// underlying message (ADR-006 rule 5) and republishes the task onto the
// poison-task destination stream; PoisonTasks reads that stream back.
// Nack fails under the same stale-token condition as Complete.
func (q *Queue) Nack(ctx context.Context, claim queue.Claim, permanent bool) error {
	pc, ok := q.takePending(claim)
	if !ok {
		return fmt.Errorf("redisstream: nack: claim token invalid or expired for task %q", claim.TaskID)
	}

	if permanent {
		pc.msg.Ack()
		return q.publishPoison(ctx, pc.taskID, pc.msg.Payload)
	}

	pc.attempt++
	pc.token = watermill.NewUUID()
	pc.claimedAt = q.clock.Now()

	q.mu.Lock()
	q.pending[pc.token] = pc
	q.mu.Unlock()

	return nil
}

func (q *Queue) publishPoison(ctx context.Context, taskID string, payload []byte) error {
	msg := message.NewMessage(watermill.NewUUID(), payload)
	msg.Metadata.Set(taskIDMetadataKey, taskID)
	msg.SetContext(ctx)

	if err := q.publisher.Publish(q.poisonStream, msg); err != nil {
		return fmt.Errorf("redisstream: publishing task %q to poison destination %q: %w", taskID, q.poisonStream, err)
	}
	return nil
}

// PoisonTasks returns every task a permanent Nack has routed to the
// poison-task destination stream, oldest first. It is not part of
// queue.TaskQueue; internal/acceptanceharness/queueconformance.Queue
// requires it so both this package's own tests and the shared
// conformance suite can assert the poison destination actually received a
// task.
func (q *Queue) PoisonTasks(ctx context.Context) ([]queue.Task, error) {
	entries, err := q.client.XRange(ctx, q.poisonStream, "-", "+").Result()
	if err != nil {
		return nil, fmt.Errorf("redisstream: reading poison-task destination %q: %w", q.poisonStream, err)
	}

	tasks := make([]queue.Task, 0, len(entries))
	for _, entry := range entries {
		msg, err := q.unmarshaller.Unmarshal(entry.Values)
		if err != nil {
			return nil, fmt.Errorf("redisstream: decoding poison-task destination entry %s: %w", entry.ID, err)
		}
		tasks = append(tasks, queue.Task{ID: msg.Metadata.Get(taskIDMetadataKey), Payload: msg.Payload})
	}
	return tasks, nil
}

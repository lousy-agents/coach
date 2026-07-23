package sqs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// errStaleClaim is returned by Complete/Nack when the given claim's token
// is not a currently-tracked in-flight claim, or is tracked under a
// different task id -- either because it was never claimed, has already
// been Complete/Nack-ed, or has been reclaimed after its visibility
// deadline elapsed (see the package doc comment's "Reclaim mechanism"
// section).
var errStaleClaim = errors.New("sqs: claim token is stale (already completed, nacked, or reclaimed)")

// maxPoisonDrainRounds bounds PoisonTasks's ReceiveMessage loop so a
// pathologically large poison queue cannot make a single call block
// forever; each round can return up to 10 messages (SQS's own
// MaxNumberOfMessages cap).
const maxPoisonDrainRounds = 50

// sqsAPI is the subset of *awssqs.Client this package calls, narrowed so
// unit tests can substitute a fake without spinning up LocalStack.
type sqsAPI interface {
	SendMessage(ctx context.Context, in *awssqs.SendMessageInput, opts ...func(*awssqs.Options)) (*awssqs.SendMessageOutput, error)
	ReceiveMessage(ctx context.Context, in *awssqs.ReceiveMessageInput, opts ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, in *awssqs.DeleteMessageInput, opts ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error)
	ChangeMessageVisibility(ctx context.Context, in *awssqs.ChangeMessageVisibilityInput, opts ...func(*awssqs.Options)) (*awssqs.ChangeMessageVisibilityOutput, error)
	CreateQueue(ctx context.Context, in *awssqs.CreateQueueInput, opts ...func(*awssqs.Options)) (*awssqs.CreateQueueOutput, error)
}

// wireTask is this adapter's SQS message body encoding. json.Marshal
// base64-encodes the []byte Payload automatically, so a Task's opaque
// binary payload survives round-tripping through SQS's text message body
// (and through the poison queue, which reuses this same encoding).
type wireTask struct {
	ID      string `json:"id"`
	Payload []byte `json:"payload"`
}

// inflightClaim is Queue's own bookkeeping for one claimed-but-not-yet-
// acknowledged message, keyed by the SQS receipt handle (the Claim's
// Token) rather than task id: SQS can deliver multiple messages sharing
// the same Task.ID (ADR-006 permits duplicate delivery/enqueue), and each
// delivered message gets its own receipt handle, so the receipt handle is
// the only value that's actually unique per in-flight claim. See the
// package doc comment's "Reclaim mechanism" section for why this exists
// alongside SQS's native visibility timeout.
type inflightClaim struct {
	taskID        string
	receiptHandle string
	payload       []byte
	attempt       int
	deadline      time.Time
}

// Queue implements internal/coachapi/queue.TaskQueue on top of one SQS
// queue plus a poison-task destination queue it manages itself. See the
// package doc comment for the design choices behind its reclaim and
// poison-task mechanisms.
type Queue struct {
	client            sqsAPI
	queueURL          string
	poisonQueueURL    string
	visibilityTimeout time.Duration
	clock             acceptanceharness.Clock

	mu       sync.Mutex
	inflight map[string]*inflightClaim // keyed by receipt handle (Claim.Token)
}

var _ queue.TaskQueue = (*Queue)(nil)

// NewQueue validates cfg, constructs an SQS client pinned to cfg's explicit
// Region/Credentials/Endpoint (never an ambient credential chain -- see the
// package doc comment), ensures the poison-task destination queue exists,
// and returns a ready-to-use Queue. clock drives Queue's own reclaim
// deadline tracking; production callers pass acceptanceharness.RealClock{},
// and this package's conformance test passes an
// acceptanceharness.FakeClock.
func NewQueue(ctx context.Context, cfg Config, clock acceptanceharness.Clock) (*Queue, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if clock == nil {
		clock = acceptanceharness.RealClock{}
	}

	httpClient := &http.Client{Timeout: cfg.httpTimeout()}
	awsCfg := aws.Config{
		Region:      cfg.Region,
		Credentials: cfg.Credentials,
		HTTPClient:  httpClient,
	}
	client := awssqs.NewFromConfig(awsCfg, func(o *awssqs.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})

	q := &Queue{
		client:            client,
		queueURL:          cfg.QueueURL,
		poisonQueueURL:    cfg.PoisonQueueURL,
		visibilityTimeout: cfg.VisibilityTimeout,
		clock:             clock,
		inflight:          make(map[string]*inflightClaim),
	}

	if q.poisonQueueURL == "" {
		poisonName := poisonQueueName(queueNameFromURL(cfg.QueueURL))
		out, err := client.CreateQueue(ctx, &awssqs.CreateQueueInput{QueueName: aws.String(poisonName)})
		if err != nil {
			return nil, fmt.Errorf("sqs: creating poison queue %q: %w", poisonName, err)
		}
		q.poisonQueueURL = aws.ToString(out.QueueUrl)
	}

	return q, nil
}

// Enqueue implements internal/coachapi/queue.TaskQueue.
func (q *Queue) Enqueue(ctx context.Context, task queue.Task) error {
	body, err := json.Marshal(wireTask{ID: task.ID, Payload: task.Payload})
	if err != nil {
		return fmt.Errorf("sqs: encoding task %q: %w", task.ID, err)
	}
	_, err = q.client.SendMessage(ctx, &awssqs.SendMessageInput{
		QueueUrl:    aws.String(q.queueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		return fmt.Errorf("sqs: SendMessage for task %q: %w", task.ID, err)
	}
	return nil
}

// Claim implements internal/coachapi/queue.TaskQueue. It first reaps any
// locally-tracked claim whose deadline (per the injected Clock) has passed
// -- see the package doc comment's "Reclaim mechanism" section -- then
// attempts to receive one message from the main queue. q.mu is held only
// while reading or writing q.inflight itself (inside reapExpired's
// snapshot/delete steps and this method's final map write below); every
// SQS network call (reapExpired's ChangeMessageVisibility calls,
// ReceiveMessage, and the JSON decode in between) runs unlocked, so a slow
// or unreachable SQS endpoint cannot stall concurrent Complete/Nack/Claim
// calls.
func (q *Queue) Claim(ctx context.Context) (queue.Claim, bool, error) {
	if err := q.reapExpired(ctx); err != nil {
		return queue.Claim{}, false, err
	}

	out, err := q.client.ReceiveMessage(ctx, &awssqs.ReceiveMessageInput{
		QueueUrl:            aws.String(q.queueURL),
		MaxNumberOfMessages: 1,
		VisibilityTimeout:   int32(q.visibilityTimeout.Seconds()),
		WaitTimeSeconds:     0,
		MessageSystemAttributeNames: []types.MessageSystemAttributeName{
			types.MessageSystemAttributeNameApproximateReceiveCount,
		},
	})
	if err != nil {
		return queue.Claim{}, false, fmt.Errorf("sqs: ReceiveMessage: %w", err)
	}
	if len(out.Messages) == 0 {
		return queue.Claim{}, false, nil
	}

	msg := out.Messages[0]
	var wt wireTask
	if err := json.Unmarshal([]byte(aws.ToString(msg.Body)), &wt); err != nil {
		return queue.Claim{}, false, fmt.Errorf("sqs: decoding received message body: %w", err)
	}

	// ApproximateReceiveCount is SQS's 1-based delivery count (first
	// delivery is "1"). Queue.Claim reports 0-based Attempt, matching the
	// redisstream adapter's first-claim convention (jobs.attempt in
	// internal/coachapi/migrations/0001_init.sql), so callers see
	// consistent semantics across both TaskQueue backends.
	attempt := 0
	if raw, ok := msg.Attributes[string(types.MessageSystemAttributeNameApproximateReceiveCount)]; ok {
		if n, err := strconv.Atoi(raw); err == nil && n > 1 {
			attempt = n - 1
		}
	}

	token := aws.ToString(msg.ReceiptHandle)
	q.mu.Lock()
	q.inflight[token] = &inflightClaim{
		taskID:        wt.ID,
		receiptHandle: token,
		payload:       wt.Payload,
		attempt:       attempt,
		deadline:      q.clock.Now().Add(q.visibilityTimeout),
	}
	q.mu.Unlock()

	return queue.Claim{TaskID: wt.ID, Attempt: attempt, Token: token}, true, nil
}

// reapExpired resets SQS visibility to 0 for every locally-tracked claim
// whose deadline (per the injected Clock) has passed, so the next
// ReceiveMessage can redeliver it, then forgets the stale receipt handle so
// a later Complete/Nack carrying it is rejected. It takes q.mu only to
// snapshot the expired entries and again to remove them; the
// ChangeMessageVisibility network calls in between run unlocked, so a
// slow/unreachable SQS endpoint reclaiming one stale claim cannot stall a
// concurrent Complete/Nack/Claim call on an unrelated claim.
func (q *Queue) reapExpired(ctx context.Context) error {
	now := q.clock.Now()

	type expiredClaim struct {
		token         string
		taskID        string
		receiptHandle string
	}

	q.mu.Lock()
	var expired []expiredClaim
	for token, claim := range q.inflight {
		if now.Before(claim.deadline) {
			continue
		}
		expired = append(expired, expiredClaim{token: token, taskID: claim.taskID, receiptHandle: claim.receiptHandle})
	}
	q.mu.Unlock()

	for _, c := range expired {
		_, err := q.client.ChangeMessageVisibility(ctx, &awssqs.ChangeMessageVisibilityInput{
			QueueUrl:          aws.String(q.queueURL),
			ReceiptHandle:     aws.String(c.receiptHandle),
			VisibilityTimeout: 0,
		})
		if err != nil && !isReceiptHandleInvalid(err) {
			return fmt.Errorf("sqs: reclaiming task %q: %w", c.taskID, err)
		}

		q.mu.Lock()
		// Only delete if this token is still the entry we snapshotted: a
		// concurrent Complete/Nack may have already removed it (the
		// original worker finished just as its deadline was judged
		// expired here), and deleting an already-absent key is a no-op we
		// want to skip rather than risk racing a legitimate completion.
		if _, ok := q.inflight[c.token]; ok {
			delete(q.inflight, c.token)
		}
		q.mu.Unlock()
	}
	return nil
}

// Complete implements internal/coachapi/queue.TaskQueue.
func (q *Queue) Complete(ctx context.Context, claim queue.Claim) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	entry, ok := q.inflight[claim.Token]
	if !ok || entry.taskID != claim.TaskID {
		return fmt.Errorf("%w: task %q", errStaleClaim, claim.TaskID)
	}

	_, err := q.client.DeleteMessage(ctx, &awssqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.queueURL),
		ReceiptHandle: aws.String(claim.Token),
	})
	if err != nil {
		return fmt.Errorf("sqs: DeleteMessage for task %q: %w", claim.TaskID, err)
	}

	delete(q.inflight, claim.Token)
	return nil
}

// Nack implements internal/coachapi/queue.TaskQueue. permanent=false resets
// the message's SQS visibility to 0, making it immediately reclaimable.
// permanent=true copies the task to the poison-task destination queue and
// deletes it from the main queue, so it can never be claimed again (see the
// package doc comment's "Poison-task destination" section).
func (q *Queue) Nack(ctx context.Context, claim queue.Claim, permanent bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	entry, ok := q.inflight[claim.Token]
	if !ok || entry.taskID != claim.TaskID {
		return fmt.Errorf("%w: task %q", errStaleClaim, claim.TaskID)
	}

	if permanent {
		body, err := json.Marshal(wireTask{ID: claim.TaskID, Payload: entry.payload})
		if err != nil {
			return fmt.Errorf("sqs: encoding poisoned task %q: %w", claim.TaskID, err)
		}
		if _, err := q.client.SendMessage(ctx, &awssqs.SendMessageInput{
			QueueUrl:    aws.String(q.poisonQueueURL),
			MessageBody: aws.String(string(body)),
		}); err != nil {
			return fmt.Errorf("sqs: sending task %q to poison queue: %w", claim.TaskID, err)
		}
		if _, err := q.client.DeleteMessage(ctx, &awssqs.DeleteMessageInput{
			QueueUrl:      aws.String(q.queueURL),
			ReceiptHandle: aws.String(claim.Token),
		}); err != nil {
			return fmt.Errorf("sqs: deleting poisoned task %q from main queue: %w", claim.TaskID, err)
		}
		delete(q.inflight, claim.Token)
		return nil
	}

	if _, err := q.client.ChangeMessageVisibility(ctx, &awssqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(q.queueURL),
		ReceiptHandle:     aws.String(claim.Token),
		VisibilityTimeout: 0,
	}); err != nil {
		return fmt.Errorf("sqs: retryable Nack for task %q: %w", claim.TaskID, err)
	}
	delete(q.inflight, claim.Token)
	return nil
}

// PoisonTasks returns the tasks currently observable on the poison-task
// destination queue, up to maxPoisonDrainRounds drain rounds; it is a
// bounded, best-effort enumeration, not guaranteed to be exhaustive under
// pathological redelivery timing. It receives with VisibilityTimeout: 0
// rather than deleting, so repeated calls keep observing the same poisoned
// tasks instead of draining the queue (see the package doc comment's
// "Poison-task destination" section).
func (q *Queue) PoisonTasks(ctx context.Context) ([]queue.Task, error) {
	var tasks []queue.Task
	seen := make(map[string]bool)

	for round := 0; round < maxPoisonDrainRounds; round++ {
		out, err := q.client.ReceiveMessage(ctx, &awssqs.ReceiveMessageInput{
			QueueUrl:            aws.String(q.poisonQueueURL),
			MaxNumberOfMessages: 10,
			VisibilityTimeout:   0,
			WaitTimeSeconds:     0,
		})
		if err != nil {
			return nil, fmt.Errorf("sqs: ReceiveMessage on poison queue: %w", err)
		}
		if len(out.Messages) == 0 {
			break
		}

		progressed := false
		for _, msg := range out.Messages {
			var wt wireTask
			if err := json.Unmarshal([]byte(aws.ToString(msg.Body)), &wt); err != nil {
				return nil, fmt.Errorf("sqs: decoding poison queue message body: %w", err)
			}
			key := aws.ToString(msg.MessageId)
			if seen[key] {
				continue
			}
			seen[key] = true
			progressed = true
			tasks = append(tasks, queue.Task{ID: wt.ID, Payload: wt.Payload})
		}
		if !progressed {
			// Every message in this round was already seen: SQS is
			// re-serving the same immediately-visible messages, so
			// further rounds cannot make progress.
			break
		}
	}

	return tasks, nil
}

// isReceiptHandleInvalid reports whether err is SQS's ReceiptHandleIsInvalid
// error, the expected outcome when this adapter tries to reset visibility
// on a message another path (a concurrent reclaim, a redelivery) has
// already invalidated the receipt handle for.
func isReceiptHandleInvalid(err error) bool {
	var apiErr interface{ ErrorCode() string }
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == "ReceiptHandleIsInvalid"
	}
	return false
}

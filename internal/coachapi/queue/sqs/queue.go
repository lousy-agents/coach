package sqs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"

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

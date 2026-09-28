package sqs

import (
	"context"

	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/lousy-agents/coach/internal/acceptanceharness"

	"net/http"
)

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

		if _, ok := q.inflight[c.token]; ok {
			delete(q.inflight, c.token)
		}
		q.mu.Unlock()
	}
	return nil
}

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

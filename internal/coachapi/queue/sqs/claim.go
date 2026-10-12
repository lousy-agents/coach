package sqs

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

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

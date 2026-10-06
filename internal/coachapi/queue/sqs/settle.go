package sqs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

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

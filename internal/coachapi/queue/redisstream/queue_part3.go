package redisstream

import (
	"context"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

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
func (q *Queue) publishPoison(ctx context.Context, taskID string, payload []byte) error {
	msg := message.NewMessage(watermill.NewUUID(), payload)
	msg.Metadata.Set(taskIDMetadataKey, taskID)
	msg.SetContext(ctx)

	if err := q.publisher.Publish(q.poisonStream, msg); err != nil {
		return fmt.Errorf("redisstream: publishing task %q to poison destination %q: %w", taskID, q.poisonStream, err)
	}
	return nil
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

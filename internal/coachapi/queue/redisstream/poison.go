package redisstream

import (
	"context"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

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

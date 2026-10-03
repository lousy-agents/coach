package sqs

import (
	"context"

	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// PoisonTasks returns the tasks currently observable on the poison-task
// destination queue, up to maxPoisonDrainRounds drain rounds; it is a
// bounded, best-effort enumeration, not guaranteed to be exhaustive under
// pathological redelivery timing. It receives with VisibilityTimeout: 0
// rather than deleting, so repeated calls keep observing the same poisoned
// tasks instead of draining the queue (see the package doc comment's
// "Poison-task destination" section).
func (q *Queue) PoisonTasks(ctx context.Context) ([]queue.Task, error) {
	var tasks []queue.Task
	drain := poisonDrain{seen: make(map[string]bool)}

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

		batch, progressed, err := drain.decodeRound(out.Messages)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, batch...)
		if !progressed {

			break
		}
	}

	return tasks, nil
}

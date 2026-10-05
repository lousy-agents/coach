package sqs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// maxPoisonDrainRounds bounds PoisonTasks's ReceiveMessage loop so a
// pathologically large poison queue cannot make a single call block
// forever; each round can return up to 10 messages (SQS's own
// MaxNumberOfMessages cap).
const maxPoisonDrainRounds = 50

type poisonDrain struct {
	seen map[string]bool
}

// decodeRound decodes one ReceiveMessage batch, skipping message IDs
// already recorded on the drain. progressed is false when every message in
// the batch was already seen -- SQS re-serving the same immediately-visible
// messages, which means further rounds cannot make progress.
func (d *poisonDrain) decodeRound(msgs []types.Message) (tasks []queue.Task, progressed bool, err error) {
	for _, msg := range msgs {
		var wt wireTask
		if err := json.Unmarshal([]byte(aws.ToString(msg.Body)), &wt); err != nil {
			return nil, false, fmt.Errorf("sqs: decoding poison queue message body: %w", err)
		}
		key := aws.ToString(msg.MessageId)
		if d.seen[key] {
			continue
		}
		d.seen[key] = true
		progressed = true
		tasks = append(tasks, queue.Task{ID: wt.ID, Payload: wt.Payload})
	}
	return tasks, progressed, nil
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
			// Every message in this round was already seen: SQS is
			// re-serving the same immediately-visible messages, so
			// further rounds cannot make progress.
			break
		}
	}

	return tasks, nil
}

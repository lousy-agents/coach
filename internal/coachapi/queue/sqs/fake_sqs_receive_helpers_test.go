package sqs

import (
	"context"
	"strconv"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

func (f *fakeSQS) ReceiveMessage(ctx context.Context, in *awssqs.ReceiveMessageInput, _ ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	url := *in.QueueUrl
	max := int(in.MaxNumberOfMessages)
	if max <= 0 {
		max = 1
	}

	var out []types.Message
	for _, msg := range f.queues[url] {
		if !msg.visible {
			continue
		}
		msg.receiveCount++
		f.nextID++
		msg.receiptHandle = "rh-" + strconv.Itoa(f.nextID)
		if in.VisibilityTimeout > 0 {
			msg.visible = false
		}
		body := msg.body
		id := msg.id
		rh := msg.receiptHandle
		out = append(out, types.Message{
			MessageId:     &id,
			Body:          &body,
			ReceiptHandle: &rh,
			Attributes: map[string]string{
				string(types.MessageSystemAttributeNameApproximateReceiveCount): strconv.Itoa(msg.receiveCount),
			},
		})
		if len(out) >= max {
			break
		}
	}
	return &awssqs.ReceiveMessageOutput{Messages: out}, nil
}

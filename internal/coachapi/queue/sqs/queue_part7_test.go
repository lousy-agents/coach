package sqs

import (
	"context"

	"strconv"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
)

func (f *fakeSQS) SendMessage(ctx context.Context, in *awssqs.SendMessageInput, _ ...func(*awssqs.Options)) (*awssqs.SendMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.nextID++
	id := strconv.Itoa(f.nextID)
	url := *in.QueueUrl
	f.queues[url] = append(f.queues[url], &fakeMessage{id: id, body: *in.MessageBody, visible: true})
	return &awssqs.SendMessageOutput{MessageId: &id}, nil
}

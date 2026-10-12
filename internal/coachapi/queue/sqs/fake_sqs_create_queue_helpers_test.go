package sqs

import (
	"context"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
)

func (f *fakeSQS) CreateQueue(ctx context.Context, in *awssqs.CreateQueueInput, _ ...func(*awssqs.Options)) (*awssqs.CreateQueueOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	name := *in.QueueName
	url, ok := f.queueURL[name]
	if !ok {
		url = "https://fake.local/queues/" + name
		f.queueURL[name] = url
		f.queues[url] = nil
	}
	return &awssqs.CreateQueueOutput{QueueUrl: &url}, nil
}

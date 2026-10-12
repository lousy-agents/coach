package sqs

import (
	"context"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
)

func (f *fakeSQS) DeleteMessage(ctx context.Context, in *awssqs.DeleteMessageInput, _ ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	url := *in.QueueUrl
	i := f.receiptIndex(url, *in.ReceiptHandle)
	if i < 0 {
		return nil, fakeAPIError{code: "ReceiptHandleIsInvalid"}
	}
	msgs := f.queues[url]
	f.queues[url] = append(msgs[:i], msgs[i+1:]...)
	return &awssqs.DeleteMessageOutput{}, nil
}

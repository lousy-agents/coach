package sqs

import (
	"context"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
)

func (f *fakeSQS) ChangeMessageVisibility(ctx context.Context, in *awssqs.ChangeMessageVisibilityInput, _ ...func(*awssqs.Options)) (*awssqs.ChangeMessageVisibilityOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	url := *in.QueueUrl
	i := f.receiptIndex(url, *in.ReceiptHandle)
	if i < 0 {
		return nil, fakeAPIError{code: "ReceiptHandleIsInvalid"}
	}
	f.queues[url][i].visible = in.VisibilityTimeout == 0
	return &awssqs.ChangeMessageVisibilityOutput{}, nil
}

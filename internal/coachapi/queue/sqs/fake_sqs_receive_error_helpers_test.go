package sqs

import (
	"context"
	"errors"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
)

// erroringSQS wraps fakeSQS but makes ReceiveMessage always fail, to prove
// Claim wraps and surfaces the underlying error.
type erroringSQS struct{ *fakeSQS }

func (e erroringSQS) ReceiveMessage(context.Context, *awssqs.ReceiveMessageInput, ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
	return nil, errors.New("boom")
}

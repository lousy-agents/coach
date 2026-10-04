package sqs

import (
	"context"
	"errors"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
)

func (e erroringSQS) ReceiveMessage(context.Context, *awssqs.ReceiveMessageInput, ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
	return nil, errors.New("boom")
}

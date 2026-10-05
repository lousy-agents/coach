package sqs

import (
	"context"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
)

func (b blockingVisibilitySQS) ChangeMessageVisibility(ctx context.Context, in *awssqs.ChangeMessageVisibilityInput, opts ...func(*awssqs.Options)) (*awssqs.ChangeMessageVisibilityOutput, error) {
	<-b.unblock
	return b.fakeSQS.ChangeMessageVisibility(ctx, in, opts...)
}

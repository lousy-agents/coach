package sqs

import (
	"context"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
)

// blockingVisibilitySQS wraps fakeSQS but makes ChangeMessageVisibility
// block until unblock is closed, so a test can prove reapExpired's network
// call for one stale claim does not hold q.mu and therefore cannot stall a
// concurrent Complete/Nack/Claim call on an unrelated claim.
type blockingVisibilitySQS struct {
	*fakeSQS
	unblock <-chan struct{}
}

func (b blockingVisibilitySQS) ChangeMessageVisibility(ctx context.Context, in *awssqs.ChangeMessageVisibilityInput, opts ...func(*awssqs.Options)) (*awssqs.ChangeMessageVisibilityOutput, error) {
	<-b.unblock
	return b.fakeSQS.ChangeMessageVisibility(ctx, in, opts...)
}

package sqs_test

import (
	"context"

	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/acceptanceharness/queueconformance"
	sqsqueue "github.com/lousy-agents/coach/internal/coachapi/queue/sqs"
)

func body_sqsConformanceTest_81(tb testing.TB, clock acceptanceharness.Clock, endpoint string, queueURL string) queueconformance.Queue {
	cfg := sqsqueue.Config{
		Region:            "us-east-1",
		QueueURL:          queueURL,
		VisibilityTimeout: time.Minute,
		Endpoint:          endpoint,

		Credentials: credentials.NewStaticCredentialsProvider("localstack-fake-access-key", "localstack-fake-secret-key", ""),
	}
	q, err := sqsqueue.NewQueue(context.Background(), cfg, clock)
	if err != nil {
		tb.Fatalf("sqs.NewQueue: %v", err)
	}
	return conformanceAdapter{q: q}
}

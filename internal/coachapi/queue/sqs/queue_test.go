package sqs

import (
	"context"

	"sync"
	"testing"
	"time"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// fakeMessage is one in-flight or pending message tracked by fakeSQS.
type fakeMessage struct {
	id            string
	body          string
	receiptHandle string
	receiveCount  int
	visible       bool
}

// fakeAPIError implements the ErrorCode() interface isReceiptHandleInvalid
// checks for, without depending on the real smithy-go error types.
type fakeAPIError struct{ code string }

// fakeSQS is a minimal in-memory stand-in for sqsAPI, exercising exactly
// the operations Queue calls, so Queue's own translation logic (reclaim,
// stale-token rejection, poison routing) is unit-testable without Docker or
// LocalStack.
type fakeSQS struct {
	mu       sync.Mutex
	nextID   int
	queues   map[string][]*fakeMessage // queueURL -> messages
	queueURL map[string]string         // queue name -> URL, for CreateQueue
}

// erroringSQS wraps fakeSQS but makes ReceiveMessage always fail, to prove
// Claim wraps and surfaces the underlying error.
type erroringSQS struct{ *fakeSQS }

// blockingVisibilitySQS wraps fakeSQS but makes ChangeMessageVisibility
// block until unblock is closed, so a test can prove reapExpired's network
// call for one stale claim does not hold q.mu and therefore cannot stall a
// concurrent Complete/Nack/Claim call on an unrelated claim.
type blockingVisibilitySQS struct {
	*fakeSQS
	unblock <-chan struct{}
}

func TestQueueNackPermanentRoutesToPoisonQueue(t *testing.T) {
	ctx := context.Background()
	api := newFakeSQS()
	api.queues["https://fake.local/queues/main"] = nil
	api.queues["https://fake.local/queues/main-poison"] = nil
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newTestQueue(t, api, clock)

	if err := q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("payload")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	claim, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("Claim: ok=%v err=%v", ok, err)
	}

	if err := q.Nack(ctx, claim, true); err != nil {
		t.Fatalf("Nack(true): %v", err)
	}

	poisoned, err := q.PoisonTasks(ctx)
	if err != nil {
		t.Fatalf("PoisonTasks: %v", err)
	}
	if len(poisoned) != 1 || poisoned[0].ID != "task-1" || string(poisoned[0].Payload) != "payload" {
		t.Fatalf("PoisonTasks() = %+v, want one task-1/payload entry", poisoned)
	}

	clock.Advance(24 * time.Hour)
	if _, ok, err := q.Claim(ctx); err != nil || ok {
		t.Fatalf("Claim after permanent Nack: ok=%v err=%v, want ok=false", ok, err)
	}

	poisonedAgain, err := q.PoisonTasks(ctx)
	if err != nil {
		t.Fatalf("second PoisonTasks: %v", err)
	}
	if len(poisonedAgain) != 1 {
		t.Fatalf("second PoisonTasks() = %+v, want it to still report task-1", poisonedAgain)
	}
}

func (f *fakeSQS) DeleteMessage(ctx context.Context, in *awssqs.DeleteMessageInput, _ ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	url := *in.QueueUrl
	msg := f.findByReceiptHandle(url, *in.ReceiptHandle)
	if msg == nil {
		return nil, fakeAPIError{code: "ReceiptHandleIsInvalid"}
	}
	msgs := f.queues[url]
	for i, m := range msgs {
		if m == msg {
			f.queues[url] = append(msgs[:i], msgs[i+1:]...)
			break
		}
	}
	return &awssqs.DeleteMessageOutput{}, nil
}

func (e fakeAPIError) Error() string { return e.code }

func (e fakeAPIError) ErrorCode() string { return e.code }

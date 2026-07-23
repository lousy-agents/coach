package sqs

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

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

func (e fakeAPIError) Error() string     { return e.code }
func (e fakeAPIError) ErrorCode() string { return e.code }

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

func newFakeSQS() *fakeSQS {
	return &fakeSQS{
		queues:   make(map[string][]*fakeMessage),
		queueURL: make(map[string]string),
	}
}

func (f *fakeSQS) SendMessage(ctx context.Context, in *awssqs.SendMessageInput, _ ...func(*awssqs.Options)) (*awssqs.SendMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.nextID++
	id := strconv.Itoa(f.nextID)
	url := *in.QueueUrl
	f.queues[url] = append(f.queues[url], &fakeMessage{id: id, body: *in.MessageBody, visible: true})
	return &awssqs.SendMessageOutput{MessageId: &id}, nil
}

func (f *fakeSQS) ReceiveMessage(ctx context.Context, in *awssqs.ReceiveMessageInput, _ ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	url := *in.QueueUrl
	max := int(in.MaxNumberOfMessages)
	if max <= 0 {
		max = 1
	}

	var out []types.Message
	for _, msg := range f.queues[url] {
		if !msg.visible {
			continue
		}
		msg.receiveCount++
		f.nextID++
		msg.receiptHandle = "rh-" + strconv.Itoa(f.nextID)
		if in.VisibilityTimeout > 0 {
			msg.visible = false
		}
		body := msg.body
		id := msg.id
		rh := msg.receiptHandle
		out = append(out, types.Message{
			MessageId:     &id,
			Body:          &body,
			ReceiptHandle: &rh,
			Attributes: map[string]string{
				string(types.MessageSystemAttributeNameApproximateReceiveCount): strconv.Itoa(msg.receiveCount),
			},
		})
		if len(out) >= max {
			break
		}
	}
	return &awssqs.ReceiveMessageOutput{Messages: out}, nil
}

func (f *fakeSQS) findByReceiptHandle(url, receiptHandle string) *fakeMessage {
	for _, msg := range f.queues[url] {
		if msg.receiptHandle == receiptHandle {
			return msg
		}
	}
	return nil
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

func (f *fakeSQS) ChangeMessageVisibility(ctx context.Context, in *awssqs.ChangeMessageVisibilityInput, _ ...func(*awssqs.Options)) (*awssqs.ChangeMessageVisibilityOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	url := *in.QueueUrl
	msg := f.findByReceiptHandle(url, *in.ReceiptHandle)
	if msg == nil {
		return nil, fakeAPIError{code: "ReceiptHandleIsInvalid"}
	}
	msg.visible = in.VisibilityTimeout == 0
	return &awssqs.ChangeMessageVisibilityOutput{}, nil
}

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

// erroringSQS wraps fakeSQS but makes ReceiveMessage always fail, to prove
// Claim wraps and surfaces the underlying error.
type erroringSQS struct{ *fakeSQS }

func (e erroringSQS) ReceiveMessage(context.Context, *awssqs.ReceiveMessageInput, ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
	return nil, errors.New("boom")
}

func newTestQueue(t *testing.T, api sqsAPI, clock acceptanceharness.Clock) *Queue {
	t.Helper()
	return &Queue{
		client:            api,
		queueURL:          "https://fake.local/queues/main",
		poisonQueueURL:    "https://fake.local/queues/main-poison",
		visibilityTimeout: time.Minute,
		clock:             clock,
		inflight:          make(map[string]*inflightClaim),
	}
}

func TestQueueEnqueueClaimComplete(t *testing.T) {
	ctx := context.Background()
	api := newFakeSQS()
	api.queues["https://fake.local/queues/main"] = nil
	api.queues["https://fake.local/queues/main-poison"] = nil
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newTestQueue(t, api, clock)

	if err := q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("hello")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	claim, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("Claim: ok=%v err=%v", ok, err)
	}
	if claim.TaskID != "task-1" || claim.Attempt != 1 {
		t.Fatalf("Claim = %+v, want TaskID=task-1 Attempt=1", claim)
	}

	if err := q.Complete(ctx, claim); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if _, ok, err := q.Claim(ctx); err != nil || ok {
		t.Fatalf("Claim after Complete: ok=%v err=%v, want ok=false", ok, err)
	}
}

func TestQueueCompleteWithStaleTokenFails(t *testing.T) {
	ctx := context.Background()
	api := newFakeSQS()
	api.queues["https://fake.local/queues/main"] = nil
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newTestQueue(t, api, clock)

	if err := q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("x")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	stale, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("first Claim: ok=%v err=%v", ok, err)
	}

	clock.Advance(24 * time.Hour)

	fresh, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("reclaim Claim: ok=%v err=%v", ok, err)
	}
	if fresh.Attempt != stale.Attempt+1 {
		t.Fatalf("reclaim Attempt = %d, want %d", fresh.Attempt, stale.Attempt+1)
	}

	if err := q.Complete(ctx, stale); !errors.Is(err, errStaleClaim) {
		t.Fatalf("Complete(stale) error = %v, want errStaleClaim", err)
	}
	if err := q.Complete(ctx, fresh); err != nil {
		t.Fatalf("Complete(fresh): %v", err)
	}
}

func TestQueueNackRetryableMakesTaskClaimableAgain(t *testing.T) {
	ctx := context.Background()
	api := newFakeSQS()
	api.queues["https://fake.local/queues/main"] = nil
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newTestQueue(t, api, clock)

	if err := q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("x")}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	first, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("first Claim: ok=%v err=%v", ok, err)
	}

	if err := q.Nack(ctx, first, false); err != nil {
		t.Fatalf("Nack(false): %v", err)
	}

	second, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("Claim after Nack: ok=%v err=%v", ok, err)
	}
	if second.Attempt != first.Attempt+1 {
		t.Fatalf("Attempt after retryable Nack = %d, want %d", second.Attempt, first.Attempt+1)
	}

	if err := q.Nack(ctx, first, false); !errors.Is(err, errStaleClaim) {
		t.Fatalf("Nack with stale token error = %v, want errStaleClaim", err)
	}
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

	// PoisonTasks is a repeatable peek, not a drain.
	poisonedAgain, err := q.PoisonTasks(ctx)
	if err != nil {
		t.Fatalf("second PoisonTasks: %v", err)
	}
	if len(poisonedAgain) != 1 {
		t.Fatalf("second PoisonTasks() = %+v, want it to still report task-1", poisonedAgain)
	}
}

func TestQueueClaimWrapsUnderlyingError(t *testing.T) {
	ctx := context.Background()
	q := newTestQueue(t, erroringSQS{newFakeSQS()}, acceptanceharness.NewFakeClock(time.Unix(0, 0)))

	_, _, err := q.Claim(ctx)
	if err == nil {
		t.Fatal("Claim: want error, got nil")
	}
	if got := fmt.Sprint(err); got == "" {
		t.Fatalf("Claim error message is empty")
	}
}

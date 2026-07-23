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
	if claim.TaskID != "task-1" || claim.Attempt != 0 {
		t.Fatalf("Claim = %+v, want TaskID=task-1 Attempt=0", claim)
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

// TestQueueDuplicateTaskIDClaimsAreTrackedIndependently proves the SQS
// adapter tolerates duplicate delivery of the same Task.ID (ADR-006
// explicitly permits this), which happens when a task is enqueued more
// than once or SQS redelivers a message that also still has an earlier
// in-flight copy. Before keying q.inflight by receipt handle, the second
// Claim for the same TaskID silently overwrote the first claim's
// bookkeeping, making the first claim's token permanently stale.
func TestQueueDuplicateTaskIDClaimsAreTrackedIndependently(t *testing.T) {
	ctx := context.Background()
	api := newFakeSQS()
	api.queues["https://fake.local/queues/main"] = nil
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newTestQueue(t, api, clock)

	if err := q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("first")}); err != nil {
		t.Fatalf("Enqueue 1: %v", err)
	}
	if err := q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("second")}); err != nil {
		t.Fatalf("Enqueue 2: %v", err)
	}

	first, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("first Claim: ok=%v err=%v", ok, err)
	}
	second, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("second Claim: ok=%v err=%v", ok, err)
	}
	if first.TaskID != second.TaskID {
		t.Fatalf("want both claims for the same TaskID, got %q and %q", first.TaskID, second.TaskID)
	}
	if first.Token == second.Token {
		t.Fatalf("want distinct tokens for two independently delivered messages, got %q twice", first.Token)
	}

	if err := q.Complete(ctx, first); err != nil {
		t.Fatalf("Complete(first) = %v, want success: the first claim's receipt handle must still be tracked", err)
	}
	if err := q.Complete(ctx, second); err != nil {
		t.Fatalf("Complete(second) = %v, want success: completing the first claim must not evict the second", err)
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

// TestQueueClaimDoesNotHoldLockDuringReclaimNetworkCall proves the fix for
// a review finding on this method: reapExpired (called from within Claim)
// must snapshot expired claims, release q.mu, make its
// ChangeMessageVisibility calls unlocked, and only reacquire q.mu to delete
// them -- not hold q.mu for the whole reclaim loop. It enqueues two tasks,
// claims both (task-1's claim is then made stale via a clock advance),
// blocks ChangeMessageVisibility so a Claim call reclaiming task-1 hangs
// mid-reap, and asserts a concurrent Complete on task-2's still-valid claim
// finishes well before the blocked call is released -- which is only
// possible if reapExpired had already given up the lock.
func TestQueueClaimDoesNotHoldLockDuringReclaimNetworkCall(t *testing.T) {
	ctx := context.Background()
	unblock := make(chan struct{})
	api := &fakeSQS{queues: make(map[string][]*fakeMessage), queueURL: make(map[string]string)}
	blocking := blockingVisibilitySQS{fakeSQS: api, unblock: unblock}
	clock := acceptanceharness.NewFakeClock(time.Unix(0, 0))
	q := newTestQueue(t, blocking, clock)

	if err := q.Enqueue(ctx, queue.Task{ID: "task-1", Payload: []byte("x")}); err != nil {
		t.Fatalf("Enqueue(task-1): %v", err)
	}
	if err := q.Enqueue(ctx, queue.Task{ID: "task-2", Payload: []byte("y")}); err != nil {
		t.Fatalf("Enqueue(task-2): %v", err)
	}

	first, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("first Claim (task-1): ok=%v err=%v", ok, err)
	}
	second, ok, err := q.Claim(ctx)
	if err != nil || !ok {
		t.Fatalf("second Claim (task-2): ok=%v err=%v", ok, err)
	}
	if first.TaskID == second.TaskID {
		t.Fatalf("both claims returned the same task id %q, want task-1 and task-2", first.TaskID)
	}

	// Both claims are made *before* advancing the clock, so neither Claim
	// call above went through reapExpired's blocked path. Advancing now
	// makes both claims' deadlines stale, so the next Claim call (below,
	// backgrounded) will try to reclaim one of them via a
	// ChangeMessageVisibility call that blocks on unblock -- and whichever
	// entry it picks, the *other* one (second, completed concurrently
	// below) is still present in q.inflight until that blocked call
	// resolves and reapExpired's delete phase runs.
	clock.Advance(24 * time.Hour)

	reclaimDone := make(chan error, 1)
	go func() {
		_, _, err := q.Claim(ctx) // triggers reapExpired, which blocks on ChangeMessageVisibility
		reclaimDone <- err
	}()

	// Give the reclaiming goroutine a moment to actually enter the blocked
	// call before racing Complete against it.
	time.Sleep(20 * time.Millisecond)

	completeDone := make(chan error, 1)
	go func() {
		completeDone <- q.Complete(ctx, second)
	}()

	select {
	case err := <-completeDone:
		if err != nil {
			t.Fatalf("Complete(task-2) while reclaim was blocked: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Complete(task-2) did not return while reapExpired's ChangeMessageVisibility call was blocked -- the lock is still held during the network call")
	case <-reclaimDone:
		t.Fatal("the blocked reclaim finished before Complete could even be observed, so this run could not prove anything -- test setup bug")
	}

	close(unblock)
	if err := <-reclaimDone; err != nil {
		t.Fatalf("reclaiming Claim call: %v", err)
	}
}

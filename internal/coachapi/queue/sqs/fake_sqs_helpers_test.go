package sqs

import (
	"sync"
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

func (e fakeAPIError) Error() string { return e.code }

func (e fakeAPIError) ErrorCode() string { return e.code }

// fakeSQS is a minimal in-memory stand-in for sqsAPI, exercising exactly
// the operations Queue calls, so Queue's own translation logic (reclaim,
// stale-token rejection, poison routing) is unit-testable without Docker or
// LocalStack. Each sqsAPI operation lives in its own fake_sqs_*_helpers_test.go.
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

// receiptIndex returns the position of the message on url currently
// holding receiptHandle, or -1 when no message holds it.
func (f *fakeSQS) receiptIndex(url, receiptHandle string) int {
	for i, msg := range f.queues[url] {
		if msg.receiptHandle == receiptHandle {
			return i
		}
	}
	return -1
}

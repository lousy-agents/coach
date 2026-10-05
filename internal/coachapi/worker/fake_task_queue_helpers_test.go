package worker_test

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

// fakeTaskQueue is an in-memory TaskQueue test double (same contract shape as
// internal/coachapi/queue's acceptance fake). Tests may use it; production
// workers must not poll Postgres for work alongside TaskQueue.
type inFlightEntry struct {
	task queue.Task
}

type fakeTaskQueue struct {
	mu         sync.Mutex
	pending    []queue.Task
	inFlight   map[string]inFlightEntry
	poison     map[string]bool
	attempts   map[string]int
	complete   []queue.Claim
	enqueueN   int
	enqueueErr error
	// permanentNackFailLeft makes the next N permanent Nack calls fail after
	// releasing the claim back to pending (simulating poison publish failure
	// while leaving the source message redeliverable).
	permanentNackFailLeft int
	permanentNackCalls    int
}

var _ queue.TaskQueue = (*fakeTaskQueue)(nil)

func newFakeTaskQueue() *fakeTaskQueue {
	return &fakeTaskQueue{
		inFlight: make(map[string]inFlightEntry),
		poison:   make(map[string]bool),
		attempts: make(map[string]int),
	}
}

func (q *fakeTaskQueue) Enqueue(_ context.Context, task queue.Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.enqueueErr != nil {
		return q.enqueueErr
	}
	q.enqueueN++
	q.pending = append(q.pending, task)
	return nil
}

func (q *fakeTaskQueue) Claim(_ context.Context) (queue.Claim, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return queue.Claim{}, false, nil
	}
	task := q.pending[0]
	q.pending = q.pending[1:]
	q.attempts[task.ID]++
	attempt := q.attempts[task.ID] - 1
	token := fmt.Sprintf("%s-attempt-%d", task.ID, q.attempts[task.ID])
	claim := queue.Claim{TaskID: task.ID, Attempt: attempt, Token: token}
	q.inFlight[token] = inFlightEntry{task: task}
	return claim, true, nil
}

func (q *fakeTaskQueue) Complete(_ context.Context, claim queue.Claim) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	entry, ok := q.inFlight[claim.Token]
	if !ok || entry.task.ID != claim.TaskID {
		return errors.New("fakeTaskQueue: stale claim")
	}
	delete(q.inFlight, claim.Token)
	q.complete = append(q.complete, claim)
	return nil
}

func (q *fakeTaskQueue) Nack(_ context.Context, claim queue.Claim, permanent bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	entry, ok := q.inFlight[claim.Token]
	if !ok || entry.task.ID != claim.TaskID {
		return errors.New("fakeTaskQueue: stale claim")
	}
	delete(q.inFlight, claim.Token)
	if permanent {
		q.permanentNackCalls++
		if q.permanentNackFailLeft > 0 {
			q.permanentNackFailLeft--

			q.pending = append(q.pending, entry.task)
			return errors.New("fakeTaskQueue: poison destination unavailable")
		}
		q.poison[claim.TaskID] = true
		return nil
	}
	q.pending = append(q.pending, entry.task)
	return nil
}

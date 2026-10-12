package worker_test

import (
	"github.com/lousy-agents/coach/internal/coachapi/queue"
)

func (q *fakeTaskQueue) completedCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.complete)
}

func (q *fakeTaskQueue) inFlightCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.inFlight)
}

func (q *fakeTaskQueue) pendingCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending)
}

func (q *fakeTaskQueue) pendingTasks() []queue.Task {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]queue.Task, len(q.pending))
	copy(out, q.pending)
	return out
}

func (q *fakeTaskQueue) enqueueCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.enqueueN
}

func (q *fakeTaskQueue) isPoisoned(taskID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.poison[taskID]
}

func (q *fakeTaskQueue) permanentNackCallCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.permanentNackCalls
}

package queueconformance

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// inMemoryQueue is a throwaway reference Queue implementation used only to
// self-validate that Run actually catches contract violations and
// green-passes a correct implementation. It is unexported and lives only
// in this _test.go file so nothing outside this package's test binary can
// import or depend on it as production code -- Baseline Task 3a's real
// Redis Streams and SQS adapters are separate, non-test packages.
type inMemoryQueue struct {
	clock             acceptanceharness.Clock
	visibilityTimeout time.Duration

	mu    sync.Mutex
	tasks map[string]*inMemoryTaskState
}

type inMemoryTaskState struct {
	task          Task
	attempt       int
	claimed       bool
	token         string
	claimDeadline time.Time
	completed     bool
	poisoned      bool
}

var (
	errUnknownTask = errors.New("queueconformance: unknown task id")
	errStaleClaim  = errors.New("queueconformance: claim token invalidated by reclaim")
)

func newInMemoryQueue(clock acceptanceharness.Clock, visibilityTimeout time.Duration) *inMemoryQueue {
	return &inMemoryQueue{
		clock:             clock,
		visibilityTimeout: visibilityTimeout,
		tasks:             make(map[string]*inMemoryTaskState),
	}
}

func (q *inMemoryQueue) Enqueue(ctx context.Context, task Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.tasks[task.ID] = &inMemoryTaskState{task: task}
	return nil
}

func (q *inMemoryQueue) Claim(ctx context.Context) (Claim, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := q.clock.Now()
	for _, state := range q.tasks {
		if state.completed || state.poisoned {
			continue
		}
		if state.claimed && now.Before(state.claimDeadline) {
			continue
		}
		state.attempt++
		state.claimed = true
		state.token = fmt.Sprintf("%s-attempt-%d", state.task.ID, state.attempt)
		state.claimDeadline = now.Add(q.visibilityTimeout)
		return Claim{TaskID: state.task.ID, Attempt: state.attempt, Token: state.token}, true, nil
	}
	return Claim{}, false, nil
}

func (q *inMemoryQueue) Complete(ctx context.Context, claim Claim) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	state, found := q.tasks[claim.TaskID]
	if !found {
		return errUnknownTask
	}
	// A reclaim overwrites state.token with a fresh value, so a Complete
	// carrying the pre-reclaim token proves the original worker was
	// "killed" and must not be allowed to record a duplicate completion.
	if state.token != claim.Token {
		return errStaleClaim
	}

	state.completed = true
	return nil
}

func (q *inMemoryQueue) Nack(ctx context.Context, claim Claim, permanent bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	state, found := q.tasks[claim.TaskID]
	if !found {
		return errUnknownTask
	}
	if state.token != claim.Token {
		return errStaleClaim
	}

	if permanent {
		state.poisoned = true
		state.claimed = false
		return nil
	}

	// A retryable Nack makes the task claimable again immediately,
	// exactly like a reclaim, rather than waiting for claimDeadline to
	// elapse -- so the token is invalidated (via the next Claim's
	// attempt/token bump) by clearing claimed and letting Claim's normal
	// path pick it up.
	state.claimed = false
	return nil
}

func (q *inMemoryQueue) PoisonTasks(ctx context.Context) ([]Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	var poisoned []Task
	for _, state := range q.tasks {
		if state.poisoned {
			poisoned = append(poisoned, state.task)
		}
	}
	return poisoned, nil
}

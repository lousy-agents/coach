package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"time"

	. "github.com/onsi/ginkgo/v2"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
	"github.com/lousy-agents/coach/internal/coachapi/worker"
)

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

// advanceUntil advances clock in small steps until cond is true or max
// total advance is exhausted. Used so FakeClock-driven heartbeats fire
// without wall-clock sleeps.
func advanceUntil(clock *acceptanceharness.FakeClock, step time.Duration, max time.Duration, cond func() bool) {
	GinkgoHelper()
	deadline := time.After(2 * time.Second)
	advanced := time.Duration(0)
	for advanced <= max {
		if cond() {
			return
		}
		select {
		case <-deadline:
			return
		default:
		}
		clock.Advance(step)
		advanced += step

		time.Sleep(time.Millisecond)
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

func successHandler(_ context.Context, job coachapi.Job, w worker.JobWriter) (*coachapi.Completion, error) {
	lease := w.Lease()
	finding := coachapi.JobFinding{
		ID:          fmt.Sprintf("f-%s-%d", job.ID, lease.Attempt),
		JobID:       job.ID,
		Attempt:     lease.Attempt,
		Source:      coachapi.FindingSourceDeterministic,
		Payload:     json.RawMessage(`{"rule_id":"state.hidden_input_mutation","path":"a.go"}`),
		PayloadHash: fmt.Sprintf("hash-%d", lease.Attempt),
		CreatedAt:   time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	if err := w.InsertFindings(context.Background(), []coachapi.JobFinding{finding}); err != nil {
		return nil, err
	}
	return &coachapi.Completion{
		Attempt:     lease.Attempt,
		CommitSHA:   "abc123def4567890abc123def4567890abc123de",
		Versions:    coachapi.ReportVersions{Analyzer: "codesignal@1"},
		FinishedAt:  time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
		GeneratedAt: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
	}, nil
}

func (s *heartbeatMuteStore) Heartbeat(ctx context.Context, jobID, workerID string, attempt int, now time.Time) error {
	if s.mute.Load() {
		return nil
	}
	return s.MemoryStore.Heartbeat(ctx, jobID, workerID, attempt, now)
}

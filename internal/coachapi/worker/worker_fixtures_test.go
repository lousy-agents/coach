package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/worker"
)

func newQueuedJob(id string) coachapi.Job {
	return coachapi.Job{
		ID:                id,
		Kind:              coachapi.JobKindRepoBaselineScan,
		Params:            json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets"}`),
		Status:            coachapi.JobStatusQueued,
		CreatedAt:         time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
		Attempt:           0,
		CreatedByProvider: "github",
		CreatedBySubject:  "12345",
		CreatedByLogin:    "octocat",
	}
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
		// Yield so heartbeat/reconciler goroutines scheduled on After can run.
		time.Sleep(time.Millisecond)
	}
}

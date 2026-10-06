package httpapi_test

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/store/memory"
)

// spyJobStore wraps a memory.Store and counts CreateJob invocations so tests
// can prove a rejected submit persisted nothing.
type spyJobStore struct {
	*memory.Store
	mu          sync.Mutex
	createCalls int
}

func newSpyJobStore() *spyJobStore {
	return &spyJobStore{Store: memory.NewStore()}
}

func (s *spyJobStore) CreateJob(ctx context.Context, job coachapi.Job) error {
	s.mu.Lock()
	s.createCalls++
	s.mu.Unlock()
	return s.Store.CreateJob(ctx, job)
}

func (s *spyJobStore) createJobCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createCalls
}

// errJobStore is a JobStore double that injects store failures (not clean
// misses) so HTTP acceptance can lock fail-closed 503 mapping.
type errJobStore struct {
	*memory.Store
	createErr error
	getErr    error
	reportErr error
}

func (s *errJobStore) CreateJob(ctx context.Context, job coachapi.Job) error {
	if s.createErr != nil {
		return s.createErr
	}
	if s.Store == nil {
		return errors.New("errJobStore: no backing memory.Store for CreateJob")
	}
	return s.Store.CreateJob(ctx, job)
}

func (s *errJobStore) GetJob(ctx context.Context, id string) (coachapi.Job, error) {
	if s.getErr != nil {
		return coachapi.Job{}, s.getErr
	}
	if s.Store == nil {
		return coachapi.Job{}, errors.New("errJobStore: no backing memory.Store for GetJob")
	}
	return s.Store.GetJob(ctx, id)
}

func (s *errJobStore) GetReport(ctx context.Context, id string) (coachapi.Report, error) {
	if s.reportErr != nil {
		return coachapi.Report{}, s.reportErr
	}
	if s.Store == nil {
		return coachapi.Report{}, errors.New("errJobStore: no backing memory.Store for GetReport")
	}
	return s.Store.GetReport(ctx, id)
}

func (s *errJobStore) RecordCompletion(ctx context.Context, jobID string, completion coachapi.Completion) error {
	if s.Store == nil {
		return errors.New("errJobStore: no backing memory.Store for RecordCompletion")
	}
	return s.Store.RecordCompletion(ctx, jobID, completion)
}

func (s *errJobStore) RecordFailure(ctx context.Context, jobID string, errMsg string, finishedAt time.Time) error {
	if s.Store == nil {
		return errors.New("errJobStore: no backing memory.Store for RecordFailure")
	}
	return s.Store.RecordFailure(ctx, jobID, errMsg, finishedAt)
}

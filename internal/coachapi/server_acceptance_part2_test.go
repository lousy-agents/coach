package coachapi_test

import (
	"context"

	"errors"

	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
)

func (s *errJobStore) GetJob(ctx context.Context, id string) (coachapi.Job, error) {
	if s.getErr != nil {
		return coachapi.Job{}, s.getErr
	}
	if s.MemoryStore == nil {
		return coachapi.Job{}, errors.New("errJobStore: no backing MemoryStore for GetJob")
	}
	return s.MemoryStore.GetJob(ctx, id)
}

func (s *errJobStore) GetReport(ctx context.Context, id string) (coachapi.Report, error) {
	if s.reportErr != nil {
		return coachapi.Report{}, s.reportErr
	}
	if s.MemoryStore == nil {
		return coachapi.Report{}, errors.New("errJobStore: no backing MemoryStore for GetReport")
	}
	return s.MemoryStore.GetReport(ctx, id)
}

func (s *errJobStore) RecordCompletion(ctx context.Context, jobID string, completion coachapi.Completion) error {
	if s.MemoryStore == nil {
		return errors.New("errJobStore: no backing MemoryStore for RecordCompletion")
	}
	return s.MemoryStore.RecordCompletion(ctx, jobID, completion)
}

func (s *errJobStore) RecordFailure(ctx context.Context, jobID string, errMsg string, finishedAt time.Time) error {
	if s.MemoryStore == nil {
		return errors.New("errJobStore: no backing MemoryStore for RecordFailure")
	}
	return s.MemoryStore.RecordFailure(ctx, jobID, errMsg, finishedAt)
}

func newSpyJobStore() *spyJobStore {
	return &spyJobStore{MemoryStore: coachapi.NewMemoryStore()}
}

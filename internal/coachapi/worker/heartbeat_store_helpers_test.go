package worker_test

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/store/memory"
)

// heartbeatMuteStore wraps memory.Store so tests can stop refreshing
// heartbeat_at while a handler is paused. FakeClock.Advance fires pending
// After waiters; the heartbeat goroutine would otherwise beat at the advanced
// Now() and un-stale the lease before ClaimJob reclaim (CI flake).
type heartbeatMuteStore struct {
	*memory.Store
	mute atomic.Bool
}

var _ coachapi.WorkerJobStore = (*heartbeatMuteStore)(nil)

func (s *heartbeatMuteStore) Heartbeat(ctx context.Context, jobID, workerID string, attempt int, now time.Time) error {
	if s.mute.Load() {
		return nil
	}
	return s.Store.Heartbeat(ctx, jobID, workerID, attempt, now)
}

// canceledHeartbeatStore returns context.Canceled from Heartbeat while the
// caller's ctx is still live — models a misclassified shutdown/timeout that
// must not be treated as claim-fence loss.
type canceledHeartbeatStore struct {
	*memory.Store
	entered chan struct{}
	once    sync.Once
}

var _ coachapi.WorkerJobStore = (*canceledHeartbeatStore)(nil)

func (s *canceledHeartbeatStore) Heartbeat(ctx context.Context, jobID, workerID string, attempt int, now time.Time) error {
	s.once.Do(func() { close(s.entered) })
	return context.Canceled
}

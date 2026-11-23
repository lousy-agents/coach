package baseline_test

import (
	"context"
	"sync"
	"time"

	"github.com/lousy-agents/coach/internal/modelgateway"
)

// steppedClock is a judgment clock that moves only when a test advances it.
type steppedClock struct {
	mu  sync.Mutex
	now time.Time
}

func newSteppedClock() *steppedClock {
	return &steppedClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (c *steppedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *steppedClock) advance(by time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(by)
}

// clockAdvancingGateway judges as inner does, and once the given number of
// judgments has completed, advances the clock by the given amount: the
// judgment budget then runs out between packs, wherever the wall clock is.
type clockAdvancingGateway struct {
	inner modelgateway.Gateway
	clock *steppedClock
	after int
	by    time.Duration

	mu     sync.Mutex
	judged int
}

func (g *clockAdvancingGateway) Judge(ctx context.Context, req modelgateway.JudgmentRequest) (modelgateway.JudgmentResponse, error) {
	resp, err := g.inner.Judge(ctx, req)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.judged++
	if g.judged == g.after {
		g.clock.advance(g.by)
	}
	return resp, err
}

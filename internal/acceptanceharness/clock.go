package acceptanceharness

import (
	"sort"
	"sync"
	"time"
)

// Clock abstracts the subset of the stdlib time package that consumers
// doing heartbeats, timeouts, and reconciliation actually need: reading the
// current time and waiting for a duration to elapse. Production code
// depends on this interface (satisfied by RealClock) so that acceptance
// tests can substitute FakeClock and drive time forward with explicit,
// broker-visible Advance calls instead of time.Sleep polling.
type Clock interface {
	// Now returns the clock's current time.
	Now() time.Time
	// After returns a channel that fires exactly once, with the deadline
	// time (Now() at the moment After was called, plus d), once that
	// deadline has been reached.
	After(d time.Duration) <-chan time.Time
}

// RealClock is a zero-value-usable Clock backed by the real time.Now and
// time.After. Use it in production code; use FakeClock in tests.
type RealClock struct{}

// Now returns time.Now().
func (RealClock) Now() time.Time { return time.Now() }

// After returns time.After(d).
func (RealClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// waiter is a single pending After() call: it fires ch, exactly once, once
// the FakeClock's Now() reaches or passes deadline.
type waiter struct {
	deadline time.Time
	ch       chan time.Time
}

// FakeClock is a test-only Clock whose Now() only changes when a test calls
// Advance: it never drifts with wall-clock time. It is safe for concurrent
// use, so a goroutine calling After (e.g. a heartbeat ticker under test) may
// race with the test goroutine calling Advance.
type FakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []*waiter
}

// NewFakeClock returns a FakeClock whose Now() starts at the given, caller-
// provided time.
func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{now: start}
}

// Now returns the fake clock's current time, as last set by NewFakeClock or
// advanced by Advance. It never reflects real wall-clock time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// After returns a channel that fires with the deadline time (Now()+d, as of
// this call) once a subsequent Advance moves Now() to or past that
// deadline. It never fires early, regardless of how much real wall-clock
// time passes.
func (c *FakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	deadline := c.now.Add(d)
	ch := make(chan time.Time, 1)

	if !deadline.After(c.now) {
		// Deadline already reached (zero or negative duration): fire
		// immediately, matching time.After's behavior for d <= 0.
		ch <- deadline
		return ch
	}

	c.waiters = append(c.waiters, &waiter{deadline: deadline, ch: ch})
	return ch
}

// Advance moves the fake clock's Now() forward by d and fires, in deadline
// order, every pending After channel whose deadline is now at or before the
// new Now().
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(d)

	sort.Slice(c.waiters, func(i, j int) bool {
		return c.waiters[i].deadline.Before(c.waiters[j].deadline)
	})

	remaining := c.waiters[:0]
	for _, w := range c.waiters {
		if !w.deadline.After(c.now) {
			w.ch <- w.deadline
			continue
		}
		remaining = append(remaining, w)
	}
	c.waiters = remaining
}

// Package clock is the time source production code depends on, so a test can
// substitute a controlled one (acceptanceharness.FakeClock) and drive time
// forward with explicit calls instead of sleeping.
package clock

import "time"

// Clock abstracts the subset of the stdlib time package that consumers doing
// heartbeats, timeouts, and reconciliation actually need: reading the current
// time and waiting for a duration to elapse. Production code depends on this
// interface (satisfied by RealClock) so that acceptance tests can substitute a
// FakeClock and drive time forward with explicit, broker-visible Advance calls
// instead of time.Sleep polling.
type Clock interface {
	// Now returns the clock's current time.
	Now() time.Time
	// After returns a channel that fires exactly once, with the deadline
	// time (Now() at the moment After was called, plus d), once that
	// deadline has been reached.
	After(d time.Duration) <-chan time.Time
}

// RealClock is a zero-value-usable Clock backed by the real time.Now and
// time.After. Use it in production code; use acceptanceharness.FakeClock in
// tests.
type RealClock struct{}

// Now returns time.Now().
func (RealClock) Now() time.Time { return time.Now() }

// After returns time.After(d).
func (RealClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

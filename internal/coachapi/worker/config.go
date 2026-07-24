package worker

import (
	"errors"
	"fmt"
	"time"
)

const (
	defaultHeartbeatInterval  = 15 * time.Second
	defaultStaleAfter         = 60 * time.Second
	defaultReconcileInterval  = 30 * time.Second
	defaultQueuedAgeThreshold = 30 * time.Second
	defaultMaxAttempts        = 5
)

// Config holds injectable worker timings and identity. Zero durations take
// the package defaults (15s heartbeat, 60s stale, 30s reconcile/queued age,
// 5 max attempts). StaleAfter must be at least 3× HeartbeatInterval after
// defaults are applied. MaxAttempts must be >= 1 after defaults.
type Config struct {
	WorkerID           string
	HeartbeatInterval  time.Duration
	StaleAfter         time.Duration
	ReconcileInterval  time.Duration
	QueuedAgeThreshold time.Duration
	// MaxAttempts is the maximum jobs.attempt value that may run a handler
	// before a retryable failure is treated as exhausted (terminal + poison).
	// Permanent handler errors are terminal on any attempt.
	MaxAttempts int
}

func (c Config) withDefaults() (Config, error) {
	if c.WorkerID == "" {
		return Config{}, errors.New("worker: Config.WorkerID is required")
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = defaultHeartbeatInterval
	}
	if c.StaleAfter <= 0 {
		c.StaleAfter = defaultStaleAfter
	}
	if c.ReconcileInterval <= 0 {
		c.ReconcileInterval = defaultReconcileInterval
	}
	if c.QueuedAgeThreshold <= 0 {
		c.QueuedAgeThreshold = defaultQueuedAgeThreshold
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = defaultMaxAttempts
	}
	if c.StaleAfter < 3*c.HeartbeatInterval {
		return Config{}, fmt.Errorf(
			"worker: StaleAfter (%s) must be >= 3× HeartbeatInterval (%s)",
			c.StaleAfter, c.HeartbeatInterval,
		)
	}
	return c, nil
}

// Retryable marks err as a transient handler failure eligible for bounded
// queue redelivery (Nack permanent=false) while lease.Attempt < MaxAttempts.
// Plain errors and errors that do not unwrap to a Retryable marker are
// permanent (FailJob + Nack permanent=true).
func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return retryableError{err: err}
}

// IsRetryable reports whether err (or any error in its unwrap chain) was
// produced by Retryable.
func IsRetryable(err error) bool {
	var target retryableError
	return errors.As(err, &target)
}

type retryableError struct {
	err error
}

func (e retryableError) Error() string { return e.err.Error() }
func (e retryableError) Unwrap() error { return e.err }

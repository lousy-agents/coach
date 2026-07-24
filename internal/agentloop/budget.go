package agentloop

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Budget limits one Loop run. Zero fields are filled with v1 defaults in New.
type Budget struct {
	MaxToolCalls  int
	MaxModelCalls int
	MaxWallTime   time.Duration
}

const (
	// DefaultMaxToolCalls is the v1 default tool-call budget.
	DefaultMaxToolCalls = 50
	// DefaultMaxModelCalls is the v1 default model-turn budget.
	DefaultMaxModelCalls = 20
	// DefaultMaxWallTime is the v1 default wall-clock budget.
	DefaultMaxWallTime = 5 * time.Minute
)

// Clock supplies Now for wall-time budget checks. Tests inject a fake clock.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func applyBudgetDefaults(b Budget) Budget {
	if b.MaxToolCalls <= 0 {
		b.MaxToolCalls = DefaultMaxToolCalls
	}
	if b.MaxModelCalls <= 0 {
		b.MaxModelCalls = DefaultMaxModelCalls
	}
	if b.MaxWallTime <= 0 {
		b.MaxWallTime = DefaultMaxWallTime
	}
	return b
}

func (l *Loop) checkWallLocked() error {
	if l.clock.Now().Sub(l.start) >= l.budget.MaxWallTime {
		return fmt.Errorf("%w: max_wall_time %s", ErrBudgetExceeded, l.budget.MaxWallTime)
	}
	return nil
}

// wallBudgetContext returns a child context cancelled when the remaining wall
// budget elapses (real-time timeout derived from clock-measured remaining).
func (l *Loop) wallBudgetContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if err := ctx.Err(); err != nil {
		return nil, func() {}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.checkWallLocked(); err != nil {
		return nil, func() {}, err
	}
	remaining := l.budget.MaxWallTime - l.clock.Now().Sub(l.start)
	if remaining <= 0 {
		return nil, func() {}, fmt.Errorf("%w: max_wall_time %s", ErrBudgetExceeded, l.budget.MaxWallTime)
	}
	opCtx, cancel := context.WithTimeout(ctx, remaining)
	return opCtx, cancel, nil
}

// mapWallErr rewrites deadline/cancel from a wall-budget child context into
// ErrBudgetExceeded when the parent context is still live.
func (l *Loop) mapWallErr(parent, opCtx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if parent.Err() != nil {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(opCtx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%w: max_wall_time %s", ErrBudgetExceeded, l.budget.MaxWallTime)
	}
	return err
}

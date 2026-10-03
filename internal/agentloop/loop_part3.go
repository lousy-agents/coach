package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
)

// runModelToolCalls executes calls in order and marshals their results into
// the next turn's prompt. Model text never becomes an action; only
// registered tool calls execute.
func (l *Loop) runModelToolCalls(ctx context.Context, calls []ToolCall) (string, error) {
	toolResults := make([]json.RawMessage, 0, len(calls))
	for _, tc := range calls {
		out, callErr := l.Call(ctx, CallSourceModel, tc.Name, tc.Args)
		if callErr != nil {
			return "", callErr
		}
		toolResults = append(toolResults, out)
	}

	payload, err := json.Marshal(toolResults)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

// reserveModelCall charges one model call against the budget. The wall-time
// check, the ceiling check, and the increment share one lock hold so two
// turns cannot both pass the ceiling before either increments.
func (l *Loop) reserveModelCall() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.checkWallLocked(); err != nil {
		return err
	}
	if l.modelCalls >= l.budget.MaxModelCalls {
		return fmt.Errorf("%w: max_model_calls %d", ErrBudgetExceeded, l.budget.MaxModelCalls)
	}
	l.modelCalls++
	return nil
}

// Calls returns a defensive copy of every registry invocation so far, in order.
func (l *Loop) Calls() []RecordedCall {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]RecordedCall, len(l.calls))
	copy(out, l.calls)
	for i := range out {
		out[i].Args = cloneRawMessage(out[i].Args)
		out[i].Result = cloneRawMessage(out[i].Result)
	}
	return out
}

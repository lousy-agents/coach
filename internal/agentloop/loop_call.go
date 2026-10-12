package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
)

// RecordedCall is one registry invocation observed by Calls().
type RecordedCall struct {
	Name   string
	Source CallSource
	Args   json.RawMessage
	Result json.RawMessage
	Err    error
}

// Call invokes a registered tool once under the given source and budgets.
// Unknown tools, schema-invalid args, and budget exhaustion are typed errors.
func (l *Loop) Call(ctx context.Context, source CallSource, name string, args json.RawMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	l.mu.Lock()
	if err := l.checkWallLocked(); err != nil {
		l.mu.Unlock()
		return nil, err
	}
	if l.toolCalls >= l.budget.MaxToolCalls {
		max := l.budget.MaxToolCalls
		l.mu.Unlock()
		return nil, fmt.Errorf("%w: max_tool_calls %d", ErrBudgetExceeded, max)
	}
	tool, ok := l.tools[name]
	if !ok {
		rec := RecordedCall{Name: name, Source: source, Args: cloneRawMessage(args), Err: ErrUnknownTool}
		l.calls = append(l.calls, rec)
		l.mu.Unlock()
		return nil, ErrUnknownTool
	}
	schema := tool.schema
	handler := tool.handler
	// Reserve the tool-call slot before releasing the lock so concurrent Call
	// cannot overshoot MaxToolCalls.
	l.toolCalls++
	l.mu.Unlock()

	if err := validateToolArgs(schema, args); err != nil {
		l.record(name, source, args, nil, err)
		return nil, err
	}

	opCtx, cancel, err := l.wallBudgetContext(ctx)
	if err != nil {
		l.record(name, source, args, nil, err)
		return nil, err
	}
	defer cancel()

	result, err := handler(opCtx, args)
	err = l.mapWallErr(ctx, opCtx, err)
	l.record(name, source, args, result, err)
	return result, err
}

func (l *Loop) record(name string, source CallSource, args, result json.RawMessage, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, RecordedCall{
		Name:   name,
		Source: source,
		Args:   cloneRawMessage(args),
		Result: cloneRawMessage(result),
		Err:    err,
	})
}

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

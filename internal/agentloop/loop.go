// Package agentloop is a bounded tool-call broker over a typed registry
// (ADR-005). Handlers drive guaranteed tools; models may only select from
// registered, schema-validated tools. Model text never becomes an arbitrary
// action.
//
// Layout for extension:
//   - loop.go — Call/Run orchestration and recording
//   - budget.go — tool/model/wall budgets
//   - tools.go — registry types + core-tool table (add always-on tools there)
//   - core_tools.go — default handlers/schemas for core tools
//   - schema.go — args-schema validation
//
// Job-specific tools (rubrics, PR-history GitHub tools) use Register at loop start.
package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CallSource identifies who initiated a tool call (ADR-005 layers 1 and 2).
type CallSource string

const (
	// CallSourceHandler marks a handler-driven (guaranteed/deterministic) call.
	CallSourceHandler CallSource = "handler"
	// CallSourceModel marks a model-selected call from the fixed allowlist.
	CallSourceModel CallSource = "model"
)

// ToolCall is one model- or handler-requested invocation.
type ToolCall struct {
	Name string
	Args json.RawMessage
}

// TurnResponse is one model generation result used by the loop's multi-turn seam.
type TurnResponse struct {
	Text      string
	ToolCalls []ToolCall
}

// TurnGateway is the multi-turn model seam for tool-call sequences. Distinct from
// modelgateway.Gateway (Judge-only); production wiring and tests inject adapters
// without pulling LLM HTTP clients into this package.
type TurnGateway interface {
	Generate(ctx context.Context, prompt string) (TurnResponse, error)
}

// RunResult is the outcome of a multi-turn Run that ends on a text-only model turn.
type RunResult struct {
	FinalText string
}

// RecordedCall is one registry invocation observed by Calls().
type RecordedCall struct {
	Name   string
	Source CallSource
	Args   json.RawMessage
	Result json.RawMessage
	Err    error
}

// Options configures New. Core tool handlers may be injected; nil uses package defaults.
// When adding a new always-on core tool, extend Options (if injectable) and coreToolDefs.
type Options struct {
	Budget           Budget
	Clock            Clock
	SemanticsAnalyze ToolHandler
	CodeSignalReport ToolHandler
}

// Loop is a bounded tool-call broker over a typed registry (ADR-005).
type Loop struct {
	mu sync.Mutex

	tools  map[string]registeredTool
	budget Budget
	clock  Clock
	start  time.Time

	toolCalls  int
	modelCalls int
	calls      []RecordedCall
}

// New constructs a Loop with core tools always registered and the given budget defaults applied.
func New(opts Options) (*Loop, error) {
	clock := opts.Clock
	if clock == nil {
		clock = realClock{}
	}

	return &Loop{
		tools:  newCoreToolRegistry(opts),
		budget: applyBudgetDefaults(opts.Budget),
		clock:  clock,
		start:  clock.Now(),
	}, nil
}

// Budget returns the effective budget for this loop (including defaults).
func (l *Loop) Budget() Budget {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.budget
}

// Register adds a job-specific tool for this run. Core tool names cannot be
// replaced here — inject handlers via Options instead.
func (l *Loop) Register(spec ToolSpec) error {
	if spec.Name == "" {
		return fmt.Errorf("agentloop: tool name is required")
	}
	if spec.Handler == nil {
		return fmt.Errorf("agentloop: tool handler is required")
	}
	if err := rejectCoreToolRegister(spec.Name); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tools[spec.Name] = registeredTool{
		schema:  cloneRawMessage(spec.ArgsSchema),
		handler: spec.Handler,
	}
	return nil
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

// Run drives multi-turn model tool-call sequences until a text-only response
// or a typed error (unknown tool, invalid args, budget). Model text is never
// executed as an action — only registered tool calls are.
func (l *Loop) Run(ctx context.Context, gw TurnGateway, prompt string) (RunResult, error) {
	if gw == nil {
		return RunResult{}, fmt.Errorf("agentloop: TurnGateway is required")
	}

	nextPrompt := prompt
	for {
		resp, err := l.nextTurn(ctx, gw, nextPrompt)
		if err != nil {
			return RunResult{}, err
		}
		if len(resp.ToolCalls) == 0 {
			return RunResult{FinalText: resp.Text}, nil
		}
		nextPrompt, err = l.runModelToolCalls(ctx, resp.ToolCalls)
		if err != nil {
			return RunResult{}, err
		}
	}
}

// nextTurn reserves a model-call slot, generates one turn, and re-checks
// wall time so a turn that overran (e.g. via injected clock) cannot succeed
// as text-only or start tools.
func (l *Loop) nextTurn(ctx context.Context, gw TurnGateway, prompt string) (TurnResponse, error) {
	if err := ctx.Err(); err != nil {
		return TurnResponse{}, err
	}
	if err := l.reserveModelCall(); err != nil {
		return TurnResponse{}, err
	}
	opCtx, cancel, err := l.wallBudgetContext(ctx)
	if err != nil {
		return TurnResponse{}, err
	}
	resp, err := gw.Generate(opCtx, prompt)
	err = l.mapWallErr(ctx, opCtx, err)
	cancel()
	if err != nil {
		return TurnResponse{}, err
	}
	if err := l.checkWall(); err != nil {
		return TurnResponse{}, err
	}
	return resp, nil
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

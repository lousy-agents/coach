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

func cloneRawMessage(m json.RawMessage) json.RawMessage {
	if m == nil {
		return nil
	}
	out := make(json.RawMessage, len(m))
	copy(out, m)
	return out
}

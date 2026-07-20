package agentloopharness

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

// ErrUnknownTool is returned by RecordingToolRegistry.Call when name has no
// registered handler, per ADR-005's "unknown tools ... are typed errors"
// principle. The call is still recorded (with this error as its outcome)
// rather than silently dropped, mirroring this package's fixture.go
// precedent that misuse must be "a recorded test failure ... never
// silently omitted from the record".
var ErrUnknownTool = errors.New("agentloopharness: unknown tool")

// CallSource identifies who initiated a recorded tool call: the job
// handler driving guaranteed/deterministic tools, or the model choosing a
// tool from its fixed allowlist during rubric judgment. This is the
// vocabulary ADR-005's validation section needs to prove "the deterministic
// path did not bypass the registry".
type CallSource string

const (
	// CallSourceHandler marks a call the job handler drove directly,
	// independent of any model-selected activity (ADR-005 layer 1).
	CallSourceHandler CallSource = "handler"
	// CallSourceModel marks a call the model chose to issue from its fixed
	// allowlist during rubric judgment (ADR-005 layer 2).
	CallSourceModel CallSource = "model"
)

// ToolHandler is the function shape a tool registers under a name: it
// receives the call's raw JSON arguments and returns a raw JSON result or
// an error.
type ToolHandler func(ctx context.Context, args json.RawMessage) (json.RawMessage, error)

// RecordedCall is one entry in a RecordingToolRegistry's append-only call
// log: the tool name, which source invoked it, the raw arguments passed,
// and the call's outcome (Result on success, Err on failure -- including
// ErrUnknownTool for an unregistered name).
type RecordedCall struct {
	Name   string
	Source CallSource
	Args   json.RawMessage
	Result json.RawMessage
	Err    error
}

// RecordingToolRegistry is a tool-call broker that invokes named,
// registered tool handlers and records every call -- successful, failed,
// or against an unknown name -- regardless of outcome, so a test can assert
// the exact call sequence and distinguish handler-driven calls from
// model-selected ones. Safe for concurrent use. The zero value is ready to
// use.
type RecordingToolRegistry struct {
	mu       sync.Mutex
	handlers map[string]ToolHandler
	calls    []RecordedCall
}

// Register adds a named tool handler. A later Register call with the same
// name replaces the earlier one.
func (r *RecordingToolRegistry) Register(name string, handler ToolHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.handlers == nil {
		r.handlers = make(map[string]ToolHandler)
	}
	r.handlers[name] = handler
}

// Call invokes the tool registered under name and records the call
// regardless of success or failure. Calling an unregistered name returns
// ErrUnknownTool -- and the call is still appended to Calls() with that
// error as its outcome, never silently dropped.
func (r *RecordingToolRegistry) Call(ctx context.Context, source CallSource, name string, args json.RawMessage) (json.RawMessage, error) {
	r.mu.Lock()
	handler, ok := r.handlers[name]
	r.mu.Unlock()

	if !ok {
		rec := RecordedCall{Name: name, Source: source, Args: args, Err: ErrUnknownTool}
		r.mu.Lock()
		r.calls = append(r.calls, rec)
		r.mu.Unlock()
		return nil, ErrUnknownTool
	}

	result, err := handler(ctx, args)

	rec := RecordedCall{Name: name, Source: source, Args: args, Result: result, Err: err}
	r.mu.Lock()
	r.calls = append(r.calls, rec)
	r.mu.Unlock()

	return result, err
}

// Calls returns a defensive copy of every RecordedCall recorded so far, in
// call order, mirroring this package's fixture.go Recorder.Records()
// pattern. Mutating the returned slice never affects the registry's
// internal state.
func (r *RecordingToolRegistry) Calls() []RecordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]RecordedCall, len(r.calls))
	copy(out, r.calls)
	return out
}

// Package agentloopharness defines test/harness seams that a later epic's
// internal/agentloop package will import from its own tests: a scripted
// model-gateway stand-in (ScriptedGateway) and a recording tool-call broker
// (RecordingToolRegistry). See docs/architecture/ADR-005-agent-loop-
// orchestration-split.md for the eventual internal/agentloop design this
// package's seams stand in for. This package is scaffolding only: it does
// not wire into any existing production code path.
package agentloopharness

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

// ErrScriptExhausted is returned by ScriptedGateway.Generate once every
// scripted Response has been consumed, rather than a test silently getting
// a zero-value Response back or the gateway panicking -- a test asserting
// "the deterministic path did not need more model calls than expected"
// needs this to fail loudly.
var ErrScriptExhausted = errors.New("agentloopharness: scripted gateway responses exhausted")

// ToolCall is the minimal shape of a tool call a scripted model Response can
// carry: a tool name plus its raw JSON arguments, mirroring the tool-call
// shape RecordingToolRegistry.Call expects.
type ToolCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// Response is a minimal stand-in for the future model-gateway contract's
// generation result: freeform text plus zero or more tool calls the model
// chose to issue.
type Response struct {
	Text      string     `json:"text"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ScriptedGateway is a minimal stand-in for the future model-gateway
// contract, useful for tests that need deterministic model output: it
// returns pre-scripted Responses in order rather than calling a real model.
type ScriptedGateway struct {
	mu        sync.Mutex
	responses []Response
	next      int
}

// NewScriptedGateway builds a ScriptedGateway that returns responses, in
// order, one per Generate call.
func NewScriptedGateway(responses ...Response) *ScriptedGateway {
	return &ScriptedGateway{responses: responses}
}

// Generate returns the next scripted Response in order. Once every scripted
// Response has been returned, it returns ErrScriptExhausted rather than a
// zero-value Response or a panic.
func (g *ScriptedGateway) Generate(ctx context.Context, prompt string) (Response, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.next >= len(g.responses) {
		return Response{}, ErrScriptExhausted
	}

	resp := g.responses[g.next]
	g.next++
	return resp, nil
}

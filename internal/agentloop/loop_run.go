package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
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

package rubrics

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/modelgateway"
)

// RegisterTools registers the seed rubric judgment tools on loop as job-specific
// tools (ADR-005). Tools call modelgateway.Gateway.Judge; schema/unavailable
// judgment failures degrade to a diagnostic envelope instead of failing the
// tool call hard. context.Canceled is returned as a hard tool error.
func RegisterTools(loop *agentloop.Loop, gw modelgateway.Gateway) error {
	if loop == nil {
		return fmt.Errorf("rubrics: loop is required")
	}
	specs, err := ToolSpecs(gw)
	if err != nil {
		return err
	}
	for _, spec := range specs {
		if err := loop.Register(spec); err != nil {
			return err
		}
	}
	return nil
}

// ToolSpecs returns agentloop.ToolSpec values for the two seed rubrics.
func ToolSpecs(gw modelgateway.Gateway) ([]agentloop.ToolSpec, error) {
	if gw == nil {
		return nil, fmt.Errorf("rubrics: gateway is required")
	}
	out := make([]agentloop.ToolSpec, 0, 2)
	for _, cfg := range seedToolConfigs() {
		spec, err := seedJudgmentTool(gw, cfg)
		if err != nil {
			return nil, err
		}
		out = append(out, spec)
	}
	return out, nil
}

type seedToolConfig struct {
	id         string
	argsSchema json.RawMessage
	assemble   func(json.RawMessage) ([]modelgateway.Message, error)
}

func seedToolConfigs() []seedToolConfig {
	return []seedToolConfig{
		{
			id:         IDHiddenMutationContextualization,
			argsSchema: hiddenMutationArgsSchema(),
			assemble:   assembleHiddenMutationArgs,
		},
		{
			id:         IDChangeCohesion,
			argsSchema: changeCohesionArgsSchema(),
			assemble:   assembleChangeCohesionArgs,
		},
	}
}

func seedJudgmentTool(gw modelgateway.Gateway, cfg seedToolConfig) (agentloop.ToolSpec, error) {
	def, ok := DefinitionByID(cfg.id)
	if !ok {
		return agentloop.ToolSpec{}, fmt.Errorf("rubrics: missing seed definition %q", cfg.id)
	}
	assemble := cfg.assemble
	return agentloop.ToolSpec{
		Name:       cfg.id,
		ArgsSchema: cfg.argsSchema,
		Handler: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			msgs, err := assemble(args)
			if err != nil {
				return nil, err
			}
			result, err := Run(ctx, gw, def, msgs)
			if err != nil {
				return nil, err
			}
			return marshalToolResult(toolResultFromRun(def, result))
		},
	}, nil
}

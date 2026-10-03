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
//
// hidden_mutation_contextualization accepts legacy singular {finding,file} or
// pack {items:[{finding_ref,finding,file},...]} args. Multi-item packs return
// a ToolPackResult envelope ({"results":[ToolResult...]}) for coachapi handlers.
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
func seedJudgmentTool(gw modelgateway.Gateway, cfg seedToolConfig) (agentloop.ToolSpec, error) {
	def, ok := DefinitionByID(cfg.id)
	if !ok {
		return agentloop.ToolSpec{}, fmt.Errorf("rubrics: missing seed definition %q", cfg.id)
	}
	if cfg.packAware {
		return agentloop.ToolSpec{
			Name:       cfg.id,
			ArgsSchema: cfg.argsSchema,
			Handler:    hiddenMutationToolHandler(gw, def),
		}, nil
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
func packRefs(items []HiddenMutationPackItem) []string {
	refs := make([]string, len(items))
	for i, it := range items {
		refs[i] = it.FindingRef
	}
	return refs
}

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
// a ToolPackResult envelope ({"results":[ToolResult...]}) for Task 3 handlers.
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
	// packAware: when true, the tool parses dual-shape args and may return a pack envelope.
	packAware bool
	assemble  func(json.RawMessage) ([]modelgateway.Message, error)
}

func seedToolConfigs() []seedToolConfig {
	return []seedToolConfig{
		{
			id:         IDHiddenMutationContextualization,
			argsSchema: hiddenMutationArgsSchema(),
			packAware:  true,
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

func hiddenMutationToolHandler(gw modelgateway.Gateway, def Definition) agentloop.ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		in, err := parseHiddenMutationArgs(args)
		if err != nil {
			return nil, err
		}
		if len(in.Items) >= 2 {
			return runHiddenMutationPack(ctx, gw, def, in.Items)
		}
		// Singular path: legacy args, or a one-item pack (may use singular schema).
		var msgs []modelgateway.Message
		if len(in.Items) == 1 {
			msgs = AssembleHiddenMutationMessages(HiddenMutationEvidence{
				Finding: in.Items[0].Finding,
				File:    in.Items[0].File,
			})
		} else {
			msgs = AssembleHiddenMutationMessages(HiddenMutationEvidence{
				Finding: in.Finding,
				File:    in.File,
			})
		}
		result, err := Run(ctx, gw, def, msgs)
		if err != nil {
			return nil, err
		}
		tr := toolResultFromRun(def, result)
		if len(in.Items) == 1 {
			tr.FindingRef = in.Items[0].FindingRef
		}
		return marshalToolResult(tr)
	}
}

func runHiddenMutationPack(ctx context.Context, gw modelgateway.Gateway, def Definition, items []HiddenMutationPackItem) (json.RawMessage, error) {
	if err := lifecycleAbortErr(ctx.Err()); err != nil {
		return nil, err
	}
	if gw == nil {
		refs := packRefs(items)
		return marshalToolPackResult(packResultsForGatewayDegrade(def, refs, degrade(def.ID, "model gateway is nil")))
	}

	msgs := AssembleHiddenMutationPackMessages(HiddenMutationPackEvidence{Items: items})
	refs := packRefs(items)

	resp, err := gw.Judge(ctx, modelgateway.JudgmentRequest{
		RubricID:      def.ID,
		RubricVersion: def.Version,
		Messages:      msgs,
		OutputSchema:  HiddenMutationBatchOutputSchema(),
	})
	if abort := firstLifecycleAbort(err, ctx.Err()); abort != nil {
		return nil, abort
	}
	// Wall-budget deadline: surface to agentloop.mapWallErr (Story 2). Do not
	// soft-degrade wall expiry as pack-level gateway-unavailable diagnostics.
	if err != nil && isOpDeadlineExceeded(ctx) {
		return nil, err
	}
	if err != nil {
		return marshalToolPackResult(packResultsForGatewayDegrade(def, refs, degradeFromErr(def.ID, err)))
	}

	pack := mapBatchJudgmentToPackResult(def, refs, resp)
	return marshalToolPackResult(pack)
}

func packRefs(items []HiddenMutationPackItem) []string {
	refs := make([]string, len(items))
	for i, it := range items {
		refs[i] = it.FindingRef
	}
	return refs
}

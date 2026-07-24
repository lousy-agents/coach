package rubrics

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/modelgateway"
)

// RegisterTools registers the seed rubric judgment tools on loop as job-specific
// tools (ADR-005). Tools call modelgateway.Gateway.Judge; judgment failures
// degrade to a diagnostic envelope instead of failing the tool call hard.
func RegisterTools(loop *agentloop.Loop, gw modelgateway.Gateway) error {
	if loop == nil {
		return fmt.Errorf("rubrics: loop is required")
	}
	if gw == nil {
		return fmt.Errorf("rubrics: gateway is required")
	}
	for _, spec := range ToolSpecs(gw) {
		if err := loop.Register(spec); err != nil {
			return err
		}
	}
	return nil
}

// ToolSpecs returns agentloop.ToolSpec values for the two seed rubrics.
func ToolSpecs(gw modelgateway.Gateway) []agentloop.ToolSpec {
	return []agentloop.ToolSpec{
		hiddenMutationToolSpec(gw),
		changeCohesionToolSpec(gw),
	}
}

func hiddenMutationToolSpec(gw modelgateway.Gateway) agentloop.ToolSpec {
	def, _ := DefinitionByID(IDHiddenMutationContextualization)
	return agentloop.ToolSpec{
		Name:       IDHiddenMutationContextualization,
		ArgsSchema: hiddenMutationArgsSchema(),
		Handler: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			var in struct {
				Finding json.RawMessage `json:"finding"`
				File    FileContext     `json:"file"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return nil, fmt.Errorf("%w: %v", agentloop.ErrInvalidArgs, err)
			}
			// Copy finding bytes so callers retain an unmodified deterministic buffer.
			finding := append(json.RawMessage(nil), in.Finding...)
			msgs := AssembleHiddenMutationMessages(HiddenMutationEvidence{
				Finding: finding,
				File:    in.File,
			})
			result := Run(ctx, gw, def, msgs)
			return marshalToolResult(toolResultFromRun(def, result))
		},
	}
}

func changeCohesionToolSpec(gw modelgateway.Gateway) agentloop.ToolSpec {
	def, _ := DefinitionByID(IDChangeCohesion)
	return agentloop.ToolSpec{
		Name:       IDChangeCohesion,
		ArgsSchema: changeCohesionArgsSchema(),
		Handler: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			var in struct {
				Findings json.RawMessage `json:"findings"`
				Files    []FileMeta      `json:"files"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return nil, fmt.Errorf("%w: %v", agentloop.ErrInvalidArgs, err)
			}
			findings := append(json.RawMessage(nil), in.Findings...)
			msgs := AssembleChangeCohesionMessages(ChangeCohesionEvidence{
				Findings: findings,
				Files:    in.Files,
			})
			result := Run(ctx, gw, def, msgs)
			return marshalToolResult(toolResultFromRun(def, result))
		},
	}
}

func marshalToolResult(r ToolResult) (json.RawMessage, error) {
	// Ensure judgment serializes as JSON null (not omitted) when absent.
	type wire struct {
		RubricID       string          `json:"rubric_id"`
		RubricVersion  string          `json:"rubric_version"`
		ModelIdentity  *string         `json:"model_identity"`
		LogicalModelID *string         `json:"logical_model_id,omitempty"`
		ServedModelID  *string         `json:"served_model_id,omitempty"`
		Judgment       json.RawMessage `json:"judgment"`
		Diagnostic     *Diagnostic     `json:"diagnostic"`
	}
	w := wire{
		RubricID:       r.RubricID,
		RubricVersion:  r.RubricVersion,
		ModelIdentity:  r.ModelIdentity,
		LogicalModelID: r.LogicalModelID,
		ServedModelID:  r.ServedModelID,
		Diagnostic:     r.Diagnostic,
	}
	if len(r.Judgment) == 0 {
		w.Judgment = json.RawMessage("null")
	} else {
		w.Judgment = r.Judgment
	}
	return json.Marshal(w)
}

func hiddenMutationArgsSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"required":["finding","file"],
		"properties":{
			"finding":{"type":"object"},
			"file":{
				"type":"object",
				"required":["path"],
				"properties":{
					"path":{"type":"string"},
					"language":{"type":"string"},
					"content":{"type":"string"}
				}
			}
		}
	}`)
}

func changeCohesionArgsSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"required":["findings","files"],
		"properties":{
			"findings":{"type":"array"},
			"files":{
				"type":"array",
				"items":{
					"type":"object",
					"required":["path"],
					"properties":{
						"path":{"type":"string"},
						"language":{"type":"string"}
					}
				}
			}
		}
	}`)
}

package rubrics

import (
	"encoding/json"
	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/modelgateway"
)

// hiddenMutationArgs is the dual-shape input for hidden_mutation_contextualization:
// legacy singular {finding,file} or pack {items:[{finding_ref,finding,file},...]}.
type hiddenMutationArgs struct {
	Finding json.RawMessage          `json:"finding"`
	File    FileContext              `json:"file"`
	Items   []HiddenMutationPackItem `json:"items"`
}

func parseHiddenMutationArgs(args json.RawMessage) (hiddenMutationArgs, error) {
	var in hiddenMutationArgs
	if err := json.Unmarshal(args, &in); err != nil {
		return hiddenMutationArgs{}, fmt.Errorf("%w: %v", agentloop.ErrInvalidArgs, err)
	}
	if len(in.Items) > 0 {
		items, err := parseHiddenMutationItems(in.Items)
		if err != nil {
			return hiddenMutationArgs{}, err
		}
		return hiddenMutationArgs{Items: items}, nil
	}
	if len(in.Finding) == 0 {
		return hiddenMutationArgs{}, fmt.Errorf("%w: finding or items is required", agentloop.ErrInvalidArgs)
	}
	return hiddenMutationArgs{
		Finding: append(json.RawMessage(nil), in.Finding...),
		File:    in.File,
	}, nil
}

// parseHiddenMutationItems validates items (each must carry a finding_ref
// and a finding) and returns defensive copies so callers retain unmodified
// buffers.
func parseHiddenMutationItems(items []HiddenMutationPackItem) ([]HiddenMutationPackItem, error) {
	for i, it := range items {
		if it.FindingRef == "" {
			return nil, fmt.Errorf("%w: items[%d].finding_ref is required", agentloop.ErrInvalidArgs, i)
		}
		if len(it.Finding) == 0 {
			return nil, fmt.Errorf("%w: items[%d].finding is required", agentloop.ErrInvalidArgs, i)
		}
	}
	out := make([]HiddenMutationPackItem, len(items))
	for i, it := range items {
		out[i] = HiddenMutationPackItem{
			FindingRef: it.FindingRef,
			Finding:    append(json.RawMessage(nil), it.Finding...),
			File:       it.File,
		}
	}
	return out, nil
}

func assembleHiddenMutationArgs(args json.RawMessage) ([]modelgateway.Message, error) {
	in, err := parseHiddenMutationArgs(args)
	if err != nil {
		return nil, err
	}
	if len(in.Items) > 0 {
		return AssembleHiddenMutationPackMessages(HiddenMutationPackEvidence{Items: in.Items}), nil
	}
	return AssembleHiddenMutationMessages(HiddenMutationEvidence{
		Finding: in.Finding,
		File:    in.File,
	}), nil
}

func assembleChangeCohesionArgs(args json.RawMessage) ([]modelgateway.Message, error) {
	var in struct {
		Findings json.RawMessage `json:"findings"`
		Files    []FileMeta      `json:"files"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("%w: %v", agentloop.ErrInvalidArgs, err)
	}
	findings := append(json.RawMessage(nil), in.Findings...)
	return AssembleChangeCohesionMessages(ChangeCohesionEvidence{
		Findings: findings,
		Files:    in.Files,
	}), nil
}

func hiddenMutationArgsSchema() json.RawMessage {
	// Dual shape: singular {finding,file} or pack {items:[...]}.
	// No top-level required so both forms pass agentloop args validation;
	// the handler enforces finding-or-items via parseHiddenMutationArgs.
	return json.RawMessage(`{
		"type":"object",
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
			},
			"items":{
				"type":"array",
				"items":{
					"type":"object",
					"required":["finding_ref","finding","file"],
					"properties":{
						"finding_ref":{"type":"string"},
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

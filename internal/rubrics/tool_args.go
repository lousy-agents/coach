package rubrics

import (
	"encoding/json"
	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/modelgateway"
)

func assembleHiddenMutationArgs(args json.RawMessage) ([]modelgateway.Message, error) {
	var in struct {
		Finding json.RawMessage `json:"finding"`
		File    FileContext     `json:"file"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("%w: %v", agentloop.ErrInvalidArgs, err)
	}
	// Copy finding bytes so callers retain an unmodified deterministic buffer.
	finding := append(json.RawMessage(nil), in.Finding...)
	return AssembleHiddenMutationMessages(HiddenMutationEvidence{
		Finding: finding,
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

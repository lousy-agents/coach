package rubrics

import (
	"encoding/json"
)

func marshalToolPackResult(p ToolPackResult) (json.RawMessage, error) {
	type itemWire struct {
		FindingRef     string          `json:"finding_ref,omitempty"`
		RubricID       string          `json:"rubric_id"`
		RubricVersion  string          `json:"rubric_version"`
		ModelIdentity  *string         `json:"model_identity"`
		LogicalModelID *string         `json:"logical_model_id,omitempty"`
		ServedModelID  *string         `json:"served_model_id,omitempty"`
		Judgment       json.RawMessage `json:"judgment"`
		Diagnostic     *Diagnostic     `json:"diagnostic"`
	}
	wire := struct {
		Results []itemWire `json:"results"`
	}{
		Results: make([]itemWire, len(p.Results)),
	}
	for i, r := range p.Results {
		w := itemWire{
			FindingRef:     r.FindingRef,
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
		wire.Results[i] = w
	}
	return json.Marshal(wire)
}

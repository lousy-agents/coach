package rubrics

import (
	"encoding/json"
)

func toolResultFromRun(def Definition, r Result) ToolResult {
	out := ToolResult{
		RubricID:      def.ID,
		RubricVersion: def.Version,
	}
	if r.Diagnostic != nil {
		out.Diagnostic = r.Diagnostic
		return out
	}
	if r.Judgment == nil {
		out.Diagnostic = &Diagnostic{
			Scope:   diagnosticScope(def.ID),
			Message: "judgment failed: empty result",
		}
		return out
	}
	identity := r.Judgment.ModelIdentity
	logical := r.Judgment.LogicalModelID
	out.ModelIdentity = &identity
	out.LogicalModelID = &logical
	if r.Judgment.ServedModelID != "" {
		served := r.Judgment.ServedModelID
		out.ServedModelID = &served
	}
	out.Judgment = append(json.RawMessage(nil), r.Judgment.JudgmentJSON...)
	return out
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

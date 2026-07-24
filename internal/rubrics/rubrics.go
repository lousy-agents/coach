// Package rubrics holds versioned LLM-as-judge definitions, prompt assembly,
// and agent-loop tool adapters for seed rubrics (ADR-005).
package rubrics

import (
	"encoding/json"
)

// Seed rubric ids (ADR-005 tool names) and the v1 version string used in reports.
const (
	IDHiddenMutationContextualization = "hidden_mutation_contextualization"
	IDChangeCohesion                  = "change_cohesion"
	Version1                          = "1"
)

// Definition is one versioned rubric: id, version, and gateway output schema.
type Definition struct {
	ID           string
	Version      string
	OutputSchema json.RawMessage
}

// FileContext is baseline file context for hidden_mutation_contextualization.
type FileContext struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Content  string `json:"content,omitempty"`
}

// FileMeta is path/language metadata for change_cohesion evidence.
type FileMeta struct {
	Path     string `json:"path"`
	Language string `json:"language"`
}

// HiddenMutationEvidence is the deterministic input for hidden_mutation_contextualization.
type HiddenMutationEvidence struct {
	Finding json.RawMessage
	File    FileContext
}

// ChangeCohesionEvidence is the deterministic input for change_cohesion.
type ChangeCohesionEvidence struct {
	Findings json.RawMessage
	Files    []FileMeta
}

// Diagnostic is a judgment-phase failure recorded instead of a source=agent finding.
type Diagnostic struct {
	Scope   string `json:"scope"`
	Message string `json:"message"`
}

// Judgment is a successful schema-valid rubric judgment with provenance fields
// suitable for later source=agent findings (Task 8 persistence).
type Judgment struct {
	RubricID       string
	RubricVersion  string
	ModelIdentity  string
	LogicalModelID string
	ServedModelID  string
	JudgmentJSON   json.RawMessage
}

// Result is the outcome of Run: either a Judgment or a Diagnostic (never both).
// On gateway unavailable / schema validation failure, Judgment is nil and
// Diagnostic is set so the job can complete with deterministic-only findings.
type Result struct {
	Judgment   *Judgment
	Diagnostic *Diagnostic
}

// ToolResult is the JSON envelope returned by rubric agent-loop tools.
// On success Judgment is set and Diagnostic is null; on Story 5 degrade
// Judgment is null and Diagnostic is set. Schema/unavailable judgment
// failures do not hard-error the tool call so handlers can finish with
// deterministic evidence. context.Canceled is returned as a hard tool error
// (not a ToolResult diagnostic).
type ToolResult struct {
	RubricID       string          `json:"rubric_id"`
	RubricVersion  string          `json:"rubric_version"`
	ModelIdentity  *string         `json:"model_identity"`
	LogicalModelID *string         `json:"logical_model_id,omitempty"`
	ServedModelID  *string         `json:"served_model_id,omitempty"`
	Judgment       json.RawMessage `json:"judgment"`
	Diagnostic     *Diagnostic     `json:"diagnostic"`
}

// HasJudgment reports whether ToolResult carries a non-null judgment payload
// suitable for a source=agent finding.
func (r ToolResult) HasJudgment() bool {
	if len(r.Judgment) == 0 {
		return false
	}
	return string(r.Judgment) != "null"
}

// Seed returns the two v1 seed rubrics (Open-Questions decision for baseline).
func Seed() []Definition {
	return []Definition{
		{
			ID:           IDHiddenMutationContextualization,
			Version:      Version1,
			OutputSchema: mustSchema(schemaHiddenMutationV1),
		},
		{
			ID:           IDChangeCohesion,
			Version:      Version1,
			OutputSchema: mustSchema(schemaChangeCohesionV1),
		},
	}
}

// DefinitionByID returns the seed Definition for id, or false if unknown.
func DefinitionByID(id string) (Definition, bool) {
	for _, def := range Seed() {
		if def.ID == id {
			return def, true
		}
	}
	return Definition{}, false
}

func mustSchema(raw []byte) json.RawMessage {
	if !json.Valid(raw) {
		panic("rubrics: embedded schema is not valid JSON")
	}
	out := make(json.RawMessage, len(raw))
	copy(out, raw)
	return out
}

func diagnosticScope(rubricID string) string {
	return "rubric:" + rubricID
}

// FormatModelIdentity builds the single model_identity string for report provenance
// from gateway LogicalModelID and optional ServedModelID.
func FormatModelIdentity(logical, served string) string {
	if logical == "" {
		return served
	}
	if served == "" || served == logical {
		return logical
	}
	return logical + "/" + served
}

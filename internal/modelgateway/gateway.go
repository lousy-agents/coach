// Package modelgateway is the sole inference seam for structured judgment:
// request in → schema-validated response out. Callers (agent loop / job
// handlers) depend only on Gateway; backends are adapters behind this contract.
package modelgateway

import (
	"context"
	"encoding/json"
)

// DefaultLogicalModel is the stable logical model id application code should
// request. Concrete upstream mapping is deployment configuration only.
const DefaultLogicalModel = "coach-default"

// LogicalModelStub is the logical model id reported by StubGateway.
const LogicalModelStub = "stub"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type JudgmentRequest struct {
	RubricID      string          `json:"rubric_id"`
	RubricVersion string          `json:"rubric_version"`
	Messages      []Message       `json:"messages"`
	OutputSchema  json.RawMessage `json:"output_schema,omitempty"`
	// LogicalModel is the stable logical model id. Empty means DefaultLogicalModel.
	LogicalModel string `json:"logical_model,omitempty"`
}

// JudgmentResponse is a schema-validated judgment with model identity for
// provenance. LogicalModelID is always set; ServedModelID is set when the
// upstream reports an authoritative served model id.
type JudgmentResponse struct {
	JudgmentJSON   json.RawMessage `json:"judgment_json"`
	LogicalModelID string          `json:"logical_model_id"`
	ServedModelID  string          `json:"served_model_id,omitempty"`
}

type Gateway interface {
	// Judge runs a structured rubric judgment and returns a schema-validated
	// response. On failure, the error satisfies errors.Is(..., ErrSchemaValidation)
	// or errors.Is(..., ErrUnavailable) as appropriate — never a panic or bare
	// unclassified string-only error as the sole signal.
	Judge(ctx context.Context, req JudgmentRequest) (JudgmentResponse, error)
}

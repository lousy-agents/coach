package rubrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/modelgateway"
)

// Run executes one rubric judgment via the gateway. On schema validation
// failure or gateway unavailability it returns a Diagnostic and no Judgment
// so the job can complete with deterministic-only findings (Story 5).
func Run(ctx context.Context, gw modelgateway.Gateway, def Definition, messages []modelgateway.Message) Result {
	if gw == nil {
		return degrade(def.ID, "model gateway is nil")
	}
	if def.ID == "" {
		return degrade("unknown", "rubric definition id is required")
	}

	resp, err := gw.Judge(ctx, modelgateway.JudgmentRequest{
		RubricID:      def.ID,
		RubricVersion: def.Version,
		Messages:      messages,
		OutputSchema:  def.OutputSchema,
	})
	if err != nil {
		return degradeFromErr(def.ID, err)
	}

	identity := FormatModelIdentity(resp.LogicalModelID, resp.ServedModelID)
	return Result{
		Judgment: &Judgment{
			RubricID:       def.ID,
			RubricVersion:  def.Version,
			ModelIdentity:  identity,
			LogicalModelID: resp.LogicalModelID,
			ServedModelID:  resp.ServedModelID,
			JudgmentJSON:   append(json.RawMessage(nil), resp.JudgmentJSON...),
		},
	}
}

func degradeFromErr(rubricID string, err error) Result {
	switch {
	case errors.Is(err, modelgateway.ErrSchemaValidation):
		return degrade(rubricID, fmt.Sprintf("schema validation failed: %v", err))
	case errors.Is(err, modelgateway.ErrUnavailable):
		return degrade(rubricID, fmt.Sprintf("model gateway unavailable: %v", err))
	default:
		return degrade(rubricID, fmt.Sprintf("judgment failed: %v", err))
	}
}

func degrade(rubricID, message string) Result {
	return Result{
		Diagnostic: &Diagnostic{
			Scope:   diagnosticScope(rubricID),
			Message: message,
		},
	}
}

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

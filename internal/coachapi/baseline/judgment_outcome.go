package baseline

import (
	"encoding/json"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/rubrics"
)

// jobOutcomeFromRubricTool maps a rubric tool envelope to a JobFinding or
// JobDiagnostic. hashDiscriminators are mixed into PayloadHash after the
// judgment payload so multiple judgments that share an identical ToolResult
// (common with stub/live canned output) remain unique under the store UNIQUE
// constraint — pass the deterministic finding's PayloadHash for per-signal
// hidden_mutation_contextualization calls.
func jobOutcomeFromRubricTool(raw json.RawMessage, hashDiscriminators ...string) (*coachapi.JobFinding, *coachapi.JobDiagnostic, error) {
	var tr rubrics.ToolResult
	if err := json.Unmarshal(raw, &tr); err != nil {
		return nil, nil, fmt.Errorf("coachapi: decoding rubric tool result: %w", err)
	}
	if tr.Diagnostic != nil {
		return nil, &coachapi.JobDiagnostic{
			ID:      watermill.NewUUID(),
			Scope:   tr.Diagnostic.Scope,
			Message: tr.Diagnostic.Message,
		}, nil
	}
	if !tr.HasJudgment() {
		return nil, &coachapi.JobDiagnostic{
			ID:      watermill.NewUUID(),
			Scope:   "rubric:" + tr.RubricID,
			Message: "judgment failed: empty result",
		}, nil
	}
	payload, err := json.Marshal(tr)
	if err != nil {
		return nil, nil, err
	}
	rubricID := tr.RubricID
	rubricVersion := tr.RubricVersion
	var modelID *string
	if tr.ModelIdentity != nil {
		modelID = tr.ModelIdentity
	}
	hashParts := make([]string, 0, 3+len(hashDiscriminators))
	hashParts = append(hashParts, "agent", rubricID, string(payload))
	hashParts = append(hashParts, hashDiscriminators...)
	return &coachapi.JobFinding{
		ID:            watermill.NewUUID(),
		Source:        coachapi.FindingSourceAgent,
		RubricID:      &rubricID,
		RubricVersion: &rubricVersion,
		ModelIdentity: modelID,
		Payload:       payload,
		PayloadHash:   stablePayloadHash(hashParts...),
	}, nil, nil
}

// appendJobOutcome appends af/d onto findings/diags; either may be nil.
func appendJobOutcome(findings []coachapi.JobFinding, diags []coachapi.JobDiagnostic, af *coachapi.JobFinding, d *coachapi.JobDiagnostic) ([]coachapi.JobFinding, []coachapi.JobDiagnostic) {
	if af != nil {
		findings = append(findings, *af)
	}
	if d != nil {
		diags = append(diags, *d)
	}
	return findings, diags
}

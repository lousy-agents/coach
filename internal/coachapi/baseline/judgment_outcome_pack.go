package baseline

import (
	"encoding/json"
	"fmt"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/rubrics"
)

// jobOutcomesFromHiddenMutationResult maps a singular ToolResult or pack
// {"results":[...]} envelope to job findings/diagnostics. Pack items use
// FindingRef (deterministic PayloadHash) as the payload_hash discriminator.
func jobOutcomesFromHiddenMutationResult(raw json.RawMessage) ([]coachapi.JobFinding, []coachapi.JobDiagnostic, error) {
	if rubrics.IsToolPackResult(raw) {
		pack, err := rubrics.ParseToolPackResult(raw)
		if err != nil {
			return nil, nil, err
		}
		return jobOutcomesFromToolPack(pack)
	}
	return jobOutcomeFromSingularToolResult(raw)
}

func jobOutcomesFromToolPack(pack rubrics.ToolPackResult) ([]coachapi.JobFinding, []coachapi.JobDiagnostic, error) {
	var findings []coachapi.JobFinding
	var diags []coachapi.JobDiagnostic
	for _, tr := range pack.Results {
		itemRaw, err := json.Marshal(tr)
		if err != nil {
			return nil, nil, err
		}
		af, d, err := jobOutcomeFromRubricTool(itemRaw, tr.FindingRef)
		if err != nil {
			return nil, nil, err
		}
		findings, diags = appendJobOutcome(findings, diags, af, d)
	}
	return findings, diags, nil
}

// jobOutcomeFromSingularToolResult handles the non-pack envelope (a
// one-item pack may also take this path). FindingRef is omitted as a
// discriminator entirely when empty, unlike jobOutcomesFromToolPack's
// per-item FindingRef, which is always passed even when empty.
func jobOutcomeFromSingularToolResult(raw json.RawMessage) ([]coachapi.JobFinding, []coachapi.JobDiagnostic, error) {
	var tr rubrics.ToolResult
	if err := json.Unmarshal(raw, &tr); err != nil {
		return nil, nil, fmt.Errorf("coachapi: decoding rubric tool result: %w", err)
	}
	var disc []string
	if tr.FindingRef != "" {
		disc = []string{tr.FindingRef}
	}
	af, d, err := jobOutcomeFromRubricTool(raw, disc...)
	if err != nil {
		return nil, nil, err
	}
	var findings []coachapi.JobFinding
	var diags []coachapi.JobDiagnostic
	findings, diags = appendJobOutcome(findings, diags, af, d)
	return findings, diags, nil
}

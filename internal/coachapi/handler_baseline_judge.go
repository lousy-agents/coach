package coachapi

import (
	"context"
	"encoding/json"

	"fmt"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/rubrics"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// judgmentBudgetExceededError is returned when the judgment-phase wall/tool
// budget stops the pack loop mid-phase. Agent findings from completed packs
// are already persisted; completeAfterJudgmentError records the diagnostic.
type judgmentBudgetExceededError struct {
	Judged    int
	Remaining int
	Err       error
}

func (e *judgmentBudgetExceededError) Error() string {
	return fmt.Sprintf("judgment_budget_exceeded judged=%d remaining=%d: %v", e.Judged, e.Remaining, e.Err)
}

func (e *judgmentBudgetExceededError) Unwrap() error { return e.Err }

// HM packs finished; wall died on cohesion. Report HM agent rows as judged.

// hiddenMutationCandidate pairs a deterministic finding with the decoded
// signal driving its judgment-pack membership.
type hiddenMutationCandidate struct {
	finding JobFinding
	sig     codesignal.Signal
}

// Priority cap before packing: select subset, then pack.

// Count successful agent rows only — diagnostics-only packs must not
// inflate judged= in the budget diagnostic.

// collectHiddenMutationCandidates builds one PackCandidate (plus its
// originating finding/signal) per deterministic hidden_input_mutation
// finding in detFindings.

// hiddenMutationPackItems assembles one judgment pack's rubric items,
// skipping any FindingRef no longer present in byRef.

// callHiddenMutationPack invokes the hidden-mutation-contextualization
// rubric for one pack and persists its findings incrementally, so a
// mid-phase budget stop keeps every already-completed pack.

// jobOutcomesFromHiddenMutationResult maps a singular ToolResult or pack
// {"results":[...]} envelope to job findings/diagnostics. Pack items use
// FindingRef (deterministic PayloadHash) as the payload_hash discriminator.

// jobOutcomeFromSingularToolResult handles the non-pack envelope (a
// one-item pack may also take this path). FindingRef is omitted as a
// discriminator entirely when empty, unlike jobOutcomesFromToolPack's
// per-item FindingRef, which is always passed even when empty.

// appendJobOutcome appends af/d onto findings/diags; either may be nil.

func judgeChangeCohesion(ctx context.Context, loop *agentloop.Loop, fileMetas []rubrics.FileMeta, detFindings []JobFinding) ([]JobFinding, []JobDiagnostic, error) {
	detPayloads := make([]json.RawMessage, 0, len(detFindings))
	for _, f := range detFindings {
		if f.Source == FindingSourceDeterministic {
			detPayloads = append(detPayloads, f.Payload)
		}
	}
	findingsJSON := json.RawMessage("[]")
	if len(detPayloads) > 0 {
		var err error
		findingsJSON, err = json.Marshal(detPayloads)
		if err != nil {
			return nil, nil, err
		}
	}
	cohesionArgs, err := json.Marshal(map[string]any{
		"findings": json.RawMessage(findingsJSON),
		"files":    fileMetas,
	})
	if err != nil {
		return nil, nil, err
	}
	raw, err := loop.Call(ctx, agentloop.CallSourceHandler, rubrics.IDChangeCohesion, cohesionArgs)
	if err != nil {
		return nil, nil, fmt.Errorf("coachapi: rubric %s: %w", rubrics.IDChangeCohesion, err)
	}
	af, d, err := jobOutcomeFromRubricTool(raw)
	if err != nil {
		return nil, nil, err
	}
	var agentFindings []JobFinding
	var diagnostics []JobDiagnostic
	if af != nil {
		agentFindings = append(agentFindings, *af)
	}
	if d != nil {
		diagnostics = append(diagnostics, *d)
	}
	return agentFindings, diagnostics, nil
}

// judgmentBudgetDiagnostic builds the stable diagnostic for mid-phase wall/tool budget stop.
func judgmentBudgetDiagnostic(judged, remaining int, err error) JobDiagnostic {
	msg := fmt.Sprintf("judgment_budget_exceeded judged=%d remaining=%d", judged, remaining)
	if err != nil {
		msg = fmt.Sprintf("%s: %v", msg, err)
	}
	return JobDiagnostic{
		ID:      watermill.NewUUID(),
		Scope:   "judgment_budget",
		Message: msg,
	}
}

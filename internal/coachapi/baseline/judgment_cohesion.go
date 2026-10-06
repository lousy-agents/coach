package baseline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func judgeChangeCohesion(ctx context.Context, loop *agentloop.Loop, fileMetas []rubrics.FileMeta, detFindings []coachapi.JobFinding) ([]coachapi.JobFinding, []coachapi.JobDiagnostic, error) {
	detPayloads := make([]json.RawMessage, 0, len(detFindings))
	for _, f := range detFindings {
		if f.Source == coachapi.FindingSourceDeterministic {
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
	var agentFindings []coachapi.JobFinding
	var diagnostics []coachapi.JobDiagnostic
	if af != nil {
		agentFindings = append(agentFindings, *af)
	}
	if d != nil {
		diagnostics = append(diagnostics, *d)
	}
	return agentFindings, diagnostics, nil
}

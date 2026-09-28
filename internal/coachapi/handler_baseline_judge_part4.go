package coachapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func judgeBaselineViaLoop(
	ctx context.Context,
	loop *agentloop.Loop,
	files []loadedBaselineFile,
	detFindings []JobFinding,
	w BaselineJobWriter,
	packCfg rubrics.PackConfig,
	maxHiddenMutationJudgments int,
) ([]JobFinding, []JobDiagnostic, error) {
	byPath := make(map[string]loadedBaselineFile, len(files))
	fileMetas := make([]rubrics.FileMeta, 0, len(files))
	for _, f := range files {
		byPath[f.Path] = f
		fileMetas = append(fileMetas, rubrics.FileMeta{
			Path:     f.Path,
			Language: string(f.Language),
		})
	}

	packCfg = rubrics.ApplyPackConfigDefaults(packCfg)
	maxJudgments := resolveMaxHiddenMutationJudgments(maxHiddenMutationJudgments)

	agentFindings, diagnostics, err := judgeHiddenMutationFindings(ctx, loop, byPath, detFindings, w, packCfg, maxJudgments)
	if err != nil {
		return agentFindings, diagnostics, err
	}

	cohesionFindings, cohesionDiags, err := judgeChangeCohesion(ctx, loop, fileMetas, detFindings)
	if err != nil {
		if errors.Is(err, agentloop.ErrBudgetExceeded) {

			return agentFindings, diagnostics, &judgmentBudgetExceededError{
				Judged:    len(agentFindings),
				Remaining: 0,
				Err:       err,
			}
		}
		return agentFindings, diagnostics, err
	}
	if err := insertBaselineFindings(ctx, w, cohesionFindings); err != nil {
		return agentFindings, diagnostics, err
	}
	agentFindings = append(agentFindings, cohesionFindings...)
	diagnostics = append(diagnostics, cohesionDiags...)
	return agentFindings, diagnostics, nil
}

// jobOutcomeFromSingularToolResult handles the non-pack envelope (a
// one-item pack may also take this path). FindingRef is omitted as a
// discriminator entirely when empty, unlike jobOutcomesFromToolPack's
// per-item FindingRef, which is always passed even when empty.
func jobOutcomeFromSingularToolResult(raw json.RawMessage) ([]JobFinding, []JobDiagnostic, error) {
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
	var findings []JobFinding
	var diags []JobDiagnostic
	findings, diags = appendJobOutcome(findings, diags, af, d)
	return findings, diags, nil
}

// appendJobOutcome appends af/d onto findings/diags; either may be nil.
func appendJobOutcome(findings []JobFinding, diags []JobDiagnostic, af *JobFinding, d *JobDiagnostic) ([]JobFinding, []JobDiagnostic) {
	if af != nil {
		findings = append(findings, *af)
	}
	if d != nil {
		diags = append(diags, *d)
	}
	return findings, diags
}

package coachapi

import (
	"context"
	"encoding/json"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func judgeHiddenMutationFindings(
	ctx context.Context,
	loop *agentloop.Loop,
	byPath map[string]loadedBaselineFile,
	detFindings []JobFinding,
	w BaselineJobWriter,
	packCfg rubrics.PackConfig,
	maxJudgments int,
) ([]JobFinding, []JobDiagnostic, error) {
	cands, metas := collectHiddenMutationCandidates(byPath, detFindings, packCfg)
	if len(cands) == 0 {
		return nil, nil, nil
	}

	byRef := make(map[string]hiddenMutationCandidate, len(metas))
	for i, c := range cands {
		byRef[c.FindingRef] = metas[i]
	}

	// Priority cap before packing: select subset, then pack.
	var diagnostics []JobDiagnostic
	selected, omitted := PrioritizeJudgmentCandidates(cands, maxJudgments)
	if omitted > 0 {
		diagnostics = append(diagnostics, judgmentCapDiagnostic(len(selected), omitted))
	}
	cands = selected

	packs := rubrics.PackJudgmentCandidates(cands, packCfg)
	total := len(cands)
	var (
		agentFindings []JobFinding
		judged        int
	)

	for _, pack := range packs {
		items := hiddenMutationPackItems(pack, byRef, byPath, packCfg)
		if len(items) == 0 {
			continue
		}

		packFindings, packDiags, err := callHiddenMutationPack(ctx, loop, w, items, total, judged)
		if err != nil {
			return agentFindings, diagnostics, err
		}
		agentFindings = append(agentFindings, packFindings...)
		diagnostics = append(diagnostics, packDiags...)

		judged += len(packFindings)
	}
	return agentFindings, diagnostics, nil
}
func jobOutcomesFromToolPack(pack rubrics.ToolPackResult) ([]JobFinding, []JobDiagnostic, error) {
	var findings []JobFinding
	var diags []JobDiagnostic
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

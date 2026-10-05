package baseline

import (
	"context"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/rubrics"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// hiddenMutationCandidate pairs a deterministic finding with the decoded
// signal driving its judgment-pack membership.
type hiddenMutationCandidate struct {
	finding coachapi.JobFinding
	sig     codesignal.Signal
}

func judgeHiddenMutationFindings(
	ctx context.Context,
	loop *agentloop.Loop,
	byPath map[string]loadedBaselineFile,
	detFindings []coachapi.JobFinding,
	w JobWriter,
	packCfg rubrics.PackConfig,
	maxJudgments int,
) ([]coachapi.JobFinding, []coachapi.JobDiagnostic, error) {
	cands, metas := collectHiddenMutationCandidates(byPath, detFindings, packCfg)
	if len(cands) == 0 {
		return nil, nil, nil
	}

	byRef := make(map[string]hiddenMutationCandidate, len(metas))
	for i, c := range cands {
		byRef[c.FindingRef] = metas[i]
	}

	// Priority cap before packing: select subset, then pack.
	var diagnostics []coachapi.JobDiagnostic
	selected, omitted := PrioritizeJudgmentCandidates(cands, maxJudgments)
	if omitted > 0 {
		diagnostics = append(diagnostics, judgmentCapDiagnostic(len(selected), omitted))
	}
	cands = selected

	packs := rubrics.PackJudgmentCandidates(cands, packCfg)
	total := len(cands)
	var (
		agentFindings []coachapi.JobFinding
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
		// Count successful agent rows only — diagnostics-only packs must not
		// inflate judged= in the budget diagnostic.
		judged += len(packFindings)
	}
	return agentFindings, diagnostics, nil
}

func countHiddenMutationFindings(detFindings []coachapi.JobFinding) int {
	n := 0
	for _, f := range detFindings {
		if f.Source != coachapi.FindingSourceDeterministic {
			continue
		}
		if _, ok := hiddenMutationSignal(f.Payload); ok {
			n++
		}
	}
	return n
}

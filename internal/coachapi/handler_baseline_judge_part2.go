package coachapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/rubrics"
)

// callHiddenMutationPack invokes the hidden-mutation-contextualization
// rubric for one pack and persists its findings incrementally, so a
// mid-phase budget stop keeps every already-completed pack.
func callHiddenMutationPack(
	ctx context.Context,
	loop *agentloop.Loop,
	w BaselineJobWriter,
	items []rubrics.HiddenMutationPackItem,
	total, judged int,
) ([]JobFinding, []JobDiagnostic, error) {
	args, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		return nil, nil, err
	}
	raw, err := loop.Call(ctx, agentloop.CallSourceHandler, rubrics.IDHiddenMutationContextualization, args)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, nil, err
		}
		if errors.Is(err, agentloop.ErrBudgetExceeded) {
			return nil, nil, &judgmentBudgetExceededError{
				Judged:    judged,
				Remaining: total - judged,
				Err:       err,
			}
		}
		return nil, nil, fmt.Errorf("coachapi: rubric %s: %w", rubrics.IDHiddenMutationContextualization, err)
	}

	packFindings, packDiags, err := jobOutcomesFromHiddenMutationResult(raw)
	if err != nil {
		return nil, nil, err
	}
	if err := insertBaselineFindings(ctx, w, packFindings); err != nil {
		return nil, nil, err
	}
	return packFindings, packDiags, nil
}

// collectHiddenMutationCandidates builds one PackCandidate (plus its
// originating finding/signal) per deterministic hidden_input_mutation
// finding in detFindings.
func collectHiddenMutationCandidates(
	byPath map[string]loadedBaselineFile,
	detFindings []JobFinding,
	packCfg rubrics.PackConfig,
) ([]rubrics.PackCandidate, []hiddenMutationCandidate) {
	var (
		cands []rubrics.PackCandidate
		metas []hiddenMutationCandidate
	)
	for _, f := range detFindings {
		if f.Source != FindingSourceDeterministic {
			continue
		}
		sig, ok := hiddenMutationSignal(f.Payload)
		if !ok {
			continue
		}
		lf, found := byPath[sig.Path]
		if !found {
			lf = loadedBaselineFile{Path: sig.Path}
		}
		startRow := int(sig.Location.StartRow)
		window := rubrics.FormatSpanWindow(lf.Content, startRow, packCfg.EvidenceWindowLines)
		cands = append(cands, rubrics.PackCandidate{
			FindingRef:    f.PayloadHash,
			Path:          sig.Path,
			StartRow:      startRow,
			Severity:      string(sig.Severity),
			Confidence:    string(sig.Confidence),
			PayloadJSON:   append([]byte(nil), f.Payload...),
			EvidenceChars: len(window),
		})
		metas = append(metas, hiddenMutationCandidate{finding: f, sig: sig})
	}
	return cands, metas
}

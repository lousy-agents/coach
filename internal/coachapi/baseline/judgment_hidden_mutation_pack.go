package baseline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/rubrics"
)

// callHiddenMutationPack invokes the hidden-mutation-contextualization
// rubric for one pack and persists its findings incrementally, so a
// mid-phase budget stop keeps every already-completed pack.
func callHiddenMutationPack(
	ctx context.Context,
	loop *agentloop.Loop,
	w JobWriter,
	items []rubrics.HiddenMutationPackItem,
	total, judged int,
) ([]coachapi.JobFinding, []coachapi.JobDiagnostic, error) {
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

// hiddenMutationPackItems assembles one judgment pack's rubric items,
// skipping any FindingRef no longer present in byRef.
func hiddenMutationPackItems(
	pack rubrics.JudgmentPack,
	byRef map[string]hiddenMutationCandidate,
	byPath map[string]loadedBaselineFile,
	packCfg rubrics.PackConfig,
) []rubrics.HiddenMutationPackItem {
	items := make([]rubrics.HiddenMutationPackItem, 0, len(pack.FindingRefs))
	for _, ref := range pack.FindingRefs {
		meta, ok := byRef[ref]
		if !ok {
			continue
		}
		lf, found := byPath[meta.sig.Path]
		if !found {
			lf = loadedBaselineFile{Path: meta.sig.Path}
		}
		window := rubrics.FormatSpanWindow(lf.Content, int(meta.sig.Location.StartRow), packCfg.EvidenceWindowLines)
		items = append(items, rubrics.HiddenMutationPackItem{
			FindingRef: ref,
			Finding:    append(json.RawMessage(nil), meta.finding.Payload...),
			File: rubrics.FileContext{
				Path:     lf.Path,
				Language: string(lf.Language),
				Content:  window,
			},
		})
	}
	return items
}

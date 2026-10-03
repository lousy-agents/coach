package coachapi

import (
	"encoding/json"

	"github.com/lousy-agents/coach/internal/rubrics"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

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
func hiddenMutationSignal(payload json.RawMessage) (codesignal.Signal, bool) {
	var sig codesignal.Signal
	if err := json.Unmarshal(payload, &sig); err != nil {
		return codesignal.Signal{}, false
	}
	if sig.Kind != "hidden_input_mutation" && sig.RuleID != "state.hidden_input_mutation" {
		return codesignal.Signal{}, false
	}
	return sig, true
}

// jobOutcomesFromHiddenMutationResult maps a singular ToolResult or pack
// {"results":[...]} envelope to job findings/diagnostics. Pack items use
// FindingRef (deterministic PayloadHash) as the payload_hash discriminator.
func jobOutcomesFromHiddenMutationResult(raw json.RawMessage) ([]JobFinding, []JobDiagnostic, error) {
	if rubrics.IsToolPackResult(raw) {
		pack, err := rubrics.ParseToolPackResult(raw)
		if err != nil {
			return nil, nil, err
		}
		return jobOutcomesFromToolPack(pack)
	}
	return jobOutcomeFromSingularToolResult(raw)
}

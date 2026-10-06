package baseline

import (
	"encoding/json"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/rubrics"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// collectHiddenMutationCandidates builds one PackCandidate (plus its
// originating finding/signal) per deterministic hidden_input_mutation
// finding in detFindings.
func collectHiddenMutationCandidates(
	byPath map[string]loadedBaselineFile,
	detFindings []coachapi.JobFinding,
	packCfg rubrics.PackConfig,
) ([]rubrics.PackCandidate, []hiddenMutationCandidate) {
	var (
		cands []rubrics.PackCandidate
		metas []hiddenMutationCandidate
	)
	for _, f := range detFindings {
		if f.Source != coachapi.FindingSourceDeterministic {
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

package baseline_test

import (
	"encoding/json"
	"strings"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/rubrics"
)

// agentHiddenPathsViaJudgedRefs maps agent rows back to deterministic paths using
// PayloadHash as finding_ref (pack path stores deterministic hash on agent row).
func agentHiddenPathsViaJudgedRefs(findings []coachapi.JobFinding) map[string]int {
	detByHash := deterministicHiddenMutationPathsByHash(findings)

	paths := map[string]int{}
	for _, f := range findings {
		if f.Source != coachapi.FindingSourceAgent {
			continue
		}
		if f.RubricID == nil || *f.RubricID != rubrics.IDHiddenMutationContextualization {
			continue
		}
		if p, ok := detByHash[f.PayloadHash]; ok {
			paths[p]++
			continue
		}
		// Fallback: finding_ref inside payload.
		var body struct {
			FindingRef string `json:"finding_ref"`
			Path       string `json:"path"`
		}
		_ = json.Unmarshal(f.Payload, &body)
		if body.Path != "" {
			paths[body.Path]++
			continue
		}
		if p, ok := detByHash[body.FindingRef]; ok {
			paths[p]++
		}
	}
	return paths
}

// deterministicHiddenMutationPathsByHash maps each deterministic
// hidden_input_mutation finding's PayloadHash to its signal path.
func deterministicHiddenMutationPathsByHash(findings []coachapi.JobFinding) map[string]string {
	detByHash := map[string]string{}
	for _, f := range findings {
		if f.Source != coachapi.FindingSourceDeterministic {
			continue
		}
		if !strings.Contains(string(f.Payload), "hidden_input_mutation") {
			continue
		}
		var sig struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(f.Payload, &sig) != nil || sig.Path == "" {
			continue
		}
		detByHash[f.PayloadHash] = sig.Path
	}
	return detByHash
}

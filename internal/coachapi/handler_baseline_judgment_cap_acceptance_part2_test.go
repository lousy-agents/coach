package coachapi_test

import (
	"encoding/json"

	"github.com/lousy-agents/coach/internal/coachapi"

	"github.com/lousy-agents/coach/internal/rubrics"
)

// agentHiddenPathsViaJudgedRefs maps agent rows back to deterministic paths using
// PayloadHash as finding_ref (pack path stores deterministic hash on agent row).
func agentHiddenPathsViaJudgedRefs(findings []coachapi.JobFinding) map[string]int {
	detByHash := map[string]string{}
	(&sigagentHiddenPathsViaJudgedRefsS1{detByHash: detByHash, findings: findings}).call()

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
		(&sigagentHiddenPathsViaJudgedRefsS6{body: body, detByHash: detByHash, paths: paths}).call()

	}
	return paths
}

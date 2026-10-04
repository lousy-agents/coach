package rubrics_test

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/rubrics"
)

// buildPathCandidates creates n small findings on path with stable refs path#i.
func buildPathCandidates(path string, n int) []rubrics.PackCandidate {
	out := make([]rubrics.PackCandidate, 0, n)
	for i := 0; i < n; i++ {
		ref := fmt.Sprintf("%s#%d", path, i+1)

		out = append(out, candidate(ref, path, (i+1)*10, 40, `{"k":1}`))
	}
	return out
}

func refPathIndex(cands []rubrics.PackCandidate) map[string]string {
	m := make(map[string]string, len(cands))
	for _, c := range cands {
		m[c.FindingRef] = c.Path
	}
	return m
}

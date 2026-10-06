package rubrics_test

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/rubrics"
)

func candidate(ref, path string, startRow, evidenceChars int, payload string) rubrics.PackCandidate {
	return rubrics.PackCandidate{
		FindingRef:    ref,
		Path:          path,
		StartRow:      startRow,
		PayloadJSON:   []byte(payload),
		EvidenceChars: evidenceChars,
	}
}

// buildPathCandidates creates n small findings on path with stable refs path#i.
func buildPathCandidates(path string, n int) []rubrics.PackCandidate {
	out := make([]rubrics.PackCandidate, 0, n)
	for i := 0; i < n; i++ {
		ref := fmt.Sprintf("%s#%d", path, i+1)

		// Tiny payload so token caps do not force extra splits in affinity/merge tests.
		out = append(out, candidate(ref, path, (i+1)*10, 40, `{"k":1}`))
	}
	return out
}

func twoHundredBytePayload() []byte {
	p := make([]byte, 200)
	for i := range p {
		p[i] = 'x'
	}
	return p
}

func refPathIndex(cands []rubrics.PackCandidate) map[string]string {
	m := make(map[string]string, len(cands))
	for _, c := range cands {
		m[c.FindingRef] = c.Path
	}
	return m
}

// pathsInPack returns the set of path prefixes encoded in finding refs of form "path#n".
func pathsInPack(refs []string, refPath map[string]string) []string {
	seen := map[string]struct{}{}
	var paths []string
	for _, ref := range refs {
		p := refPath[ref]
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	return paths
}

// classifyPackPaths reports whether paths include hotPath and whether they
// include any other path.
func classifyPackPaths(paths []string, hotPath string) (hasHot, hasOther bool) {
	for _, path := range paths {
		if path == hotPath {
			hasHot = true
		} else {
			hasOther = true
		}
	}
	return hasHot, hasOther
}

// packRefs flattens pack finding refs for stable boundary comparisons.
func packRefs(packs []rubrics.JudgmentPack) [][]string {
	out := make([][]string, len(packs))
	for i, p := range packs {
		out[i] = append([]string(nil), p.FindingRefs...)
	}
	return out
}

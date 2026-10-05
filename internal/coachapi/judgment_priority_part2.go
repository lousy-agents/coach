package coachapi

import (
	"sort"

	"github.com/lousy-agents/coach/internal/rubrics"
)

// judgmentCandidateLess reports whether a should be preferred before b within a path
// (higher severity, then higher confidence, then lower start_row, then FindingRef).
func judgmentCandidateLess(a, b rubrics.PackCandidate) bool {
	if ra, rb := severityRankString(a.Severity), severityRankString(b.Severity); ra != rb {
		return ra > rb
	}
	if ra, rb := confidenceRankString(a.Confidence), confidenceRankString(b.Confidence); ra != rb {
		return ra > rb
	}
	if a.StartRow != b.StartRow {
		return a.StartRow < b.StartRow
	}
	return a.FindingRef < b.FindingRef
}

// groupCandidatesByPath buckets cands by path, each bucket in per-path
// priority order (judgmentCandidateLess).
func groupCandidatesByPath(cands []rubrics.PackCandidate) map[string][]rubrics.PackCandidate {
	byPath := make(map[string][]rubrics.PackCandidate)
	for _, c := range cands {
		byPath[c.Path] = append(byPath[c.Path], c)
	}
	for path := range byPath {
		sort.SliceStable(byPath[path], func(i, j int) bool {
			return judgmentCandidateLess(byPath[path][i], byPath[path][j])
		})
	}
	return byPath
}

// pathsByFindingCount orders byPath's keys by finding count descending, then
// path ascending -- a total order, so map iteration order never leaks into
// the round-robin result.
func pathsByFindingCount(byPath map[string][]rubrics.PackCandidate) []string {
	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.SliceStable(paths, func(i, j int) bool {
		if len(byPath[paths[i]]) != len(byPath[paths[j]]) {
			return len(byPath[paths[i]]) > len(byPath[paths[j]])
		}
		return paths[i] < paths[j]
	})
	return paths
}

// resolveMaxHiddenMutationJudgments applies config defaults for the judgment cap.
// Zero → DefaultMaxHiddenMutationJudgments (16). Negative → unlimited (-1).
func resolveMaxHiddenMutationJudgments(n int) int {
	if n < 0 {
		return -1
	}
	if n == 0 {
		return DefaultMaxHiddenMutationJudgments
	}
	return n
}
func confidenceRankString(c string) int {
	switch c {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

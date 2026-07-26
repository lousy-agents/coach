package coachapi

import (
	"fmt"
	"sort"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/rubrics"
)

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

// PrioritizeJudgmentCandidates selects up to max candidates under the binding
// Story 3 policy:
//  1. Sort paths by finding count descending, then path ascending.
//  2. Within each path: severity desc, confidence desc, start_row asc, FindingRef asc.
//  3. Round-robin one finding per path until the cap is reached.
//
// max < 0 means unlimited (return all in stable path-RR order for packing).
// max == 0 returns nil (callers should resolve defaults first).
// When len(cands) <= max, all candidates are returned in selection order and omitted is 0.
func PrioritizeJudgmentCandidates(cands []rubrics.PackCandidate, max int) (selected []rubrics.PackCandidate, omitted int) {
	if len(cands) == 0 {
		return nil, 0
	}
	if max < 0 {
		// Unlimited: still apply path ordering + within-path priority for stable packing input.
		return prioritizeAllRoundRobin(cands), 0
	}
	if max == 0 {
		return nil, len(cands)
	}
	if len(cands) <= max {
		return prioritizeAllRoundRobin(cands), 0
	}

	selected = prioritizeRoundRobin(cands, max)
	return selected, len(cands) - len(selected)
}

func prioritizeAllRoundRobin(cands []rubrics.PackCandidate) []rubrics.PackCandidate {
	return prioritizeRoundRobin(cands, len(cands))
}

func prioritizeRoundRobin(cands []rubrics.PackCandidate, max int) []rubrics.PackCandidate {
	// Group by path preserving per-path priority order.
	byPath := make(map[string][]rubrics.PackCandidate)
	for _, c := range cands {
		byPath[c.Path] = append(byPath[c.Path], c)
	}
	for path := range byPath {
		sort.SliceStable(byPath[path], func(i, j int) bool {
			return judgmentCandidateLess(byPath[path][i], byPath[path][j])
		})
	}

	type pathGroup struct {
		path  string
		count int
	}
	groups := make([]pathGroup, 0, len(byPath))
	for path, items := range byPath {
		groups = append(groups, pathGroup{path: path, count: len(items)})
	}
	// Paths: finding count descending, then path ascending.
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].count != groups[j].count {
			return groups[i].count > groups[j].count
		}
		return groups[i].path < groups[j].path
	})

	// Round-robin indices into each path's ordered queue.
	idx := make(map[string]int, len(groups))
	selected := make([]rubrics.PackCandidate, 0, max)
	for len(selected) < max {
		progress := false
		for _, g := range groups {
			if len(selected) >= max {
				break
			}
			i := idx[g.path]
			items := byPath[g.path]
			if i >= len(items) {
				continue
			}
			selected = append(selected, items[i])
			idx[g.path] = i + 1
			progress = true
		}
		if !progress {
			break
		}
	}
	return selected
}

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

func severityRankString(s string) int {
	switch s {
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

// judgmentCapDiagnostic builds the stable diagnostic when the priority cap omits findings.
func judgmentCapDiagnostic(selected, omitted int) JobDiagnostic {
	return JobDiagnostic{
		ID:      watermill.NewUUID(),
		Scope:   "judgment_cap",
		Message: fmt.Sprintf("judgment_cap_omitted selected=%d omitted=%d", selected, omitted),
	}
}

package baseline

import (
	"fmt"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/rubrics"
)

// PrioritizeJudgmentCandidates selects up to max candidates under the binding
// policy:
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
	byPath := groupCandidatesByPath(cands)
	paths := pathsByFindingCount(byPath)

	pick := &roundRobinPick{
		idx:      make(map[string]int, len(paths)),
		selected: make([]rubrics.PackCandidate, 0, max),
		max:      max,
		byPath:   byPath,
	}
	for len(pick.selected) < max {
		if !pick.sweep(paths) {
			break
		}
	}
	return pick.selected
}

type roundRobinPick struct {
	// Round-robin indices into each path's ordered queue.
	idx      map[string]int
	selected []rubrics.PackCandidate
	max      int
	byPath   map[string][]rubrics.PackCandidate
}

func (p *roundRobinPick) sweep(paths []string) bool {
	progress := false
	for _, path := range paths {
		if len(p.selected) >= p.max {
			break
		}
		i := p.idx[path]
		items := p.byPath[path]
		if i >= len(items) {
			continue
		}
		p.selected = append(p.selected, items[i])
		p.idx[path] = i + 1
		progress = true
	}
	return progress
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

// judgmentCapDiagnostic builds the stable diagnostic when the priority cap omits findings.
func judgmentCapDiagnostic(selected, omitted int) coachapi.JobDiagnostic {
	return coachapi.JobDiagnostic{
		ID:      watermill.NewUUID(),
		Scope:   "judgment_cap",
		Message: fmt.Sprintf("judgment_cap_omitted selected=%d omitted=%d", selected, omitted),
	}
}

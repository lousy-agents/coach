package rubrics

import (
	"sort"
)

// PackCandidate is one deterministic finding eligible for judgment packing.
type PackCandidate struct {
	FindingRef    string
	Path          string
	StartRow      int
	Severity      string
	Confidence    string
	PayloadJSON   []byte
	EvidenceChars int
}

// JudgmentPack is one gateway Judge batch: ordered finding refs after packing.
type JudgmentPack struct {
	FindingRefs []string
}

// PackJudgmentCandidates forms deterministic judgment packs from candidates.
//
// Ordering: path ascending, then start_row ascending, then FindingRef ascending.
// Paths with finding count ≥ JudgmentFileAffinityMinFindings stay in path-dedicated
// packs (still split by max findings / token budget). Paths below the threshold may
// cross-file merge under the same caps. A single candidate always fits in its own
// pack even if its estimate alone exceeds MaxJudgmentPromptTokens.
func PackJudgmentCandidates(cands []PackCandidate, cfg PackConfig) []JudgmentPack {
	cfg = ApplyPackConfigDefaults(cfg)
	if len(cands) == 0 {
		return nil
	}

	sorted := append([]PackCandidate(nil), cands...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.StartRow != b.StartRow {
			return a.StartRow < b.StartRow
		}
		return a.FindingRef < b.FindingRef
	})

	pathCounts := make(map[string]int, len(sorted))
	for _, c := range sorted {
		pathCounts[c.Path]++
	}

	// Preserve first-seen path order from the sorted slice.
	var pathOrder []string
	seenPath := make(map[string]struct{}, len(pathCounts))
	byPath := make(map[string][]PackCandidate, len(pathCounts))
	for _, c := range sorted {
		if _, ok := seenPath[c.Path]; !ok {
			seenPath[c.Path] = struct{}{}
			pathOrder = append(pathOrder, c.Path)
		}
		byPath[c.Path] = append(byPath[c.Path], c)
	}

	var packs []JudgmentPack
	var mergeable []PackCandidate
	for _, path := range pathOrder {
		group := byPath[path]
		if pathCounts[path] >= cfg.JudgmentFileAffinityMinFindings {
			packs = append(packs, packGreedy(group, cfg)...)
			continue
		}
		mergeable = append(mergeable, group...)
	}
	if len(mergeable) > 0 {
		packs = append(packs, packGreedy(mergeable, cfg)...)
	}
	return packs
}

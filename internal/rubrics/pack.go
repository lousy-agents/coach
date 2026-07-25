package rubrics

import "sort"

// PackConfig controls deterministic judgment packing for local-LLM hidden-mutation
// batches. Zero fields are filled by ApplyPackConfigDefaults.
type PackConfig struct {
	MaxFindingsPerJudgmentPack      int
	MaxJudgmentPromptTokens         int
	JudgmentFileAffinityMinFindings int
	EvidenceWindowLines             int
}

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

// Default pack knobs (local-LLM oriented; see coach-api-platform-local-llm-judgment spec).
const (
	DefaultMaxFindingsPerJudgmentPack      = 4
	DefaultMaxJudgmentPromptTokens         = 3500
	DefaultJudgmentFileAffinityMinFindings = 5
	DefaultEvidenceWindowLines             = 15
)

// packPromptOverheadTokens is a fixed chars/4-style allowance for rubric/system
// prompt text shared by every pack (not per finding).
const packPromptOverheadTokens = 64

// ApplyPackConfigDefaults returns cfg with zero-valued fields set to binding defaults.
func ApplyPackConfigDefaults(cfg PackConfig) PackConfig {
	if cfg.MaxFindingsPerJudgmentPack == 0 {
		cfg.MaxFindingsPerJudgmentPack = DefaultMaxFindingsPerJudgmentPack
	}
	if cfg.MaxJudgmentPromptTokens == 0 {
		cfg.MaxJudgmentPromptTokens = DefaultMaxJudgmentPromptTokens
	}
	if cfg.JudgmentFileAffinityMinFindings == 0 {
		cfg.JudgmentFileAffinityMinFindings = DefaultJudgmentFileAffinityMinFindings
	}
	if cfg.EvidenceWindowLines == 0 {
		cfg.EvidenceWindowLines = DefaultEvidenceWindowLines
	}
	return cfg
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

// packGreedy fills packs left-to-right under max-findings and token caps.
func packGreedy(cands []PackCandidate, cfg PackConfig) []JudgmentPack {
	if len(cands) == 0 {
		return nil
	}
	var packs []JudgmentPack
	var cur []PackCandidate
	curTokens := packPromptOverheadTokens

	flush := func() {
		if len(cur) == 0 {
			return
		}
		refs := make([]string, len(cur))
		for i, c := range cur {
			refs[i] = c.FindingRef
		}
		packs = append(packs, JudgmentPack{FindingRefs: refs})
		cur = nil
		curTokens = packPromptOverheadTokens
	}

	for _, c := range cands {
		itemTokens := estimateCandidateTokens(c)
		if len(cur) == 0 {
			cur = append(cur, c)
			curTokens += itemTokens
			continue
		}
		if len(cur)+1 > cfg.MaxFindingsPerJudgmentPack ||
			curTokens+itemTokens > cfg.MaxJudgmentPromptTokens {
			flush()
			cur = append(cur, c)
			curTokens += itemTokens
			continue
		}
		cur = append(cur, c)
		curTokens += itemTokens
	}
	flush()
	return packs
}

// estimateCandidateTokens uses a chars/4 estimator over payload bytes + evidence chars.
func estimateCandidateTokens(c PackCandidate) int {
	chars := len(c.PayloadJSON) + c.EvidenceChars
	if chars <= 0 {
		return 0
	}
	return (chars + 3) / 4
}

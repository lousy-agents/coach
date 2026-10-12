package rubrics

// packPromptOverheadTokens is a fixed chars/4-style allowance for rubric/system
// prompt text shared by every pack (not per finding).
const packPromptOverheadTokens = 64

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

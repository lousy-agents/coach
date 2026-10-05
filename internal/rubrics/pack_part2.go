package rubrics

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

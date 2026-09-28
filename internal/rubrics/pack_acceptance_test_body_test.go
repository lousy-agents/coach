package rubrics_test

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/rubrics"
)

func body_packAcceptanceTest_emitsAStablePackCountAndNeverMergesTheHotPathWit_59(cfg rubrics.PackConfig, cands []rubrics.PackCandidate, byPath map[string]string) {
	packs := rubrics.PackJudgmentCandidates(cands, cfg)
	Expect(packs).NotTo(BeEmpty())

	Expect(len(packs)).To(BeNumerically("<", len(cands)))
	Expect(len(packs)).To(Equal(4))

	hotPath := "pkg/hot/service.go"
	for _, p := range packs {
		paths := pathsInPack(p.FindingRefs, byPath)
		hasHot := false
		hasOther := false
		(&sigbodypackAcceptanceTestemitsAStablePackCountAndNeverMerges{hasHot: &hasHot, hasOther: &hasOther, hotPath: hotPath, paths: paths}).call()

		Expect(hasHot && hasOther).To(BeFalse(),
			"hot path must not share a pack with other paths; pack=%v paths=%v",
			p.FindingRefs, paths)
	}

	seen := map[string]int{}
	for _, p := range packs {
		Expect(len(p.FindingRefs)).To(BeNumerically("<=", cfg.MaxFindingsPerJudgmentPack))
		for _, ref := range p.FindingRefs {
			seen[ref]++
		}
	}
	Expect(seen).To(HaveLen(len(cands)))
	for ref, n := range seen {
		Expect(n).To(Equal(1), "finding %s packed %d times", ref, n)
	}
}

func body_packAcceptanceTest_123() []byte {
	p := make([]byte, 200)
	for i := range p {
		p[i] = 'x'
	}
	return p
}

func body_packAcceptanceTest_splitsPacksByEstimatedPromptTokensChars4WhenOneI_139(threeBig func() []rubrics.PackCandidate) {
	tight := rubrics.PackConfig{
		MaxFindingsPerJudgmentPack:      4,
		MaxJudgmentPromptTokens:         150,
		JudgmentFileAffinityMinFindings: 5,
		EvidenceWindowLines:             15,
	}
	packs := rubrics.PackJudgmentCandidates(threeBig(), tight)
	Expect(packs).To(HaveLen(3))
	for _, p := range packs {
		Expect(p.FindingRefs).To(HaveLen(1))
	}
	Expect(packRefs(packs)).To(Equal([][]string{
		{"big.go#1"},
		{"big.go#2"},
		{"big.go#3"},
	}))
}

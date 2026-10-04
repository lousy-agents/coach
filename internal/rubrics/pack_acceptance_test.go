package rubrics_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/rubrics"
)

var _ = Describe("judgment pack planner", func() {
	Describe("ApplyPackConfigDefaults", func() {
		It("fills zero fields with local-LLM binding defaults", func() {
			got := rubrics.ApplyPackConfigDefaults(rubrics.PackConfig{})
			Expect(got.MaxFindingsPerJudgmentPack).To(Equal(4))
			Expect(got.MaxJudgmentPromptTokens).To(Equal(3500))
			Expect(got.JudgmentFileAffinityMinFindings).To(Equal(5))
			Expect(got.EvidenceWindowLines).To(Equal(15))
		})

		It("preserves explicitly set non-zero fields", func() {
			got := rubrics.ApplyPackConfigDefaults(rubrics.PackConfig{
				MaxFindingsPerJudgmentPack:      2,
				MaxJudgmentPromptTokens:         1000,
				JudgmentFileAffinityMinFindings: 3,
				EvidenceWindowLines:             8,
			})
			Expect(got.MaxFindingsPerJudgmentPack).To(Equal(2))
			Expect(got.MaxJudgmentPromptTokens).To(Equal(1000))
			Expect(got.JudgmentFileAffinityMinFindings).To(Equal(3))
			Expect(got.EvidenceWindowLines).To(Equal(8))
		})
	})

	Describe("PackJudgmentCandidates", func() {
		var cfg rubrics.PackConfig

		BeforeEach(func() {
			cfg = rubrics.ApplyPackConfigDefaults(rubrics.PackConfig{})
		})

		When("a fixture has one hot path (≥6 findings) and ≥2 colder paths (≥12 findings, ≥3 paths)", func() {
			var cands []rubrics.PackCandidate
			var byPath map[string]string

			BeforeEach(func() {

				cands = nil
				cands = append(cands, buildPathCandidates("pkg/hot/service.go", 6)...)
				cands = append(cands, buildPathCandidates("pkg/cold/a.go", 3)...)
				cands = append(cands, buildPathCandidates("pkg/cold/b.go", 3)...)

				cands = []rubrics.PackCandidate{
					cands[8], cands[1], cands[11], cands[0], cands[5], cands[9],
					cands[2], cands[7], cands[3], cands[10], cands[4], cands[6],
				}
				byPath = refPathIndex(cands)
			})

			It("emits a stable pack count and never merges the hot path with other paths", func() {
				body_packAcceptanceTest_emitsAStablePackCountAndNeverMergesTheHotPathWit_59(cfg, cands, byPath)
			})

			It("produces identical pack boundaries for identical inputs (deterministic)", func() {
				a := packRefs(rubrics.PackJudgmentCandidates(cands, cfg))
				b := packRefs(rubrics.PackJudgmentCandidates(append([]rubrics.PackCandidate(nil), cands...), cfg))
				Expect(a).To(Equal(b))
			})
		})

		When("a single path exceeds max findings per pack", func() {
			It("splits the path into multiple path-dedicated packs by max findings", func() {
				cands := buildPathCandidates("only/file.go", 10)
				packs := rubrics.PackJudgmentCandidates(cands, cfg)

				Expect(packs).To(HaveLen(3))
				Expect(packs[0].FindingRefs).To(Equal([]string{
					"only/file.go#1", "only/file.go#2", "only/file.go#3", "only/file.go#4",
				}))
				Expect(packs[1].FindingRefs).To(Equal([]string{
					"only/file.go#5", "only/file.go#6", "only/file.go#7", "only/file.go#8",
				}))
				Expect(packs[2].FindingRefs).To(Equal([]string{
					"only/file.go#9", "only/file.go#10",
				}))
			})
		})

		When("a single path's findings exceed the token budget before max findings", func() {

			bigPayload := func() []byte {
				return body_packAcceptanceTest_123()
			}
			threeBig := func() []rubrics.PackCandidate {
				p := bigPayload()
				return []rubrics.PackCandidate{
					{FindingRef: "big.go#1", Path: "big.go", StartRow: 1, PayloadJSON: p, EvidenceChars: 0},
					{FindingRef: "big.go#2", Path: "big.go", StartRow: 2, PayloadJSON: p, EvidenceChars: 0},
					{FindingRef: "big.go#3", Path: "big.go", StartRow: 3, PayloadJSON: p, EvidenceChars: 0},
				}
			}

			It("splits packs by estimated prompt tokens (chars/4) when one item fits and two do not", func() {
				body_packAcceptanceTest_splitsPacksByEstimatedPromptTokensChars4WhenOneI_139(threeBig)
			})

			It("packs more than one finding under the token budget when two fit and three do not", func() {

				roomy := rubrics.PackConfig{
					MaxFindingsPerJudgmentPack:      4,
					MaxJudgmentPromptTokens:         200,
					JudgmentFileAffinityMinFindings: 5,
					EvidenceWindowLines:             15,
				}
				packs := rubrics.PackJudgmentCandidates(threeBig(), roomy)
				Expect(packRefs(packs)).To(Equal([][]string{
					{"big.go#1", "big.go#2"},
					{"big.go#3"},
				}))
			})
		})

		When("paths fall below the file-affinity density threshold", func() {
			It("cross-file merges findings under token and max-findings caps", func() {

				cands := append(buildPathCandidates("pkg/a.go", 2), buildPathCandidates("pkg/b.go", 2)...)

				cands = []rubrics.PackCandidate{cands[3], cands[1], cands[2], cands[0]}

				packs := rubrics.PackJudgmentCandidates(cands, cfg)
				Expect(packs).To(HaveLen(1))
				Expect(packs[0].FindingRefs).To(Equal([]string{
					"pkg/a.go#1", "pkg/a.go#2", "pkg/b.go#1", "pkg/b.go#2",
				}))
			})

			It("does not exceed max findings when cross-file merging", func() {

				cands := append(buildPathCandidates("z/a.go", 3), buildPathCandidates("z/b.go", 3)...)
				packs := rubrics.PackJudgmentCandidates(cands, cfg)
				Expect(packs).To(HaveLen(2))
				Expect(packs[0].FindingRefs).To(Equal([]string{
					"z/a.go#1", "z/a.go#2", "z/a.go#3", "z/b.go#1",
				}))
				Expect(packs[1].FindingRefs).To(Equal([]string{
					"z/b.go#2", "z/b.go#3",
				}))
			})
		})

		When("candidates arrive unsorted", func() {
			It("sorts by path ascending, then start_row ascending, then finding_ref ascending", func() {
				cands := []rubrics.PackCandidate{
					candidate("b#2", "b.go", 20, 10, `{}`),
					candidate("a#2", "a.go", 20, 10, `{}`),
					candidate("a#1b", "a.go", 10, 10, `{}`),
					candidate("a#1a", "a.go", 10, 10, `{}`),
					candidate("b#1", "b.go", 10, 10, `{}`),
				}
				packs := rubrics.PackJudgmentCandidates(cands, cfg)

				Expect(packs).To(HaveLen(2))
				Expect(packs[0].FindingRefs).To(Equal([]string{
					"a#1a", "a#1b", "a#2", "b#1",
				}))
				Expect(packs[1].FindingRefs).To(Equal([]string{"b#2"}))
			})
		})
	})
})

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

// packRefs flattens pack finding refs for stable boundary comparisons.
func packRefs(packs []rubrics.JudgmentPack) [][]string {
	out := make([][]string, len(packs))
	for i, p := range packs {
		out[i] = append([]string(nil), p.FindingRefs...)
	}
	return out
}

func candidate(ref, path string, startRow, evidenceChars int, payload string) rubrics.PackCandidate {
	return rubrics.PackCandidate{
		FindingRef:    ref,
		Path:          path,
		StartRow:      startRow,
		PayloadJSON:   []byte(payload),
		EvidenceChars: evidenceChars,
	}
}

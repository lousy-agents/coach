package rubrics_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/rubrics"
)

// packRefs flattens pack finding refs for stable boundary comparisons.
func packRefs(packs []rubrics.JudgmentPack) [][]string {
	out := make([][]string, len(packs))
	for i, p := range packs {
		out[i] = append([]string(nil), p.FindingRefs...)
	}
	return out
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

func refPathIndex(cands []rubrics.PackCandidate) map[string]string {
	m := make(map[string]string, len(cands))
	for _, c := range cands {
		m[c.FindingRef] = c.Path
	}
	return m
}

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
				// hot: 6 (≥ affinity default 5), cold: 3 + 3 → 12 findings across 3 paths
				cands = nil
				cands = append(cands, buildPathCandidates("pkg/hot/service.go", 6)...)
				cands = append(cands, buildPathCandidates("pkg/cold/a.go", 3)...)
				cands = append(cands, buildPathCandidates("pkg/cold/b.go", 3)...)
				// Shuffle input order so sort, not input order, drives packing.
				cands = []rubrics.PackCandidate{
					cands[8], cands[1], cands[11], cands[0], cands[5], cands[9],
					cands[2], cands[7], cands[3], cands[10], cands[4], cands[6],
				}
				byPath = refPathIndex(cands)
			})

			It("emits a stable pack count and never merges the hot path with other paths", func() {
				packs := rubrics.PackJudgmentCandidates(cands, cfg)
				Expect(packs).NotTo(BeEmpty())

				// max 4/pack: hot 6 → 2 packs; cold 6 mergeable → at least 2 packs under max 4
				// → pack count is deterministic and strictly less than finding count
				Expect(len(packs)).To(BeNumerically("<", len(cands)))
				Expect(len(packs)).To(Equal(4)) // 4+2 hot + 4+2 cold merge

				hotPath := "pkg/hot/service.go"
				for _, p := range packs {
					paths := pathsInPack(p.FindingRefs, byPath)
					hasHot := false
					hasOther := false
					for _, path := range paths {
						if path == hotPath {
							hasHot = true
						} else {
							hasOther = true
						}
					}
					Expect(hasHot && hasOther).To(BeFalse(),
						"hot path must not share a pack with other paths; pack=%v paths=%v",
						p.FindingRefs, paths)
				}

				// Every finding appears exactly once.
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

				Expect(packs).To(HaveLen(3)) // 4+4+2
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
			// Shared fixture math (chars/4 estimator, packPromptOverheadTokens=64):
			// 200-byte payload → (200+3)/4 = 50 tokens per candidate.
			// one item: 64+50=114; two: 164; three: 214.
			// Budget 150: one fits, two do not → three solo packs.
			// Budget 200: two fit (164≤200), third does not → [[#1,#2],[#3]].
			// A wrong chars/8 estimator (25 tok/item) packs all three under both
			// budgets (64+75=139), so these cases lock chars/4.
			bigPayload := func() []byte {
				p := make([]byte, 200)
				for i := range p {
					p[i] = 'x'
				}
				return p
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
			})

			It("packs more than one finding under the token budget when two fit and three do not", func() {
				// max-findings=4 so only the token cap forces the split after #2.
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
				// 2+2 findings, affinity min 5 → merge into one pack of 4
				cands := append(buildPathCandidates("pkg/a.go", 2), buildPathCandidates("pkg/b.go", 2)...)
				// Reverse order to prove sort: path a before b, start_row ascending
				cands = []rubrics.PackCandidate{cands[3], cands[1], cands[2], cands[0]}

				packs := rubrics.PackJudgmentCandidates(cands, cfg)
				Expect(packs).To(HaveLen(1))
				Expect(packs[0].FindingRefs).To(Equal([]string{
					"pkg/a.go#1", "pkg/a.go#2", "pkg/b.go#1", "pkg/b.go#2",
				}))
			})

			It("does not exceed max findings when cross-file merging", func() {
				// 3+3 under affinity; max 4 → two packs, second may still be multi-path
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
				// a.go row10: a#1a before a#1b; then a.go row20; then b.go — max 4 splits b#2
				Expect(packs).To(HaveLen(2))
				Expect(packs[0].FindingRefs).To(Equal([]string{
					"a#1a", "a#1b", "a#2", "b#1",
				}))
				Expect(packs[1].FindingRefs).To(Equal([]string{"b#2"}))
			})
		})
	})
})

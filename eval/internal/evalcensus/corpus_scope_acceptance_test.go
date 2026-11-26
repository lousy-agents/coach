package evalcensus_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

// scopedCorpus is validCorpus with the licence and sample every census repository now needs.
func scopedCorpus() map[string]any {
	corpus := validCorpus()
	for _, entry := range corpus["repositories"].([]any) {
		repo := entry.(map[string]any)
		repo["licence"] = "MIT"
		repo["sample"] = map[string]any{"seed": 7}
	}
	return corpus
}

func firstRepository(manifest map[string]any) map[string]any {
	return manifest["repositories"].([]any)[0].(map[string]any)
}

var _ = Describe("the corpus manifest's scope", func() {
	When("a repository records no licence", func() {
		It("refuses the manifest, naming the repository and the licence", func() {
			manifest := scopedCorpus()
			delete(firstRepository(manifest), "licence")

			_, err := evalcensus.LoadCensusCorpus(writeJSON(manifest))

			Expect(err).To(MatchError(ContainSubstring(`"coach"`)))
			Expect(err).To(MatchError(ContainSubstring("licence")))
		})
	})

	When("a repository's licence is only whitespace", func() {
		It("refuses the manifest, naming the repository", func() {
			manifest := scopedCorpus()
			firstRepository(manifest)["licence"] = " \t "

			_, err := evalcensus.LoadCensusCorpus(writeJSON(manifest))

			Expect(err).To(MatchError(ContainSubstring(`"coach"`)))
		})
	})

	When("a repository records a licence", func() {
		It("loads the manifest", func() {
			_, err := evalcensus.LoadCensusCorpus(writeJSON(scopedCorpus()))

			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("a repository has no sample", func() {
		It("refuses the manifest, naming the repository and the seed", func() {
			manifest := scopedCorpus()
			delete(firstRepository(manifest), "sample")

			_, err := evalcensus.LoadCensusCorpus(writeJSON(manifest))

			Expect(err).To(MatchError(ContainSubstring(`"coach"`)))
			Expect(err).To(MatchError(ContainSubstring("seed")))
		})
	})

	When("a repository's sample has no seed", func() {
		It("refuses the manifest, naming the repository and the seed", func() {
			manifest := scopedCorpus()
			firstRepository(manifest)["sample"] = map[string]any{"size": 30}

			_, err := evalcensus.LoadCensusCorpus(writeJSON(manifest))

			Expect(err).To(MatchError(ContainSubstring(`"coach"`)))
			Expect(err).To(MatchError(ContainSubstring("seed")))
		})
	})

	When("a repository's sample names a seed and no size", func() {
		It("loads the manifest", func() {
			manifest := scopedCorpus()
			firstRepository(manifest)["sample"] = map[string]any{"seed": 7}

			_, err := evalcensus.LoadCensusCorpus(writeJSON(manifest))

			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("a repository's include glob is malformed", func() {
		It("refuses the manifest, naming the repository", func() {
			manifest := scopedCorpus()
			firstRepository(manifest)["include"] = []any{"src/["}

			_, err := evalcensus.LoadCorpus(writeJSON(manifest))

			Expect(err).To(MatchError(ContainSubstring(`"coach"`)))
		})
	})

	When("a repository's exclude glob is malformed", func() {
		It("refuses the manifest, naming the repository", func() {
			manifest := scopedCorpus()
			firstRepository(manifest)["exclude"] = []any{"gen/**/["}

			_, err := evalcensus.LoadCorpus(writeJSON(manifest))

			Expect(err).To(MatchError(ContainSubstring(`"coach"`)))
		})
	})

	When("a repository's include and exclude globs are well formed", func() {
		It("loads the manifest", func() {
			manifest := scopedCorpus()
			firstRepository(manifest)["include"] = []any{"src/**"}
			firstRepository(manifest)["exclude"] = []any{"src/gen/**"}

			_, err := evalcensus.LoadCorpus(writeJSON(manifest))

			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("a manifest for eval-history names no licence and no sample", func() {
		It("is still accepted by the shared loader, so the census requirements stay with the census", func() {
			_, err := evalcensus.LoadCorpus(writeJSON(validCorpus()))

			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("a repository's sample names a size", func() {
		It("refuses the manifest by name, since the size is fixed at 30 and nothing honours a size key", func() {
			manifest := scopedCorpus()
			firstRepository(manifest)["sample"] = map[string]any{"seed": 7, "size": 30}

			_, err := evalcensus.LoadCensusCorpus(writeJSON(manifest))

			Expect(err).To(MatchError(ContainSubstring("size")))
		})
	})

	DescribeTable("a repository's include or exclude glob is not repository-relative",
		func(pattern string) {
			manifest := scopedCorpus()
			firstRepository(manifest)["include"] = []any{pattern}

			_, err := evalcensus.LoadCorpus(writeJSON(manifest))

			Expect(err).To(MatchError(ContainSubstring(`"coach"`)))
		},
		Entry("starts with ./", "./src/**"),
		Entry("starts with /", "/src/**"),
		Entry("climbs out with ../", "../src/**"),
	)
})

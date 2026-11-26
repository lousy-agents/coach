package antipatternfixtures_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgen/antipatternfixtures"
)

var _ = Describe("deriving the layer fixtures from a profile's layers (#900, #872 AC-4.1)", func() {
	var out string
	BeforeEach(func() { out = filepath.Join(GinkgoT().TempDir(), "secret-label") })

	When("the profile records an ordered layer count of 3 to 5 with the packages of its roles", func() {
		It("emits both layer fixtures, in #871's layout, with the policy stored as coach.json.txt (AC-4.2)", func() {
			result, err := generate(layered(map[string]int{"4": 1}, evenly(4, map[string]int{"2": 1})), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(ids(out)).To(Equal(layerIDs))
			Expect(result.Emitted).To(ConsistOf(
				antipatternfixtures.Emitted{ID: "dependency-rule-breach", Value: 4},
				antipatternfixtures.Emitted{ID: "skipped-layer", Value: 4}))
			for _, id := range layerIDs {
				Expect(tree(filepath.Join(out, id))).To(SatisfyAll(
					HaveKey("expect.json"), HaveKey("bad/go.mod.txt"), HaveKey("good/go.mod.txt"),
					HaveKey("bad/.lousy-agents/coach.json.txt"), HaveKey("good/.lousy-agents/coach.json.txt")))
			}
		})

		DescribeTable("scales to the most frequent layer count from 3 to 5, ties to the larger, and ignores a count outside the range",
			func(layers map[string]int, want int) {
				_, err := generate(layered(layers, evenly(want, map[string]int{"1": 1})), out)

				Expect(err).NotTo(HaveOccurred())
				expectLayerCount(out, want)
			},
			Entry("the most frequent", map[string]int{"3": 1, "4": 5, "5": 2}, 4),
			Entry("a tie goes to the larger", map[string]int{"3": 2, "5": 2}, 5),
			Entry("a count of 3, the lowest", map[string]int{"3": 1}, 3),
			Entry("a count over 5 is ignored, though it is the most frequent", map[string]int{"4": 1, "9": 7}, 4),
			Entry("a count under 3 is ignored, though it is the most frequent", map[string]int{"3": 1, "2": 7}, 3),
		)

		DescribeTable("holds each layer to the most frequent package count the roles observed, ties to the larger, capped at 3",
			func(packages map[string]int, want, from int) {
				_, err := generate(layered(map[string]int{"4": 1}, evenly(4, packages)), out)

				Expect(err).NotTo(HaveOccurred())
				expectPackagesPerLayer(out, 4, want, from)
			},
			Entry("the most frequent", map[string]int{"1": 1, "2": 4, "3": 2}, 2, 2),
			Entry("a tie goes to the larger", map[string]int{"1": 3, "2": 3}, 2, 2),
			Entry("a count over the cap is capped, and the note says from what", map[string]int{"9": 2, "1": 1}, 3, 9),
		)

		It("counts the packages of every role of the layer count together, and ignores a role the layer count does not use", func() {
			roles := map[string]map[string]int{"api": {"2": 1}, "service": {"2": 1}, "store": {"3": 1}, "repository": {"1": 9}}

			_, err := generate(layered(map[string]int{"3": 1}, roles), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(packageDirs(out, "skipped-layer", "bad")["api"]).To(HaveLen(2), "2 is the most frequent of the three roles of 3 layers; repository is not one of them")
		})

		DescribeTable("counts the roles the layer count adds beyond the first three",
			func(layers int, roles map[string]map[string]int, want int) {
				_, err := generate(layered(map[string]int{itoa(layers): 1}, roles), out)

				Expect(err).NotTo(HaveOccurred())
				Expect(packageDirs(out, "skipped-layer", "bad")["api"]).To(HaveLen(want))
			},
			Entry("domain, the fourth role of 4 layers", 4,
				map[string]map[string]int{"api": {"1": 1}, "service": {"1": 1}, "store": {"1": 1}, "domain": {"3": 5}}, 3),
			Entry("repository, the fifth role of 5 layers", 5,
				map[string]map[string]int{"api": {"1": 1}, "service": {"1": 1}, "domain": {"1": 1}, "store": {"1": 1}, "repository": {"2": 5}}, 2),
		)

		It("emits neither layer fixture when the profile records no packages for the roles, which is what a profile without an ordered policy records", func() {
			result, err := generate(layered(map[string]int{"4": 1}, map[string]map[string]int{}), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Emitted).To(BeEmpty())
			Expect(out).NotTo(BeADirectory())
		})

		It("emits neither layer fixture when the only packages recorded are a role the layer count does not use", func() {
			result, err := generate(layered(map[string]int{"3": 1}, map[string]map[string]int{"repository": {"2": 1}}), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Emitted).To(BeEmpty())
		})

		It("leaves the other fixtures as they were when it emits neither layer fixture for want of roles", func() {
			base := profileWith(map[string]int{"4": 1}, map[string]int{}, map[string]int{}, 0)

			result, err := generate(layeredOn(base, map[string]int{"4": 1}, map[string]map[string]int{}), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Emitted).To(ConsistOf(antipatternfixtures.Emitted{ID: "arrow-code", Value: 4}))
		})

		It("emits both when a single role of the layer count has an observed package count", func() {
			result, err := generate(layered(map[string]int{"3": 1}, map[string]map[string]int{"service": {"2": 1}}), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Emitted).To(HaveLen(2))
			Expect(packageDirs(out, "skipped-layer", "bad")["api"]).To(HaveLen(2))
		})

		DescribeTable("orders the layers by forbidden pairs: every inner layer is forbidden to import every outer one, once",
			func(layers int) {
				_, err := generate(layered(map[string]int{itoa(layers): 1}, evenly(layers, map[string]int{"1": 1})), out)

				Expect(err).NotTo(HaveOccurred())
				expectOrderedByForbiddenPairs(out, layers)
			},
			Entry("3 layers", 3), Entry("4 layers", 4), Entry("5 layers", 5),
		)

		It("requires the second layer in the skipped-layer policy only, the same in both variants", func() {
			_, err := generate(layered(map[string]int{"5": 1}, evenly(5, map[string]int{"1": 1})), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(policyOf(out, "skipped-layer", "bad").RequiredLayer).To(Equal("service"))
			Expect(policyOf(out, "skipped-layer", "good").RequiredLayer).To(Equal("service"))
			Expect(policyOf(out, "dependency-rule-breach", "bad").RequiredLayer).To(BeEmpty())
			Expect(policyOf(out, "dependency-rule-breach", "good").RequiredLayer).To(BeEmpty())
		})

		DescribeTable("declares each expected finding from the template, whatever the layer count and package count",
			func(layers, packages int) {
				_, err := generate(layered(map[string]int{itoa(layers): 1}, evenly(layers, map[string]int{itoa(packages): 1})), out)
				Expect(err).NotTo(HaveOccurred())

				expectDeclaredFindings(out)
			},
			Entry("3 layers, 1 package", 3, 1), Entry("4 layers, 2 packages", 4, 2), Entry("5 layers, 3 packages", 5, 3),
		)

		It("builds a bad variant that differs from its good variant in the breach or the bypass alone", func() {
			_, err := generate(layered(map[string]int{"4": 1}, evenly(4, map[string]int{"2": 1})), out)
			Expect(err).NotTo(HaveOccurred())

			bad, good := tree(filepath.Join(out, "dependency-rule-breach", "bad")), tree(filepath.Join(out, "dependency-rule-breach", "good"))
			Expect(bad).To(HaveKey("internal/store/orders/reach.go.txt"))
			Expect(good).NotTo(HaveKey("internal/store/orders/reach.go.txt"))
			delete(bad, "internal/store/orders/reach.go.txt")
			Expect(bad).To(Equal(good))
			Expect(tree(filepath.Join(out, "skipped-layer", "bad"))["internal/api/orders/handle.go.txt"]).
				NotTo(Equal(tree(filepath.Join(out, "skipped-layer", "good"))["internal/api/orders/handle.go.txt"]))
		})

		It("writes nothing of the reference, and the same bytes from the same profile twice (AC-4.3a, AC-4.6)", func() {
			again := filepath.Join(GinkgoT().TempDir(), "again")
			raw := layered(map[string]int{"5": 1}, evenly(5, map[string]int{"3": 1}))
			_, err := generate(raw, out)
			Expect(err).NotTo(HaveOccurred())
			_, err = generate(raw, again)
			Expect(err).NotTo(HaveOccurred())

			Expect(tree(again)).To(Equal(tree(out)))
			Expect(allFiles(out)).To(HaveEach(Not(ContainSubstring("secret-label"))))
			Expect(allContents(out)).To(HaveEach(Not(ContainSubstring("secret-label"))))
		})
	})

	When("the profile records no layer the generator can scale", func() {
		DescribeTable("emits neither layer fixture",
			func(raw []byte) {
				result, err := generate(raw, out)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Emitted).To(BeEmpty())
				Expect(out).NotTo(BeADirectory())
			},
			Entry("no layer count at all", layered(map[string]int{}, evenly(4, map[string]int{"2": 1}))),
			Entry("only a layer count over 5", layered(map[string]int{"9": 1}, evenly(4, map[string]int{"2": 1}))),
			Entry("only a layer count under 3", layered(map[string]int{"2": 3}, evenly(3, map[string]int{"2": 1}))),
			Entry("a layer count whose bucket counts nothing", layered(map[string]int{"4": 0}, evenly(4, map[string]int{"2": 1}))),
			Entry("the shape coach-self records: 9 layers, no roles, layers and packages listed as not covered",
				layered(map[string]int{"9": 1}, map[string]map[string]int{}, "layers", "packages")),
		)

		It("emits neither when not_covered lists layers, though a layer count in range was also recorded", func() {
			result, err := generate(layered(map[string]int{"4": 3, "9": 1}, evenly(4, map[string]int{"2": 1}), "layers"), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Emitted).To(BeEmpty())
		})

		It("still emits both when not_covered lists only other shapes", func() {
			result, err := generate(layered(map[string]int{"4": 1}, evenly(4, map[string]int{"2": 1}), "packages", "depth", "nesting"), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Emitted).To(HaveLen(2))
		})
	})
})

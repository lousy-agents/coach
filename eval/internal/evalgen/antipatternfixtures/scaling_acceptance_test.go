package antipatternfixtures_test

import (
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("scaling a derived fixture to the profile (#872 Task 5, AC-4.1, AC-4.3)", func() {
	var out string
	BeforeEach(func() { out = filepath.Join(GinkgoT().TempDir(), "secret-label") })

	DescribeTable("scales the bad variant to the most frequent observed value at or over Coach's threshold, ties to the larger, capped, and says so in the note",
		func(nesting map[string]int, value, from int) {
			_, err := generate(profileWith(nesting, map[string]int{}, map[string]int{}, 0), out)
			Expect(err).NotTo(HaveOccurred())

			Expect(ids(out)).To(Equal([]string{"arrow-code"}))
			note := readExpect(out, "arrow-code").Note
			Expect(note).To(ContainSubstring("nesting depth " + itoa(value)))
			if from != value {
				Expect(note).To(ContainSubstring("capped at " + itoa(value) + " from " + itoa(from)))
			} else {
				Expect(note).NotTo(ContainSubstring("capped"))
			}
			Expect(tree(filepath.Join(out, "arrow-code"))["bad/ledger/apply.go.txt"]).To(ContainSubstring(strings.Repeat("\t", value+1) + "total += value"))
		},
		Entry("the most frequent value", map[string]int{"2": 50, "4": 2, "6": 3, "7": 1}, 6, 6),
		Entry("a tie goes to the larger value", map[string]int{"4": 2, "5": 2}, 5, 5),
		Entry("a value over the cap is capped, and the note says from what", map[string]int{"12": 4}, 8, 12),
	)

	DescribeTable("declares every rule the scaled bad variant trips, from the template's own arithmetic",
		func(depth int, cognitive bool) {
			_, err := generate(profileWith(map[string]int{itoa(depth): 1}, map[string]int{}, map[string]int{}, 0), out)
			Expect(err).NotTo(HaveOccurred())

			rules := []finding{{RuleID: "complexity.max_nesting_depth", Path: "ledger/apply.go", Subject: ""}}
			if cognitive {
				rules = append(rules, finding{RuleID: "complexity.cognitive_complexity", Path: "ledger/apply.go", Subject: "Balance"})
			}
			Expect(badFindings(readExpect(out, "arrow-code"))).To(Equal(rules))
		},
		Entry("depth 4: cognitive complexity 10, under Coach's 15", 4, false),
		Entry("depth 5: cognitive complexity 15, which reaches it", 5, true),
	)

	DescribeTable("emits a fixture for a value exactly at Coach's threshold, in the profile's units",
		func(nesting, cognitive, density map[string]int, id string) {
			_, err := generate(profileWith(nesting, cognitive, density, 0), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(ids(out)).To(Equal([]string{id}))
		},
		Entry("nesting 3", map[string]int{"3": 1}, map[string]int{}, map[string]int{}, "arrow-code"),
		Entry("cognitive complexity 15", map[string]int{}, map[string]int{"15": 1}, map[string]int{}, "spaghetti-conditional"),
		Entry("12 branch points", map[string]int{}, map[string]int{}, map[string]int{"12": 1}, "god-file"),
	)

	It("writes a god file with exactly the observed branch points, the last check holding what is left over three", func() {
		_, err := generate(profileWith(map[string]int{}, map[string]int{}, map[string]int{"13": 1}, 0), out)
		Expect(err).NotTo(HaveOccurred())

		files := tree(filepath.Join(out, "god-file"))
		Expect(strings.Count(files["bad/orders/orders.go.txt"], "\tif ")).To(Equal(13))
		Expect(strings.Count(files["good/orders/check5.go.txt"], "\tif ")).To(Equal(1))
	})
})

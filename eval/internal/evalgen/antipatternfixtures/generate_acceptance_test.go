package antipatternfixtures_test

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgen/antipatternfixtures"
)

func generate(raw []byte, out string) (antipatternfixtures.Result, error) {
	return antipatternfixtures.Generate(raw, acceptAll, out, antipatternfixtures.Templates(), antipatternfixtures.GoVet)
}

var _ = Describe("profile-derived anti-pattern fixtures (#872 Task 5)", func() {
	var out string
	BeforeEach(func() { out = filepath.Join(GinkgoT().TempDir(), "secret-label") })

	When("the profile observed every shape a template reads", func() {
		It("emits one fixture per observed #871 pattern, in #871's layout, with the sources stored as .txt (AC-4.1, AC-4.2)", func() {
			result, err := generate(everyShape(), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(ids(out)).To(Equal([]string{"arrow-code", "constructor-does-real-work", "god-file", "spaghetti-conditional"}))
			Expect(result.Emitted).To(HaveLen(4))
			for _, id := range ids(out) {
				files := tree(filepath.Join(out, id))
				Expect(files).To(HaveKey("expect.json"))
				Expect(files).To(HaveKey("bad/go.mod.txt"))
				Expect(files).To(HaveKey("good/go.mod.txt"))
				for name := range files {
					Expect(name == "expect.json" || strings.HasSuffix(name, ".txt")).To(BeTrue(), name)
				}
			}
		})

		It("names the #871 entry, records the profile in seed, and keeps #871's expect.json fields (AC-4.3a, AC-4.7)", func() {
			_, err := generate(everyShape(), out)
			Expect(err).NotTo(HaveOccurred())

			for _, id := range ids(out) {
				expect := readExpect(out, id)
				Expect(expect.ID).To(Equal(id))
				Expect(expect.Seed).To(Equal("profile:" + profileIdentity))
				Expect(expect.Source).To(ContainSubstring("#871"))
				Expect(expect.Source).To(ContainSubstring(id))
			}
		})

		It("takes each expectation from the template's declaration, and writes a pattern with no rule as not_covered (AC-4.3, AC-4.4)", func() {
			_, err := generate(everyShape(), out)
			Expect(err).NotTo(HaveOccurred())

			Expect(badFindings(readExpect(out, "arrow-code"))).To(Equal([]finding{{RuleID: "complexity.max_nesting_depth", Path: "ledger/apply.go", Subject: ""}}))
			Expect(badFindings(readExpect(out, "spaghetti-conditional"))).To(Equal([]finding{{RuleID: "complexity.cognitive_complexity", Path: "archive/select.go", Subject: "Expired"}}))
			Expect(badFindings(readExpect(out, "god-file"))).To(Equal([]finding{{RuleID: "complexity.branch_density", Path: "orders/orders.go", Subject: ""}}))
			constructor := readExpect(out, "constructor-does-real-work")
			Expect(constructor.Status).To(Equal("not_covered"))
			Expect(string(constructor.Expect.Bad)).To(Equal(`"none"`))
			for _, id := range []string{"arrow-code", "spaghetti-conditional", "god-file"} {
				Expect(readExpect(out, id).Status).To(Equal("covered"))
				Expect(string(readExpect(out, id).Expect.Good)).To(Equal(`"none"`))
			}
		})

		It("writes nothing of the reference: not its label, only the template's own names (AC-4.3a)", func() {
			_, err := generate(everyShape(), out)
			Expect(err).NotTo(HaveOccurred())

			for _, id := range ids(out) {
				for name, content := range tree(filepath.Join(out, id)) {
					Expect(name).NotTo(ContainSubstring("secret-label"))
					Expect(content).NotTo(ContainSubstring("secret-label"))
				}
			}
		})
	})

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

	When("the profile observed no shape a template reads", func() {
		It("emits no fixture and says so in the result", func() {
			result, err := generate(profileWith(map[string]int{"2": 9}, map[string]int{"14": 1}, map[string]int{"11": 1}, 0), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Emitted).To(BeEmpty())
			Expect(out).NotTo(BeADirectory())
		})
	})

	When("the output directory holds fixtures of an earlier run", func() {
		It("replaces them, so a shape no longer observed leaves no fixture behind", func() {
			_, err := generate(everyShape(), out)
			Expect(err).NotTo(HaveOccurred())

			_, err = generate(profileWith(map[string]int{"4": 1}, map[string]int{}, map[string]int{}, 0), out)

			Expect(err).NotTo(HaveOccurred())
			Expect(ids(out)).To(Equal([]string{"arrow-code"}))
		})
	})

	It("writes byte-identical fixtures from the same profile twice (AC-4.6)", func() {
		again := filepath.Join(GinkgoT().TempDir(), "again")
		_, err := generate(everyShape(), out)
		Expect(err).NotTo(HaveOccurred())
		_, err = generate(everyShape(), again)
		Expect(err).NotTo(HaveOccurred())

		Expect(tree(again)).To(Equal(tree(out)))
	})

	DescribeTable("refuses a profile it cannot read with exit 2, writing nothing",
		func(raw []byte, validate antipatternfixtures.Validator, reason string) {
			_, err := antipatternfixtures.Generate(raw, validate, out, antipatternfixtures.Templates(), antipatternfixtures.GoVet)

			Expect(exitOf(err)).To(Equal(2))
			Expect(err.Error()).To(ContainSubstring(reason))
			Expect(out).NotTo(BeADirectory())
		},
		Entry("one the schema validator rejects", everyShape(), func([]byte) error { return errors.New("closed schema: unexpected field") },
			"closed schema: unexpected field"),
		Entry("one from another extractor version", []byte(strings.Replace(string(everyShape()), `"0.2.0"`, `"0.1.0"`, 1)), antipatternfixtures.Validator(acceptAll),
			`extractor_version "0.1.0"`),
		Entry("one of another schema version", []byte(strings.Replace(string(everyShape()), `"schema_version":"1"`, `"schema_version":"2"`, 1)), antipatternfixtures.Validator(acceptAll),
			`schema_version "2"`),
	)

	When("a variant does not pass go vet", func() {
		It("leaves that fixture out, writes the others, and exits 3 naming it (AC-4.5)", func() {
			broken := antipatternfixtures.Templates()[0]
			broken.ID = "broken-template"
			good := broken.Good
			broken.Good = func(value int) map[string]string {
				files := good(value)
				for name := range files {
					if strings.HasSuffix(name, ".go") {
						files[name] += "\nfunc unused() { x := 1 }\n"
					}
				}
				return files
			}
			templates := append(antipatternfixtures.Templates(), broken)

			_, err := antipatternfixtures.Generate(everyShape(), acceptAll, out, templates, antipatternfixtures.GoVet)

			Expect(exitOf(err)).To(Equal(3))
			Expect(err.Error()).To(ContainSubstring("broken-template"))
			Expect(filepath.Join(out, "broken-template")).NotTo(BeADirectory())
			Expect(ids(out)).To(ContainElement("arrow-code"))
		})
	})
})

func itoa(n int) string { return strconv.Itoa(n) }

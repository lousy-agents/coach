package evalhistory_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
	"github.com/lousy-agents/coach/eval/internal/evalhistory"
)

var _ = Describe("the unit inventory at since", func() {
	When("the tree holds Go files the census admits (test files, testdata and hidden code), vendored code, and other files", func() {
		It("lists each Go file the census admits, with its functions and lines, and leaves out vendored and other files", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{
				"p/p.go":          baseSource,
				"p/p_test.go":     "package p\n\nfunc TestA() {}\n",
				"vendor/v/v.go":   "package v\n\nfunc V() {}\n",
				"p/testdata/t.go": "package t\n\nfunc T() {}\n",
				".hidden/h.go":    "package h\n\nfunc H() {}\n",
				"README.md":       "# readme\n",
				"q/q.go":          "package q\n\ntype T struct{}\n\nfunc (T) M() {}\n",
			})
			h.commit("later", map[string]string{"p/p.go": baseSource + "\nfunc Later() {}\n"})

			inventory, err := evalhistory.GoInventory(h.dir, since, evalcensus.Repository{})
			Expect(err).NotTo(HaveOccurred())
			Expect(inventory.Functions).To(ConsistOf(
				evalhistory.Unit{Path: ".hidden/h.go", Start: 3, End: 3},
				evalhistory.Unit{Path: "p/p.go", Start: 3, End: 5},
				evalhistory.Unit{Path: "p/p.go", Start: 7, End: 10},
				evalhistory.Unit{Path: "p/p_test.go", Start: 3, End: 3},
				evalhistory.Unit{Path: "p/testdata/t.go", Start: 3, End: 3},
				evalhistory.Unit{Path: "q/q.go", Start: 5, End: 5},
			))
			Expect(inventory.Files).To(ConsistOf(
				evalhistory.Unit{Path: ".hidden/h.go", Start: 1, End: 3},
				evalhistory.Unit{Path: "p/p.go", Start: 1, End: 10},
				evalhistory.Unit{Path: "p/p_test.go", Start: 1, End: 3},
				evalhistory.Unit{Path: "p/testdata/t.go", Start: 1, End: 3},
				evalhistory.Unit{Path: "q/q.go", Start: 1, End: 5},
			))
		})
	})

	When("the census counts findings in a test file", func() {
		It("lists that test file's functions, so the history's controls and flagged units come from the same code", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{
				"p/p.go":      baseSource,
				"p/p_test.go": "package p\n\nfunc TestA() {}\n",
			})

			inventory, err := evalhistory.GoInventory(h.dir, since, evalcensus.Repository{})
			Expect(err).NotTo(HaveOccurred())
			Expect(inventory.Functions).To(ContainElement(evalhistory.Unit{Path: "p/p_test.go", Start: 3, End: 3}))
			Expect(inventory.Files).To(ContainElement(evalhistory.Unit{Path: "p/p_test.go", Start: 1, End: 3}))
		})
	})

	When("the repository names include and exclude globs", func() {
		It("draws no control from eval/, a vendored directory, or an excluded glob", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{
				"q/q.go":        "package q\n\nfunc Q() {}\n",
				"eval/e/e.go":   "package e\n\nfunc E() {}\n",
				"gen/g/g.go":    "package g\n\nfunc G() {}\n",
				"vendor/v/v.go": "package v\n\nfunc V() {}\n",
			})
			repo := evalcensus.Repository{Exclude: []string{"gen/**"}}

			inventory, err := evalhistory.GoInventory(h.dir, since, repo)
			Expect(err).NotTo(HaveOccurred())
			Expect(inventory.Functions).To(ConsistOf(evalhistory.Unit{Path: "q/q.go", Start: 3, End: 3}))
			Expect(inventory.Files).To(ConsistOf(evalhistory.Unit{Path: "q/q.go", Start: 1, End: 3}))
		})

		It("lists exactly the paths the census admits, through the census's own scope", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{
				"q/q.go":      "package q\n\nfunc Q() {}\n",
				"q/skip/s.go": "package skip\n\nfunc S() {}\n",
				"r/r.go":      "package r\n\nfunc R() {}\n",
			})
			repo := evalcensus.Repository{Include: []string{"q/**"}, Exclude: []string{"q/skip/**"}}

			inventory, err := evalhistory.GoInventory(h.dir, since, repo)
			Expect(err).NotTo(HaveOccurred())
			Expect(inventory.Files).To(ConsistOf(evalhistory.Unit{Path: "q/q.go", Start: 1, End: 3}))
			Expect(repo.Admits("q/q.go")).To(BeTrue())
			Expect(repo.Admits("q/skip/s.go")).To(BeFalse())
		})
	})

	When("two files' functions interleave by line number", func() {
		It("orders units by path, then by first line, so a seeded draw sees the same order", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{
				"a.go": "package p\n\nfunc A1() {}\n\n\n\n\n\n\nfunc A2() {}\n",
				"b.go": "package p\n\n\n\nfunc B1() {}\n",
			})

			inventory, err := evalhistory.GoInventory(h.dir, since, evalcensus.Repository{})
			Expect(err).NotTo(HaveOccurred())
			Expect(inventory.Functions).To(Equal([]evalhistory.Unit{
				{Path: "a.go", Start: 3, End: 3},
				{Path: "a.go", Start: 10, End: 10},
				{Path: "b.go", Start: 5, End: 5},
			}))
		})
	})

	When("a Go file at since does not parse", func() {
		It("refuses, naming the file", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{"p/p.go": baseSource, "p/broken.go": "package p\n\nfunc (\n"})

			_, err := evalhistory.GoInventory(h.dir, since, evalcensus.Repository{})
			Expect(err).To(MatchError(ContainSubstring("p/broken.go")))
		})
	})
})

var _ = Describe("mapping Coach's findings to units", func() {
	inventory := evalhistory.Inventory{
		Functions: []evalhistory.Unit{{Path: "p/p.go", Start: 3, End: 5}, {Path: "p/p.go", Start: 7, End: 10}, {Path: "q/q.go", Start: 3, End: 9}},
		Files:     []evalhistory.Unit{{Path: "p/p.go", Start: 1, End: 10}, {Path: "q/q.go", Start: 1, End: 9}},
	}

	It("places a function finding in the function holding its first line, and a file finding in its file", func() {
		signals := []evalhistory.Signal{
			{RuleID: "complexity.cognitive_complexity", Path: "p/p.go", Subject: "B", StartRow: 6, EndRow: 9},
			{RuleID: "complexity.branch_density", Path: "q/q.go", StartRow: 0, EndRow: 0},
			{RuleID: "complexity.cognitive_complexity", Path: "q/q.go", Subject: "Q", StartRow: 2, EndRow: 8},
		}

		cognitive := flaggedUnits(inventory, signals, "complexity.cognitive_complexity")
		density := flaggedUnits(inventory, signals, "complexity.branch_density")

		Expect(cognitive.Level).To(Equal(evalhistory.FunctionLevel))
		Expect(cognitive.Units).To(Equal([]int{1, 2}))
		Expect(density.Level).To(Equal(evalhistory.FileLevel))
		Expect(density.Units).To(Equal([]int{1}))
		Expect(cognitive.Unplaced).To(BeZero())
	})

	It("counts a finding that falls in no unit as unplaced", func() {
		signals := []evalhistory.Signal{{RuleID: "complexity.cognitive_complexity", Path: "gone.go", Subject: "G", StartRow: 2, EndRow: 4}}

		flagged := flaggedUnits(inventory, signals, "complexity.cognitive_complexity")
		Expect(flagged.Units).To(BeEmpty())
		Expect(flagged.Unplaced).To(Equal(1))
	})

	It("names the reason for each finding it cannot place", func() {
		inTypeScript, err := evalhistory.FlaggedUnits(inventory, []evalhistory.Signal{{RuleID: "complexity.branch_density", Path: "js/a.ts"}}, "complexity.branch_density")
		Expect(err).NotTo(HaveOccurred())
		Expect(inTypeScript.UnplacedReasons).To(Equal([]string{"language not in the Go history inventory"}))

		missing, err := evalhistory.FlaggedUnits(inventory, []evalhistory.Signal{{RuleID: "complexity.cognitive_complexity", Path: "gone.go", Subject: "G", StartRow: 2, EndRow: 4}}, "complexity.cognitive_complexity")
		Expect(err).NotTo(HaveOccurred())
		Expect(missing.UnplacedReasons).To(Equal([]string{"no unit of the Go history inventory holds its line"}))
	})

	It("places a max_nesting_depth finding in its file, whatever its location", func() {
		signals := []evalhistory.Signal{{RuleID: "complexity.max_nesting_depth", Path: "p/p.go", StartRow: 6, EndRow: 9}}

		flagged := flaggedUnits(inventory, signals, "complexity.max_nesting_depth")
		Expect(flagged.Level).To(Equal(evalhistory.FileLevel))
		Expect(flagged.Units).To(Equal([]int{0}))
	})

	It("places an architecture.layer_violation finding on an import line in its file unit", func() {
		signals := []evalhistory.Signal{{RuleID: "architecture.layer_violation", Path: "p/p.go", Subject: "architecture.layer_violation:p->q", StartRow: 1}}

		flagged := flaggedUnits(inventory, signals, "architecture.layer_violation")
		Expect(flagged.Level).To(Equal(evalhistory.FileLevel))
		Expect(flagged.Units).To(Equal([]int{0}))
		Expect(flagged.Unplaced).To(BeZero())
	})

	It("places a coupling.deep_relative_import finding in its file, as an import edge", func() {
		signals := []evalhistory.Signal{{RuleID: "coupling.deep_relative_import", Path: "p/p.go", Subject: "../x", StartRow: 1}}

		flagged := flaggedUnits(inventory, signals, "coupling.deep_relative_import")
		Expect(flagged.Level).To(Equal(evalhistory.FileLevel))
		Expect(flagged.Units).To(Equal([]int{0}))
		Expect(flagged.Unplaced).To(BeZero())
	})

	It("refuses a rule the table does not hold when the comparison runs directly, naming the rule", func() {
		Expect(evalhistory.CompareRulesFor([]string{"architecture.made_up"})).To(MatchError(ContainSubstring("architecture.made_up")))
	})

	It("refuses a rule the placement table does not hold, naming the rule", func() {
		signals := []evalhistory.Signal{{RuleID: "architecture.made_up", Path: "p/p.go", StartRow: 6, EndRow: 9}}

		_, err := evalhistory.FlaggedUnits(inventory, signals, "architecture.made_up")
		Expect(err).To(MatchError(ContainSubstring("architecture.made_up")))
	})

	It("places each finding of a file-level rule in its file, whatever its rows", func() {
		signals := []evalhistory.Signal{
			{RuleID: "complexity.branch_density", Path: "q/q.go"},
			{RuleID: "complexity.branch_density", Path: "p/p.go", Subject: "B", StartRow: 6, EndRow: 9},
		}

		flagged := flaggedUnits(inventory, signals, "complexity.branch_density")
		Expect(flagged.Level).To(Equal(evalhistory.FileLevel))
		Expect(flagged.Units).To(Equal([]int{1, 0}))
		Expect(flagged.Unplaced).To(BeZero())
	})

	DescribeTable("a function finding in a gap between units is unplaced, not given to the next unit",
		func(startRow int) {
			signals := []evalhistory.Signal{{RuleID: "complexity.cognitive_complexity", Path: "p/p.go", Subject: "X", StartRow: startRow, EndRow: startRow}}

			flagged := flaggedUnits(inventory, signals, "complexity.cognitive_complexity")
			Expect(flagged.Units).To(BeEmpty())
			Expect(flagged.Unplaced).To(Equal(1))
		},
		Entry("before the first function", 1),
		Entry("between two functions", 5),
	)

	It("lists a unit flagged twice once", func() {
		signals := []evalhistory.Signal{
			{RuleID: "complexity.cognitive_complexity", Path: "p/p.go", Subject: "B", StartRow: 6, EndRow: 9},
			{RuleID: "complexity.cognitive_complexity", Path: "p/p.go", Subject: "B", StartRow: 7, EndRow: 8},
		}

		Expect(flaggedUnits(inventory, signals, "complexity.cognitive_complexity").Units).To(Equal([]int{1}))
	})
})

// flaggedUnits is FlaggedUnits for a rule the table holds: any error fails the spec.
func flaggedUnits(inventory evalhistory.Inventory, signals []evalhistory.Signal, ruleID string) evalhistory.Flagged {
	GinkgoHelper()
	flagged, err := evalhistory.FlaggedUnits(inventory, signals, ruleID)
	Expect(err).NotTo(HaveOccurred())
	return flagged
}

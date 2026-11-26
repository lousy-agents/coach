package evalhistory_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalhistory"
)

var _ = Describe("the unit inventory at since", func() {
	When("the tree holds production Go files, a test file, vendored and testdata code, and other files", func() {
		It("lists each production function with its lines, and each production file", func() {
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

			inventory, err := evalhistory.GoInventory(h.dir, since)
			Expect(err).NotTo(HaveOccurred())
			Expect(inventory.Functions).To(ConsistOf(
				evalhistory.Unit{Path: "p/p.go", Start: 3, End: 5},
				evalhistory.Unit{Path: "p/p.go", Start: 7, End: 10},
				evalhistory.Unit{Path: "q/q.go", Start: 5, End: 5},
			))
			Expect(inventory.Files).To(ConsistOf(
				evalhistory.Unit{Path: "p/p.go", Start: 1, End: 10},
				evalhistory.Unit{Path: "q/q.go", Start: 1, End: 5},
			))
		})
	})

	When("a Go file at since does not parse", func() {
		It("refuses, naming the file", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{"p/p.go": baseSource, "p/broken.go": "package p\n\nfunc (\n"})

			_, err := evalhistory.GoInventory(h.dir, since)
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

		cognitive := evalhistory.FlaggedUnits(inventory, signals, "complexity.cognitive_complexity")
		density := evalhistory.FlaggedUnits(inventory, signals, "complexity.branch_density")

		Expect(cognitive.Level).To(Equal(evalhistory.FunctionLevel))
		Expect(cognitive.Units).To(Equal([]int{1, 2}))
		Expect(density.Level).To(Equal(evalhistory.FileLevel))
		Expect(density.Units).To(Equal([]int{1}))
		Expect(cognitive.Unplaced).To(BeZero())
	})

	It("counts a finding that falls in no unit as unplaced", func() {
		signals := []evalhistory.Signal{{RuleID: "complexity.cognitive_complexity", Path: "gone.go", Subject: "G", StartRow: 2, EndRow: 4}}

		flagged := evalhistory.FlaggedUnits(inventory, signals, "complexity.cognitive_complexity")
		Expect(flagged.Units).To(BeEmpty())
		Expect(flagged.Unplaced).To(Equal(1))
	})

	It("lists a unit flagged twice once", func() {
		signals := []evalhistory.Signal{
			{RuleID: "complexity.cognitive_complexity", Path: "p/p.go", Subject: "B", StartRow: 6, EndRow: 9},
			{RuleID: "complexity.cognitive_complexity", Path: "p/p.go", Subject: "B", StartRow: 7, EndRow: 8},
		}

		Expect(evalhistory.FlaggedUnits(inventory, signals, "complexity.cognitive_complexity").Units).To(Equal([]int{1}))
	})
})

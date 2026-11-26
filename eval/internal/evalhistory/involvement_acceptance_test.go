package evalhistory_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalhistory"
)

var unitsAB = []evalhistory.Unit{{Path: "p/p.go", Start: 3, End: 5}, {Path: "p/p.go", Start: 7, End: 10}}

func fixesOf(scan evalhistory.FixScan) []evalhistory.FixCommit { return scan.Fixes }

var _ = Describe("which units a later fix touched", func() {
	When("fixes change an old line of one unit, only new lines, only whitespace, or only add lines", func() {
		It("marks only the unit whose old line a fix changed, and counts the other fixes as unmapped", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{"p/p.go": baseSource})
			withC := baseSource + "\nfunc C() int {\n\treturn 3\n}\n"
			h.commit("add C", map[string]string{"p/p.go": withC})
			fixB := h.commit("fix: wrong value in B", map[string]string{"p/p.go": replaceLine(withC, 8, "\tx := 3")})
			afterB := replaceLine(withC, 8, "\tx := 3")
			h.commit("fix: C returns the wrong value", map[string]string{"p/p.go": replaceLine(afterB, 13, "\treturn 4")})
			afterC := replaceLine(afterB, 13, "\treturn 4")
			h.commit("fix: indentation in A", map[string]string{"p/p.go": replaceLine(afterC, 4, "    return 1")})
			afterWS := replaceLine(afterC, 4, "    return 1")
			until := h.commit("fix: add a guard to A", map[string]string{"p/p.go": insertLine(afterWS, 4, "\t_ = 0")})

			scan, err := evalhistory.FindFixes(h.dir, since, until)
			Expect(err).NotTo(HaveOccurred())
			Expect(scan.Fixes).To(HaveLen(4))

			result, err := evalhistory.Involve(h.dir, since, unitsAB, fixesOf(scan))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.FixSHA).To(Equal([]string{"", fixB}))
			Expect(result.Unmapped).To(Equal(3))
		})
	})

	When("the file is renamed before the fix", func() {
		It("still traces the fixed line to its unit at since", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{"p/p.go": baseSource})
			h.git("mv", "p/p.go", "q.go")
			h.commit("move p.go", nil)
			fix := h.commit("fix: A returns the wrong value", map[string]string{"q.go": replaceLine(baseSource, 4, "\treturn 5")})

			scan, err := evalhistory.FindFixes(h.dir, since, fix)
			Expect(err).NotTo(HaveOccurred())
			result, err := evalhistory.Involve(h.dir, since, unitsAB, fixesOf(scan))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.FixSHA).To(Equal([]string{fix, ""}))
			Expect(result.Unmapped).To(BeZero())
		})
	})

	When("a fix deletes a unit's line", func() {
		It("involves that unit", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{"p/p.go": baseSource})
			fix := h.commit("fix: drop the dead store in B", map[string]string{"p/p.go": deleteLine(baseSource, 8)})

			scan, err := evalhistory.FindFixes(h.dir, since, fix)
			Expect(err).NotTo(HaveOccurred())
			result, err := evalhistory.Involve(h.dir, since, unitsAB, fixesOf(scan))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.FixSHA).To(Equal([]string{"", fix}))
		})
	})
})

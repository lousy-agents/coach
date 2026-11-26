package evalhistory_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalhistory"
)

var unitsAB = []evalhistory.Unit{{Path: "p/p.go", Start: 3, End: 5}, {Path: "p/p.go", Start: 7, End: 10}}

// sumAndOther are Sum (lines 3-9) and Other (lines 11-14) in "package p\n\n" + sumFunc + "\n" + otherFunc.
var sumAndOther = []evalhistory.Unit{{Path: "p/p.go", Start: 3, End: 9}, {Path: "p/p.go", Start: 11, End: 14}}

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

	When("a unit's lines move to another file in the same commit before the fix", func() {
		It("follows the copied lines back to the unit", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{"p/p.go": "package p\n\n" + sumFunc + "\n" + otherFunc})
			h.commit("move Sum to sum.go", map[string]string{"p/p.go": "package p\n\n" + otherFunc, "p/sum.go": "package p\n\n" + sumFunc})
			moved := "package p\n\n" + sumFunc
			fix := h.commit("fix: Sum starts from one", map[string]string{"p/sum.go": replaceLine(moved, 4, "\ttotal := 1")})

			scan, err := evalhistory.FindFixes(h.dir, since, fix)
			Expect(err).NotTo(HaveOccurred())
			result, err := evalhistory.Involve(h.dir, since, sumAndOther, fixesOf(scan))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.FixSHA).To(Equal([]string{fix, ""}))
		})
	})

	When("a unit moves within its file before the fix", func() {
		It("follows the moved lines back to the unit", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{"p/p.go": "package p\n\n" + sumFunc + "\n" + otherFunc})
			reordered := "package p\n\n" + otherFunc + "\n" + sumFunc
			h.commit("put Other first", map[string]string{"p/p.go": reordered})
			sumStart := 3 + len(lines(otherFunc)) + 1
			fix := h.commit("fix: Sum starts from one", map[string]string{"p/p.go": replaceLine(reordered, sumStart+1, "\ttotal := 1")})

			scan, err := evalhistory.FindFixes(h.dir, since, fix)
			Expect(err).NotTo(HaveOccurred())
			result, err := evalhistory.Involve(h.dir, since, sumAndOther, fixesOf(scan))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.FixSHA).To(Equal([]string{fix, ""}))
		})
	})

	When("a unit's line is only reindented before the fix changes it", func() {
		It("still traces the line to the unit, ignoring the whitespace change", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{"p/p.go": baseSource})
			reindented := replaceLine(baseSource, 8, "    x := 2")
			h.commit("reindent B", map[string]string{"p/p.go": reindented})
			fix := h.commit("fix: wrong value in B", map[string]string{"p/p.go": replaceLine(reindented, 8, "    x := 3")})

			scan, err := evalhistory.FindFixes(h.dir, since, fix)
			Expect(err).NotTo(HaveOccurred())
			result, err := evalhistory.Involve(h.dir, since, unitsAB, fixesOf(scan))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.FixSHA).To(Equal([]string{"", fix}))
		})
	})

	When("two fixes touch the same unit", func() {
		It("records the older fix", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{"p/p.go": baseSource})
			first := h.commit("fix: wrong value in B", map[string]string{"p/p.go": replaceLine(baseSource, 9, "\treturn x + 1")})
			second := h.commit("fix: wrong start in B", map[string]string{"p/p.go": replaceLine(replaceLine(baseSource, 9, "\treturn x + 1"), 8, "\tx := 3")})

			scan, err := evalhistory.FindFixes(h.dir, since, second)
			Expect(err).NotTo(HaveOccurred())
			result, err := evalhistory.Involve(h.dir, since, unitsAB, fixesOf(scan))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.FixSHA).To(Equal([]string{"", first}))
		})
	})

	DescribeTable("a fix that also changes another file still traces its unit, whatever that file is called or holds",
		func(before, after map[string]string) {
			h := newHistory()
			before["p/p.go"] = baseSource
			since := h.commit("base", before)
			after["p/p.go"] = replaceLine(baseSource, 8, "\tx := 3")
			fix := h.commit("fix: wrong value in B", after)

			scan, err := evalhistory.FindFixes(h.dir, since, fix)
			Expect(err).NotTo(HaveOccurred())
			result, err := evalhistory.Involve(h.dir, since, unitsAB, fixesOf(scan))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.FixSHA).To(Equal([]string{"", fix}))
		},
		Entry("a path with a space, which git's diff header ends with a tab",
			map[string]string{"docs/design notes.md": "one\ntwo\n"}, map[string]string{"docs/design notes.md": "one\nTWO\n"}),
		Entry("a non-ASCII path, which git's diff header quotes",
			map[string]string{"docs/été.md": "one\ntwo\n"}, map[string]string{"docs/été.md": "one\nTWO\n"}),
		Entry("a removed line that starts with \"-- \", before a second hunk",
			map[string]string{"q.sql": "-- one\nselect 1;\nselect 2;\nselect 3;\nselect 4;\n"}, map[string]string{"q.sql": "select 1;\nselect 2;\nselect 3;\nselect 5;\n"}),
		Entry("a path that reads as a pathspec glob for another changed path",
			map[string]string{"d/[a].md": "x\n", "d/a.md": strings.Repeat("line\n", 30)},
			map[string]string{"d/[a].md": "y\n", "d/a.md": replaceLine(strings.Repeat("line\n", 30), 20, "changed")}),
	)

	When("a unit sits in a file with a non-ASCII name", func() {
		It("traces a fix of that file to the unit", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{"p/é.go": baseSource})
			fix := h.commit("fix: wrong value in B", map[string]string{"p/é.go": replaceLine(baseSource, 8, "\tx := 3")})

			scan, err := evalhistory.FindFixes(h.dir, since, fix)
			Expect(err).NotTo(HaveOccurred())
			units := []evalhistory.Unit{{Path: "p/é.go", Start: 3, End: 5}, {Path: "p/é.go", Start: 7, End: 10}}
			result, err := evalhistory.Involve(h.dir, since, units, fixesOf(scan))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.FixSHA).To(Equal([]string{"", fix}))
		})
	})

	When("the fix itself renames the file and edits one unit", func() {
		It("involves only the edited unit, not every unit of the renamed file", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{"p/p.go": baseSource})
			h.git("mv", "p/p.go", "q.go")
			fix := h.commit("fix: wrong value in B", map[string]string{"q.go": replaceLine(baseSource, 8, "\tx := 3")})

			scan, err := evalhistory.FindFixes(h.dir, since, fix)
			Expect(err).NotTo(HaveOccurred())
			result, err := evalhistory.Involve(h.dir, since, unitsAB, fixesOf(scan))
			Expect(err).NotTo(HaveOccurred())
			Expect(result.FixSHA).To(Equal([]string{"", fix}))
		})
	})

	DescribeTable("a unit outside its file at since is refused, naming the unit, rather than traced in part",
		func(unit evalhistory.Unit, named string) {
			h := newHistory()
			since := h.commit("base", map[string]string{"p/p.go": baseSource})

			_, err := evalhistory.Involve(h.dir, since, []evalhistory.Unit{unit}, nil)
			Expect(err).To(MatchError(ContainSubstring(named)))
		},
		Entry("one that runs past the end", evalhistory.Unit{Path: "p/p.go", Start: 7, End: 40}, "p/p.go:7-40"),
		Entry("one that starts before line 1", evalhistory.Unit{Path: "p/p.go", Start: 0, End: 3}, "p/p.go:0-3"),
	)
})

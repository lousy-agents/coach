package evalhistory_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalhistory"
)

var _ = Describe("finding fix commits offline", func() {
	DescribeTable("the pinned fix pattern",
		func(message string, matches bool) {
			Expect(evalhistory.FixPattern.MatchString(message)).To(Equal(matches))
		},
		Entry("Fix crash", "Fix crash on empty input", true),
		Entry("fixes a leak", "fixes a leak in the pool", true),
		Entry("fixed race", "fixed race in scheduler", true),
		Entry("bugfix", "bugfix: wrong total", true),
		Entry("bug fix", "bug fix for parser", true),
		Entry("bug", "bug in parser", true),
		Entry("defect", "defect 9 addressed", true),
		Entry("regression", "regression in y", true),
		Entry("hotfix", "hotfix for prod", true),
		Entry("closes #N", "Closes #42", true),
		Entry("resolves #N", "resolves #7 by retrying", true),
		Entry("fixture is not a fix", "add fixture data", false),
		Entry("prefix is not a fix", "prefix routes with /api", false),
		Entry("debug is not a bug", "debug logging", false),
		Entry("resolve without an issue", "resolve conflicts", false),
	)

	When("the window holds fixes, other commits and a merge", func() {
		It("returns the non-merge fixes after since up to until, oldest first, and counts every non-merge commit", func() {
			h := newHistory()
			since := h.commit("fix: before the window", map[string]string{"p/p.go": baseSource})
			fix1 := h.commit("fix: null deref", map[string]string{"a.txt": "1"})
			fix2 := h.commit("Fixes #12", map[string]string{"a.txt": "2"})
			h.commit("refactor names", map[string]string{"a.txt": "3"})
			h.git("checkout", "-q", "-b", "side")
			h.commit("side work", map[string]string{"b.txt": "1"})
			h.git("checkout", "-q", "main")
			h.git("merge", "-q", "--no-ff", "-m", "Merge branch fix-side", "side")
			until := h.git("rev-parse", "HEAD")
			h.commit("fix: after the window", map[string]string{"a.txt": "4"})

			scan, err := evalhistory.FindFixes(h.dir, since, until)
			Expect(err).NotTo(HaveOccurred())
			Expect(scan.Fixes).To(Equal([]evalhistory.FixCommit{{SHA: fix1, Subject: "fix: null deref"}, {SHA: fix2, Subject: "Fixes #12"}}))
			Expect(scan.Commits).To(Equal(4))
		})
	})

	When("since or until is not a commit in the repository", func() {
		It("refuses", func() {
			h := newHistory()
			since := h.commit("base", map[string]string{"p/p.go": baseSource})

			_, err := evalhistory.FindFixes(h.dir, since, "0123456789abcdef0123456789abcdef01234567")
			Expect(err).To(HaveOccurred())
		})
	})
})

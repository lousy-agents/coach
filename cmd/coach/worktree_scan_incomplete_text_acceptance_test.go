package main

import (
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree text names each cause once, per reason (#168 item 22)", func() {
	const hiddenName = "zz_hidden_name.go"

	notListedFive := `\b5\b[^\n]*not listed`

	It("groups 25 in-scope roots into the clauses one root gets, with the first 20 and a not-listed count (#168 item 22)", func() {
		repo := scanRepoWith(rootedFiles("d", 25, hiddenName, "package d\n"))
		hidePaths(repo, skipWorktreeHiding, rootsNamed("d", 1, 25), hiddenName)
		oneRoot := scanRepoWith(rootedFiles("d", 1, hiddenName, "package d\n"))
		hidePaths(oneRoot, skipWorktreeHiding, rootsNamed("d", 1, 1), hiddenName)

		text := runProjectText(repo)

		Expect(noFindingsHeadline(text)).NotTo(Equal(unqualifiedAllClear))
		Expect(strings.Count(text, skipWorktreeHiding.hint)).To(Equal(strings.Count(runProjectText(oneRoot), skipWorktreeHiding.hint)),
			"the cause is stated as often for 25 roots as for one, not once per root")
		Expect(text).To(ContainSubstring("d01/"))
		Expect(text).To(ContainSubstring("d20/"))
		Expect(text).NotTo(ContainSubstring("d21/"))
		Expect(text).To(MatchRegexp(notListedFive))
		Expect(text).NotTo(ContainSubstring("zz_hidden_name"), "individual hidden files are never listed")
		Expect(text).NotTo(ContainSubstring("warning:"))
	})

	It("lists the root that hides in-scope source in the text although it sorts after the 20 listed (#168 item 22)", func() {
		files := rootedFiles("d", 24, "zz_hidden_name.md", "# notes\n")
		files["d25/zz_hidden_name.go"] = "package d\n"
		repo := scanRepoWith(files)
		hidePaths(repo, skipWorktreeHiding, rootsNamed("d", 1, 24), "zz_hidden_name.md")
		skipWorktreeHiding.hide(repo, "d25/zz_hidden_name.go")

		text := runProjectText(repo)

		Expect(text).To(ContainSubstring("d25/"))
		Expect(text).To(ContainSubstring("d19/"))
		Expect(text).NotTo(ContainSubstring("d20/"))
		Expect(text).To(MatchRegexp(notListedFive))
	})

	It("groups 25 roots that hide no source into one disclosed line, leaving the all-clear unqualified (#168 item 22)", func() {
		repo := scanRepoWith(rootedFiles("d", 25, "zz_hidden_name.md", "# notes\n"))
		hidePaths(repo, sparseHiding, rootsNamed("d", 1, 25), "zz_hidden_name.md")

		text := runProjectText(repo)

		Expect(noFindingsHeadline(text)).To(Equal(unqualifiedAllClear))
		Expect(regexp.MustCompile(hiddenLine("sparse checkout")).FindAllString(text, -1)).To(HaveLen(1))
		Expect(text).To(ContainSubstring("d20/"))
		Expect(text).NotTo(ContainSubstring("d21/"))
		Expect(text).To(MatchRegexp(notListedFive))
		Expect(text).NotTo(ContainSubstring("zz_hidden_name"))
		Expect(text).NotTo(ContainSubstring("warning:"))
	})
})

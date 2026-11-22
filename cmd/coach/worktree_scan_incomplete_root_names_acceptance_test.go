package main

import (
	"regexp"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree shows a hidden root whose name holds a control character quoted, never raw (#168 item 22)", func() {
	DescribeTable("the text names the root quoted and escaped on one line, and the JSON names the real directory (#168 item 22)",
		func(m hiding, directory string) {
			repo := scanRepoWith(map[string]string{
				directory + "/hidden.md":  "# hidden\n",
				directory + "/visible.md": "# visible\n",
			})
			m.hide(repo, directory+"/hidden.md")

			run := runProjectScan(repo)
			text := runProjectText(repo)

			Expect(run.roots(m.reason)).To(Equal([]string{directory}))
			Expect(text).NotTo(ContainSubstring("\x1b"), "a raw escape byte would reach the terminal")
			Expect(text).To(MatchRegexp(quotedRootLine(directory)), "the root is quoted and the diagnostic stays on one line")
		},
		Entry("skip-worktree, escape sequence", skipWorktreeHiding, "ctl\x1b[31mred"),
		Entry("assume-unchanged, escape sequence", assumeUnchangedHiding, "ctl\x1b[31mred"),
		Entry("skip-worktree, newline", skipWorktreeHiding, "line\nbreak"),
		Entry("assume-unchanged, newline", assumeUnchangedHiding, "line\nbreak"),
	)
})

func quotedRootLine(directory string) string {
	outside := regexp.QuoteMeta(strconv.Quote(directory)) + "/"
	inside := regexp.QuoteMeta(strconv.Quote(directory + "/"))
	return `(?m)^[^\n]*kind: worktree_scan_incomplete[^\n]*, root: (` + outside + `|` + inside + `)[^\n]*hides_in_scope: no$`
}

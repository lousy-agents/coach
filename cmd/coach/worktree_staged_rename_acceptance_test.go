package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The status of the working tree is run without rename detection, which reads the blob of each side of a staged
// deletion and a staged addition and, on a git that ignores the instruction not to fetch, fetched one that only the
// index names. A file renamed in the index is therefore two paths wherever the status is reported: the old one, deleted,
// and the new one, added. It used to be one, the new path, and what was listed as relevant or not analyzed is the same
// otherwise.
var _ = Describe("coach codesignal: a file renamed in the index is reported as its deletion and its addition", func() {
	It("lists both paths in dirty_worktree and in the text warning of --check-project", func() {
		repo := noSupportedCompilerRepo()
		commitFile(repo, "src/a.ts", "export const a = 1;\n")
		gitIn(repo, "mv", "src/a.ts", "src/b.ts")

		doc, text := checkProjectBothFormats(repo, pathWithoutNode())

		Expect(doc.DirtyWorktree.RelevantChanges).To(BeTrue())
		Expect(doc.DirtyWorktree.Paths).To(Equal([]string{"src/a.ts", "src/b.ts"}), "the old path as well as the new one")
		Expect(text).To(ContainSubstring("  src/a.ts\n  src/b.ts\n"))
	})

	It("counts both in the staged files a scan did not analyze", func() {
		repo := commitScannableGoRepo()
		gitIn(repo, "mv", "a.go", "b.go")

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		report := decodeCoachReport(stdout)
		Expect(report.Diagnostics).To(ContainElement(HaveField("Message", "2 staged files were not analyzed: a.go, b.go")), "diagnostics: %+v", report.Diagnostics)
	})
})

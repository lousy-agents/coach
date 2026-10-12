package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --baseline", func() {
	When("the tracked tree contains a supported and an unsupported file", func() {
		It("summarizes the unsupported file via Coverage instead of a per-file diagnostic", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "notes.txt", "hello\n")

			report, stderr := runCoachCodesignalBaseline(repo)
			Expect(stderr).To(BeEmpty())

			Expect(report.Coverage).NotTo(BeNil())
			Expect(report.Coverage.TrackedFilesDiscovered).To(Equal(2))
			Expect(report.Coverage.FilesAnalyzed).To(Equal(1))

			foundUnsupported := false
			for _, g := range report.Coverage.Unsupported {
				if g.Reason == "unsupported_language" && g.Count >= 1 {
					foundUnsupported = true
				}
			}
			Expect(foundUnsupported).To(BeTrue(), "notes.txt should be accounted for in Coverage.Unsupported")

			Expect(hasDiagnostic(report, "unsupported_language", "notes.txt")).To(BeFalse(), "a baseline scan must not flood the report with a per-file unsupported_language diagnostic")
		})
	})
})

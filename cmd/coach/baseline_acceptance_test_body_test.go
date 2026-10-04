package main

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func body_baselineAcceptanceTest_summarizesTheUnsupportedFileViaCoverageInsteadOf_164() {
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
}

func body_baselineAcceptanceTest_shallIncludeComplexityCognitiveComplexityInBasel_192() {
	// Six nested ifs: structural costs 1+2+3+4+5+6 = 21 (>= 15).
	const tangled = `package a

func tangle(n int) {
	if n > 0 {
		if n > 1 {
			if n > 2 {
				if n > 3 {
					if n > 4 {
						if n > 5 {
							return
						}
					}
				}
			}
		}
	}
}
`
	repo := newTempGitRepo()
	commitFile(repo, "tangle.go", tangled)

	report, stderr := runCoachCodesignalBaseline(repo)
	Expect(stderr).To(BeEmpty())

	var cc []codesignal.Signal
	for _, s := range report.Signals {
		if s.RuleID == "complexity.cognitive_complexity" && s.Path == "tangle.go" {
			cc = append(cc, s)
		}
	}
	Expect(cc).To(HaveLen(1), "baseline must surface the over-threshold function via the existing report path")
	Expect(cc[0].Kind).To(Equal("cognitive_complexity"))
	Expect(cc[0].Subject).To(Equal("tangle"))
	Expect(cc[0].Evidence).To(Equal("cognitive_complexity=21"))
	Expect(cc[0].Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))
	Expect(cc[0].Confidence).To(Equal(codesignal.Confidence("high")))
	Expect(cc[0].Provenance.Producer).To(Equal("codesignal"))
}

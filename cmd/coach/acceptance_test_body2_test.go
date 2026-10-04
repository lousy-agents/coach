package main

import (
	"encoding/json"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func body_acceptanceTest_stillAnalyzesTheHEADPathWhenGitReportsAScoredRen_823() {
	repo := newTempGitRepo()
	base := paddedGoPackage("a", 60, "")
	initialSHA := commitFile(repo, "carrier.go", base)
	renameAndWrite(repo, "carrier.go", "relocated.go", paddedGoPackage("a", 60, hiddenInputMutationFn))
	expectCoachStatusPrefix(repo, initialSHA, "relocated.go", "R")

	report, _ := runCoachCodesignal(repo, initialSHA)

	Expect(hasDiagnostic(report, "unsupported_change_type", "relocated.go")).To(BeFalse())
	signals := signalsForPath(report, "relocated.go")
	Expect(signals).NotTo(BeEmpty(), "new risky code on a renamed path must produce signals")
	for i := range signals {
		Expect(signals[i].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
	}
	Expect(hasDiagnostic(report, "continuity_not_determined", "relocated.go")).To(BeTrue())
}

func body_acceptanceTest_analyzesTheCopySHEADPathWithoutEstablishingCopyC_843() {
	repo := newTempGitRepo()
	content := paddedGoPackage("a", 60, hiddenInputMutationFn)
	initialSHA := commitFile(repo, "template.go", content)
	commitFile(repo, "duplicate.go", content)
	expectCoachStatusPrefix(repo, initialSHA, "duplicate.go", "C")

	report, _ := runCoachCodesignal(repo, initialSHA)

	Expect(hasDiagnostic(report, "unsupported_change_type", "duplicate.go")).To(BeFalse())
	signals := signalsForPath(report, "duplicate.go")
	Expect(signals).NotTo(BeEmpty(), "a copy-detected new file must be analyzed")
	for i := range signals {
		Expect(signals[i].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
	}
	Expect(hasDiagnostic(report, "continuity_not_determined", "duplicate.go")).To(BeTrue())
}

func body_acceptanceTest_qualifiesTheNoFindingsHeadlineAndCountsTheUnanal_863() {
	repo := newTempGitRepo()
	initialSHA := commitFile(repo, "typed.go", "package typed\n\nfunc A() {}\n")
	commitFile(repo, "target.go", "package target\n")
	typechangeFileToSymlink(repo, "typed.go", "target.go")
	expectCoachStatusPrefix(repo, initialSHA, "typed.go", "T")

	stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
	Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

	var report codesignal.Report
	Expect(json.Unmarshal(stdout, &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout)
	Expect(hasDiagnostic(&report, "unsupported_change_type", "typed.go")).To(BeTrue())
	Expect(report.Summary.FilesWithDiagnostics).To(BeNumerically(">=", 1),
		"a diagnostic on an unanalyzed path must count in files_with_diagnostics; got %d", report.Summary.FilesWithDiagnostics)

	var document struct {
		Summary struct {
			FilesUnanalyzed *int `json:"files_unanalyzed"`
		} `json:"summary"`
	}
	Expect(json.Unmarshal(stdout, &document)).To(Succeed())
	Expect(document.Summary.FilesUnanalyzed).NotTo(BeNil(), "JSON must expose a numeric files_unanalyzed field when files were not analyzed")
	Expect(*document.Summary.FilesUnanalyzed).To(BeNumerically(">=", 1),
		"files_unanalyzed must count the typechanged path; got %d", *document.Summary.FilesUnanalyzed)

	textOut, textErr, textExit := runCoachCodesignalRaw(repo, initialSHA)
	Expect(textExit).To(Equal(0), "stderr: %s", textErr)
	verdict := strings.SplitN(string(textOut), "\n", 2)[0]

	for _, line := range strings.Split(string(textOut), "\n") {
		if strings.HasPrefix(line, "No active CodeSignal findings") {
			verdict = line
			break
		}
	}
	Expect(verdict).NotTo(Equal("No active CodeSignal findings."),
		"unqualified all-clear must not appear when a file was not analyzed")
	Expect(verdict).To(ContainSubstring("1 path was not analyzed"),
		"text unanalyzed count must match JSON files_unanalyzed=%d; got %q", *document.Summary.FilesUnanalyzed, verdict)
}

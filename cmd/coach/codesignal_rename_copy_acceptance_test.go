package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal", func() {
	When("a file containing a signal-producing construct is renamed with no content change", func() {
		It("analyzes the HEAD path without establishing rename continuity", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "old.go", hiddenInputMutationGo)
			renameFile(repo, "old.go", "new.go")
			expectCoachStatusPrefix(repo, initialSHA, "new.go", "R")

			report, _ := runCoachCodesignal(repo, initialSHA)

			Expect(hasDiagnostic(report, "unsupported_change_type", "new.go")).To(BeFalse(),
				"a rename's new path must be analyzed, not skipped as unsupported_change_type")
			signals := signalsForPath(report, "new.go")
			Expect(signals).NotTo(BeEmpty(), "HEAD content of a renamed file must yield signals")
			for i := range signals {
				Expect(signals[i].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")),
					"rename continuity is not established, so signals must not inherit introduced from added-file policy")
			}
			Expect(hasDiagnostic(report, "continuity_not_determined", "new.go")).To(BeTrue(),
				"the report must say continuity was not determined on the analyzed new path")
			Expect(signalsForPath(report, "old.go")).To(BeEmpty(), "the old path no longer exists at HEAD and must not appear in the report")

			textOut, textErr, textExit := runCoachCodesignalRaw(repo, initialSHA)
			Expect(textExit).To(Equal(0), "stderr: %s", textErr)
			Expect(string(textOut)).NotTo(ContainSubstring("not analyzed"),
				"an analyzed rename must not be described as unanalyzed")
			var document struct {
				Summary struct {
					FilesUnanalyzed *int `json:"files_unanalyzed"`
				} `json:"summary"`
			}
			jsonOut, jsonErr, jsonExit := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(jsonExit).To(Equal(0), "stderr: %s", jsonErr)
			Expect(json.Unmarshal(jsonOut, &document)).To(Succeed())
			Expect(document.Summary.FilesUnanalyzed).To(BeNil(),
				"JSON must omit files_unanalyzed when every changed path was analyzed")
		})
	})

	When("a large file is renamed and new risky code is appended", func() {
		It("still analyzes the HEAD path when git reports a scored rename rather than D+A", func() {
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
		})
	})

	When("a file is copied from an untouched source and Git reports a copy", func() {
		It("analyzes the copy's HEAD path without establishing copy continuity", func() {
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
		})
	})

	When("a changed file has an unsupported git status such as typechange", func() {
		It("qualifies the no-findings headline and counts the unanalyzed path in JSON", func() {
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
			verdict := noFindingsHeadline(string(textOut))
			Expect(verdict).NotTo(Equal("No active CodeSignal findings."),
				"unqualified all-clear must not appear when a file was not analyzed")
			Expect(verdict).To(ContainSubstring("1 path was not analyzed"),
				"text unanalyzed count must match JSON files_unanalyzed=%d; got %q", *document.Summary.FilesUnanalyzed, verdict)
		})
	})

	When("an analyzed rename carries a syntax diagnostic and a typechanged path is unanalyzed", func() {
		It("renders an unanalyzed count that matches JSON files_unanalyzed rather than every diagnostic path", func() {
			repo := newTempGitRepo()
			commitFile(repo, "typed.go", "package typed\n\nfunc A() {}\n")
			initialSHA := commitFile(repo, "broken.go", "package broken\n\nfunc (\n")
			renameFile(repo, "broken.go", "moved.go")
			commitFile(repo, "target.go", "package target\n")
			typechangeFileToSymlink(repo, "typed.go", "target.go")
			expectCoachStatusPrefix(repo, initialSHA, "moved.go", "R")
			expectCoachStatusPrefix(repo, initialSHA, "typed.go", "T")

			jsonOut, jsonErr, jsonExit := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(jsonExit).To(Equal(0), "stderr: %s", jsonErr)
			var report codesignal.Report
			Expect(json.Unmarshal(jsonOut, &report)).To(Succeed(), "stdout should be one JSON report: %s", jsonOut)
			Expect(hasDiagnostic(&report, "syntax_errors", "moved.go")).To(BeTrue(),
				"the analyzed rename must still surface its syntax diagnostic")
			Expect(hasDiagnostic(&report, "unsupported_change_type", "typed.go")).To(BeTrue())
			Expect(report.Signals).To(BeEmpty(), "syntax-error HEAD content must not produce signals")
			Expect(report.Summary.FilesUnanalyzed).To(Equal(1),
				"only the typechanged path is unanalyzed; the renamed path is in Files")

			textOut, textErr, textExit := runCoachCodesignalRaw(repo, initialSHA)
			Expect(textExit).To(Equal(0), "stderr: %s", textErr)
			verdict := verdictLine(string(textOut))
			Expect(verdict).To(ContainSubstring("1 path was not analyzed"),
				"text must use Summary.FilesUnanalyzed, not the count of diagnostic paths; got %q", verdict)
			Expect(verdict).NotTo(ContainSubstring("2 paths were not analyzed"),
				"an analyzed rename's diagnostic must not inflate the unanalyzed count")
		})
	})
})

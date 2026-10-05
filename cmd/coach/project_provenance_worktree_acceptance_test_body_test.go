package main

import (
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func body_projectProvenanceWorktreeAcceptanceTest_51() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectProvenanceWorktreeAcceptanceTest_emitsWorktreeReportReflectsCommittedHeadDiagnost_80(report *codesignal.Report) {
	kinds := make([]string, len(report.Diagnostics))
	for i, d := range report.Diagnostics {
		kinds[i] = d.Kind
	}
	Expect(kinds).To(ContainElement(codesignal.DiagKindWorktreeReportReflectsCommittedHEAD),
		"diagnostics must include worktree_report_reflects_committed_head; got %v", kinds)
	Expect(kinds).NotTo(ContainElement(codesignal.DiagKindWorktreeChangesNotAnalyzed),
		"unsupported lockfile dirt must not use the file-local skip kind; got %v", kinds)
}

func body_projectProvenanceWorktreeAcceptanceTest_diagnosticMessageStatesTheReportAppliesToCommitt_91(report *codesignal.Report) {
	var msg string
	for _, d := range report.Diagnostics {
		if d.Kind == codesignal.DiagKindWorktreeReportReflectsCommittedHEAD {
			msg = d.Message
		}
	}
	Expect(msg).To(ContainSubstring("HEAD"), "message must reference committed HEAD; got %q", msg)
}

func body_projectProvenanceWorktreeAcceptanceTest_emitsWorktreeReportReflectsCommittedHeadDiagnost_154(report *codesignal.Report) {
	kinds := make([]string, len(report.Diagnostics))
	for i, d := range report.Diagnostics {
		kinds[i] = d.Kind
	}
	Expect(kinds).To(ContainElement(codesignal.DiagKindWorktreeReportReflectsCommittedHEAD))
	Expect(kinds).NotTo(ContainElement(codesignal.DiagKindWorktreeChangesNotAnalyzed))
}

func body_projectProvenanceWorktreeAcceptanceTest_reportsPackageManagerVersionAsASemverStringWhenN_191(report *codesignal.Report) {
	if _, err := exec.LookPath("npm"); err != nil {
		Skip("npm not on PATH: cannot assert version")
	}
	Expect(report.ProjectProvenance.PackageManager).NotTo(BeNil())
	Expect(report.ProjectProvenance.PackageManager.Version).To(MatchRegexp(`^\d+\.\d+\.\d+`),
		"package_manager.version must be a semver string when npm is on PATH; got %q", report.ProjectProvenance.PackageManager.Version)
}

func body_projectProvenanceWorktreeAcceptanceTest_doesNotEmitAWorktreeSkipOrProvenanceDiagnosticFo_200(report *codesignal.Report) {
	for _, d := range report.Diagnostics {
		Expect(d.Kind).NotTo(Equal(codesignal.DiagKindWorktreeChangesNotAnalyzed),
			"clean worktree must not trigger worktree_changes_not_analyzed")
		Expect(d.Kind).NotTo(Equal(codesignal.DiagKindWorktreeReportReflectsCommittedHEAD),
			"clean worktree must not trigger worktree_report_reflects_committed_head")
	}
}

func body_projectProvenanceWorktreeAcceptanceTest_233() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

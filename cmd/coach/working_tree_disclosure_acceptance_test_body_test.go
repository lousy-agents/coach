package main

import (
	"path/filepath"

	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func body_workingTreeDisclosureAcceptanceTest_theCLIReportsTheTotalCountAndABoundedSampleRathe_207(mode, format string) {
	repo := newTempGitRepo()
	commitBenignHistory(repo)
	names := bulkUntrackedNames(20)
	for _, name := range names {
		writeUntrackedFile(repo, name, hiddenInputMutationGo)
	}
	porcelain := expectDirtyPorcelain(repo, "?? bulk00.go")
	for _, name := range names {
		Expect(porcelain).To(ContainSubstring("?? " + name))
	}

	stdout, stderr, exitCode := runWorkingTreeCodesignal(repo, mode, format)
	Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
	Expect(stderr).To(BeEmpty())
	expectBoundedUntrackedDisclosure(stdout, format, names)
}

func body_workingTreeDisclosureAcceptanceTest_theCLIStillProducesAReportAndRecordsTheFailureAs_233(mode, format string) {
	repo := newTempGitRepo()
	commitBenignHistory(repo)
	Expect(strings.TrimSpace(gitPorcelain(repo))).To(BeEmpty())

	binDir, env := installGitStatusFailureWrapper()
	expectStatusWrapperBehavior(filepath.Join(binDir, "git"), repo)

	args := []string{"codesignal", "--format=" + format}
	if mode == codesignalModeBaseline {
		args = append(args, "--baseline")
	} else {
		args = append(args, "--base", "HEAD~1")
	}
	stdout, stderr, exitCode := runCoachBinary(commandPath, repo, env, args...)
	Expect(exitCode).To(Equal(0),
		"a failed working-tree status check must not fail the run; stderr: %s\nstdout: %s", stderr, stdout)
	Expect(stdout).NotTo(BeEmpty())
	expectStatusCheckFailureDiagnostic(stdout, format)
}

func body_workingTreeDisclosureAcceptanceTest_theCLIRecordsOneStatusFailureAndDoesNotSayWorkWa_332(mode, format string) {
	repo := newTempGitRepo()
	commitGoProjectForDisclosure(repo)
	Expect(strings.TrimSpace(gitPorcelain(repo))).To(BeEmpty())

	binDir, env := installGitStatusFailureWrapper()
	expectStatusWrapperBehavior(filepath.Join(binDir, "git"), repo)

	args := []string{"codesignal", "--project-config", "project.json", "--format=" + format}
	if mode == codesignalModeBaseline {
		args = append(args, "--baseline")
	} else {
		args = append(args, "--base", "HEAD~1")
	}
	stdout, stderr, exitCode := runCoachBinary(commandPath, repo, env, args...)
	Expect(exitCode).To(Equal(0),
		"a failed working-tree status check must not fail the run; stderr: %s\nstdout: %s", stderr, stdout)
	Expect(stdout).NotTo(BeEmpty())
	expectStatusCheckFailureDiagnostic(stdout, format)
	Expect(strings.ToLower(string(stdout))).NotTo(ContainSubstring("not analyzed"),
		"a status-check failure must not be described as skipped analysis; stdout:\n%s", stdout)
	Expect(countKind(diagnosticKinds(stdout, format), codesignal.DiagKindWorktreeChangesNotAnalyzed)).To(Equal(0),
		"status failure must not emit the skip kind; kinds=%v stdout:\n%s", diagnosticKinds(stdout, format), stdout)
}

package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func runWorkingTreeCodesignal(repo, mode, format string) (stdout, stderr []byte, exitCode int) {
	switch mode {
	case codesignalModeBaseline:
		return runCoachCodesignalBaselineRaw(repo, "--format="+format)
	case codesignalModeBase:
		return runCoachCodesignalRaw(repo, "HEAD~1", "--format="+format)
	default:
		Fail("unknown codesignal mode " + mode)
		return nil, nil, -1
	}
}

func assertIntentToAddGoDisclosure(mode, format string) {
	repo := newTempGitRepo()
	commitBenignHistory(repo)
	intentToAddWorktreeFile(repo, "pending.go", hiddenInputMutationGo)
	expectDirtyPorcelain(repo, " A pending.go")

	stdout, stderr, exitCode := runWorkingTreeCodesignal(repo, mode, format)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
	ExpectWithOffset(1, stderr).To(BeEmpty())
	if format == "json" {
		report := decodeCoachReport(stdout)
		ExpectWithOffset(1, signalsForPath(report, "pending.go")).To(BeEmpty(),
			"intent-to-add bytes must not become a signal; signals: %+v", report.Signals)
	} else {
		ExpectWithOffset(1, string(stdout)).NotTo(ContainSubstring("Mutating a caller-owned input"),
			"intent-to-add bytes must not be rendered as a hidden-input-mutation finding; stdout:\n%s", stdout)
	}
	expectSupportedWorktreeDisclosure(stdout, format, []string{"pending.go"}, "untracked", "staged", "modified", "unmerged")
}

func assertStagedGoDisclosure(mode, format string) {
	repo := newTempGitRepo()
	commitBenignHistory(repo)
	stageWorktreeFile(repo, "indexed.go", hiddenInputMutationGo)
	expectDirtyPorcelain(repo, "A  indexed.go")

	stdout, stderr, exitCode := runWorkingTreeCodesignal(repo, mode, format)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
	ExpectWithOffset(1, stderr).To(BeEmpty())
	if format == "json" {
		report := decodeCoachReport(stdout)
		ExpectWithOffset(1, signalsForPath(report, "indexed.go")).To(BeEmpty(),
			"staged bytes must not become a signal; signals: %+v", report.Signals)
	} else {
		ExpectWithOffset(1, string(stdout)).NotTo(ContainSubstring("Mutating a caller-owned input"),
			"staged bytes must not be rendered as a hidden-input-mutation finding; stdout:\n%s", stdout)
	}
	expectSupportedWorktreeDisclosure(stdout, format, []string{"indexed.go"}, "staged", "untracked", "modified")
}

func runWorkingTreeCodesignalWithProject(repo, mode, format string, extra ...string) (stdout, stderr []byte, exitCode int) {
	args := append([]string{"--project-config", "project.json", "--format=" + format}, extra...)
	switch mode {
	case codesignalModeBaseline:
		return runCoachCodesignalBaselineRaw(repo, args...)
	case codesignalModeBase:
		return runCoachCodesignalRaw(repo, "HEAD~1", args...)
	default:
		Fail("unknown codesignal mode " + mode)
		return nil, nil, -1
	}
}

func assertUnmergedBothAddedGoDisclosure(mode, format string) {
	repo := newTempGitRepo()
	commitBenignHistory(repo)
	leaveUnmergedGoFile(repo, "added.go", benignGoA, "package a\n\nfunc Alpha() int { return 3 }\n")
	expectDirtyPorcelain(repo, "AA added.go")

	stdout, stderr, exitCode := runWorkingTreeCodesignal(repo, mode, format)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
	ExpectWithOffset(1, stderr).To(BeEmpty())
	if format == "json" {
		report := decodeCoachReport(stdout)
		ExpectWithOffset(1, len(report.Diagnostics)).To(BeNumerically(">", 0),
			"AA added.go must not be a silent all-clear; diagnostics: %+v", report.Diagnostics)
		ExpectWithOffset(1, signalsForPath(report, "added.go")).To(BeEmpty(),
			"unmerged bytes must not become a signal; signals: %+v", report.Signals)
	} else {
		ExpectWithOffset(1, string(stdout)).NotTo(MatchRegexp(`diagnostics: 0\nNo active CodeSignal findings\.\n\z`),
			"AA added.go must not print an unqualified all-clear with diagnostics:0; stdout:\n%s", stdout)
	}
	expectSupportedWorktreeDisclosure(stdout, format, []string{"added.go"}, "unmerged", "untracked", "staged", "modified")
}

func assertUnmergedGoDisclosure(mode, format string) {
	repo := newTempGitRepo()
	commitBenignHistory(repo)
	commitFile(repo, "conflict.go", benignGoB)
	leaveUnmergedGoFile(repo, "conflict.go", benignGoA, "package a\n\nfunc Alpha() int { return 3 }\n")
	expectDirtyPorcelain(repo, "UU conflict.go")

	stdout, stderr, exitCode := runWorkingTreeCodesignal(repo, mode, format)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
	ExpectWithOffset(1, stderr).To(BeEmpty())
	if format == "json" {
		report := decodeCoachReport(stdout)
		ExpectWithOffset(1, len(report.Diagnostics)).To(BeNumerically(">", 0),
			"UU conflict.go must not be a silent all-clear; diagnostics: %+v", report.Diagnostics)
		ExpectWithOffset(1, signalsForPath(report, "conflict.go")).To(BeEmpty(),
			"unmerged bytes must not become a signal; signals: %+v", report.Signals)
	} else {
		ExpectWithOffset(1, string(stdout)).NotTo(MatchRegexp(`diagnostics: 0\nNo active CodeSignal findings\.\n\z`),
			"UU conflict.go must not print an unqualified all-clear with diagnostics:0; stdout:\n%s", stdout)
	}
	expectSupportedWorktreeDisclosure(stdout, format, []string{"conflict.go"}, "unmerged", "untracked", "staged", "modified")
}

func assertModifiedGoDisclosure(mode, format string) {
	repo := newTempGitRepo()
	commitBenignHistory(repo)
	modifyTrackedFile(repo, "a.go", hiddenInputMutationGo)
	expectDirtyPorcelain(repo, " M a.go")

	stdout, stderr, exitCode := runWorkingTreeCodesignal(repo, mode, format)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
	ExpectWithOffset(1, stderr).To(BeEmpty())
	if format == "json" {
		report := decodeCoachReport(stdout)
		ExpectWithOffset(1, signalsForPath(report, "a.go")).To(BeEmpty(),
			"modified worktree bytes must not become a signal; signals: %+v", report.Signals)
	} else {
		ExpectWithOffset(1, string(stdout)).NotTo(ContainSubstring("Mutating a caller-owned input"),
			"modified worktree bytes must not be rendered as a hidden-input-mutation finding; stdout:\n%s", stdout)
	}
	expectSupportedWorktreeDisclosure(stdout, format, []string{"a.go"}, "modified", "untracked", "staged")
}

func assertUntrackedGoDisclosure(mode, format string) {
	repo := newTempGitRepo()
	commitBenignHistory(repo)
	writeUntrackedFile(repo, "pending.go", hiddenInputMutationGo)
	expectDirtyPorcelain(repo, "?? pending.go")

	stdout, stderr, exitCode := runWorkingTreeCodesignal(repo, mode, format)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
	ExpectWithOffset(1, stderr).To(BeEmpty())
	expectSupportedWorktreeDisclosure(stdout, format, []string{"pending.go"}, "untracked", "staged", "modified")
}

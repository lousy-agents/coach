package main

import (
	"errors"
	"fmt"

	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func expectStatusWrapperBehavior(wrapper, repo string) {
	status := exec.Command(wrapper, "-C", repo, "status", "--porcelain")
	statusOut, statusErr := status.CombinedOutput()
	ExpectWithOffset(1, statusErr).To(HaveOccurred(), "wrapper must fail git status; output: %s", statusOut)
	var exitErr *exec.ExitError
	ExpectWithOffset(1, errors.As(statusErr, &exitErr)).To(BeTrue(), "status wrapper error: %v", statusErr)
	ExpectWithOffset(1, exitErr.ExitCode()).NotTo(Equal(0))

	for _, args := range [][]string{
		{"-C", repo, "rev-parse", "HEAD"},
		{"-C", repo, "ls-tree", "-r", "--name-only", "HEAD"},
		{"-C", repo, "diff", "--name-status", "--find-renames", "--find-copies-harder", "HEAD~1", "HEAD"},
		{"-C", repo, "show", "HEAD:a.go"},
		{"-C", repo, "cat-file", "-e", "HEAD"},
	} {
		cmd := exec.Command(wrapper, args...)
		out, err := cmd.CombinedOutput()
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "wrapper must exec real git for %v: %s", args, out)
	}
}

func bulkUntrackedNames(n int) []string {

	names := make([]string, n)
	for i := 0; i < n; i++ {
		names[i] = fmt.Sprintf("bulk%02d.go", i)
	}
	return names
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

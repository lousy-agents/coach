package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// corruptTheIndex overwrites the repository's index with bytes git cannot read as one. It is a fault real git
// has: every command that reads the index (git status, git ls-files) fails, and every command that reads
// objects and refs only (rev-parse, ls-tree, diff between two commits, show, cat-file) still works.
func corruptTheIndex(repo string) {
	GinkgoHelper()
	Expect(os.WriteFile(filepath.Join(repo, ".git", "index"), []byte("not an index"), 0o644)).To(Succeed())
}

// expectOnlyTheIndexReadsFail proves the fault is the one the specs mean: git status fails, and the commands a
// scan reads history with, run by the same git, do not.
func expectOnlyTheIndexReadsFail(repo string) {
	GinkgoHelper()
	statusOut, statusErr := exec.Command("git", "-C", repo, "status", "--porcelain").CombinedOutput()
	var exitErr *exec.ExitError
	Expect(errors.As(statusErr, &exitErr)).To(BeTrue(), "git status must fail on the corrupt index; output: %s", statusOut)
	Expect(exitErr.ExitCode()).NotTo(Equal(0))

	for _, args := range [][]string{
		{"-C", repo, "rev-parse", "HEAD"},
		{"-C", repo, "ls-tree", "-r", "--name-only", "HEAD"},
		{"-C", repo, "diff", "--name-status", "--find-renames", "--find-copies-harder", "HEAD~1", "HEAD"},
		{"-C", repo, "show", "HEAD:a.go"},
		{"-C", repo, "cat-file", "-e", "HEAD"},
	} {
		out, err := exec.Command("git", args...).CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "git %v must work with the index corrupt: %s", args, out)
	}
}

func expectStatusCheckFailureDiagnostic(stdout []byte, format string) {
	if format == "json" {
		report := decodeCoachReport(stdout)
		ExpectWithOffset(1, report.SchemaVersion).NotTo(BeEmpty(), "stdout must still be a report:\n%s", stdout)
		ExpectWithOffset(1, reportRecordsStatusCheckFailure(report)).To(BeTrue(),
			"a failed working-tree status check must be recorded as a diagnostic; diagnostics: %+v\nstdout:\n%s",
			report.Diagnostics, stdout)
		return
	}
	ExpectWithOffset(1, string(stdout)).To(ContainSubstring("active signals:"),
		"stdout must still be a report when the status check fails:\n%s", stdout)
	found := false
	for _, line := range strings.Split(string(stdout), "\n") {
		if recordsStatusCheckFailure("", line) {
			found = true
		}
	}
	ExpectWithOffset(1, found).To(BeTrue(),
		"a failed working-tree status check must be recorded as a diagnostic; stdout:\n%s", stdout)
}

func reportRecordsStatusCheckFailure(report *codesignal.Report) bool {
	for _, diagnostic := range report.Diagnostics {
		if recordsStatusCheckFailure(diagnostic.Kind, diagnostic.Message) {
			return true
		}
	}
	return false
}

func recordsStatusCheckFailure(kind, message string) bool {
	blob := strings.ToLower(kind + " " + message)
	if !strings.Contains(blob, "status") {
		return false
	}
	for _, word := range []string{"fail", "error", "unable", "could not"} {
		if strings.Contains(blob, word) {
			return true
		}
	}
	return false
}

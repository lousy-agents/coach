package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
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

// installGitStatusFailureWrapper returns a PATH directory whose git fails
// only when the subcommand is status. Coach invokes git as
// "git -C <dir> <subcommand> ...", so options that precede the subcommand
// (and "--name-status", which is a diff option) must not be treated as status.
func installGitStatusFailureWrapper() (binDir string, env []string) {
	realGit, err := exec.LookPath("git")
	Expect(err).NotTo(HaveOccurred())

	binDir, err = os.MkdirTemp("", "coach-git-status-wrapper-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, binDir)

	script := fmt.Sprintf(`#!/bin/sh
real=%q
prev=
for arg in "$@"; do
  if [ -n "$prev" ]; then
    prev=
    continue
  fi
  case "$arg" in
    -C|-c|--git-dir|--work-tree|--namespace)
      prev=$arg
      continue
      ;;
    -*)
      continue
      ;;
    status)
      echo "git status failed" >&2
      exit 1
      ;;
    *)
      exec "$real" "$@"
      ;;
  esac
done
exec "$real" "$@"
`, realGit)
	Expect(os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755)).To(Succeed())
	env = []string{"PATH=" + binDir, "HOME=" + os.Getenv("HOME")}
	return binDir, env
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

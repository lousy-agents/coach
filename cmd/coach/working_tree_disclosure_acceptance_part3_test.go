package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func diagnosticKinds(stdout []byte, format string) []string {
	if format == "json" {
		report := decodeCoachReport(stdout)
		kinds := make([]string, len(report.Diagnostics))
		for i, diagnostic := range report.Diagnostics {
			kinds[i] = diagnostic.Kind
		}
		return kinds
	}
	var kinds []string
	for _, line := range strings.Split(string(stdout), "\n") {
		_, rest, ok := strings.Cut(line, "kind: ")
		if !ok {
			continue
		}
		kind, _, _ := strings.Cut(rest, ",")
		kinds = append(kinds, strings.TrimSpace(kind))
	}
	return kinds
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

func countKind(kinds []string, kind string) int {
	n := 0
	for _, got := range kinds {
		if got == kind {
			n++
		}
	}
	return n
}

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

func expectCleanReportOmitsWorktreeDisclosure(stdout []byte, format, want string) {
	ExpectWithOffset(1, string(stdout)).To(Equal(want),
		"a clean working tree must stay byte-identical to today's unqualified all-clear")
	if format == "text" {
		ExpectWithOffset(1, string(stdout)).To(ContainSubstring("No active CodeSignal findings.\n"))
		ExpectWithOffset(1, string(stdout)).NotTo(ContainSubstring("Diagnostics:"))
		ExpectWithOffset(1, string(stdout)).NotTo(ContainSubstring(codesignal.DiagKindWorktreeChangesNotAnalyzed))
		return
	}
	ExpectWithOffset(1, string(stdout)).NotTo(ContainSubstring(codesignal.DiagKindWorktreeChangesNotAnalyzed))
	ExpectWithOffset(1, string(stdout)).NotTo(ContainSubstring("No active CodeSignal findings.\n"))
}

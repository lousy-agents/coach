package main

import (
	"strconv"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func expectSupportedWorktreeDisclosure(stdout []byte, format string, paths []string, classification string, absentClassifications ...string) {
	body := worktreeDisclosureBody(stdout, format)
	ExpectWithOffset(1, strings.TrimSpace(body)).NotTo(BeEmpty(),
		"unqualified all-clear must not be the whole verdict: kind %s must disclose that %s file(s) %v were not analyzed; stdout:\n%s",
		codesignal.DiagKindWorktreeChangesNotAnalyzed, classification, paths, stdout)
	ExpectWithOffset(1, strings.ToLower(body)).To(ContainSubstring("not analyzed"),
		"disclosure must state the files were not analyzed; body:\n%s\nstdout:\n%s", body, stdout)
	ExpectWithOffset(1, hasWord(body, classification)).To(BeTrue(),
		"disclosure must classify the files as %q; body:\n%s\nstdout:\n%s", classification, body, stdout)
	ExpectWithOffset(1, hasWord(body, strconv.Itoa(len(paths)))).To(BeTrue(),
		"disclosure must report the file count %d; body:\n%s\nstdout:\n%s", len(paths), body, stdout)
	for _, path := range paths {
		ExpectWithOffset(1, body).To(ContainSubstring(path),
			"disclosure must name %s; body:\n%s\nstdout:\n%s", path, body, stdout)
	}
	for _, other := range absentClassifications {
		ExpectWithOffset(1, hasWord(body, other)).To(BeFalse(),
			"a %s-only worktree must not be classified as %q; body:\n%s", classification, other, body)
	}
	if format == "text" {
		ExpectWithOffset(1, string(stdout)).To(ContainSubstring("Diagnostics:"),
			"text output must render the disclosure in a Diagnostics section; stdout:\n%s", stdout)
		if strings.Contains(string(stdout), "No active CodeSignal findings.\n") {
			ExpectWithOffset(1, strings.TrimSpace(body)).NotTo(BeEmpty(),
				"unqualified all-clear must not be the whole verdict; stdout:\n%s", stdout)
		}
	}
}

func expectBoundedUntrackedDisclosure(stdout []byte, format string, names []string) {
	body := worktreeDisclosureBody(stdout, format)
	ExpectWithOffset(1, strings.TrimSpace(body)).NotTo(BeEmpty(),
		"a large untracked set must be disclosed with kind %s, a total count, and a bounded sample; stdout:\n%s",
		codesignal.DiagKindWorktreeChangesNotAnalyzed, stdout)
	ExpectWithOffset(1, strings.ToLower(body)).To(ContainSubstring("not analyzed"))
	ExpectWithOffset(1, hasWord(body, "untracked")).To(BeTrue(), "body:\n%s", body)
	ExpectWithOffset(1, hasWord(body, "staged")).To(BeFalse(), "untracked-only disclosure must not say staged; body:\n%s", body)
	ExpectWithOffset(1, hasWord(body, "modified")).To(BeFalse(), "untracked-only disclosure must not say modified; body:\n%s", body)
	ExpectWithOffset(1, hasWord(body, strconv.Itoa(len(names)))).To(BeTrue(),
		"disclosure must report the total count %d rather than only a sample; body:\n%s\nstdout:\n%s", len(names), body, stdout)

	named := 0
	for _, name := range names {
		if strings.Contains(body, name) {
			named++
		}
	}
	ExpectWithOffset(1, named).To(BeNumerically(">", 0),
		"disclosure must include a bounded sample of file names; body:\n%s", body)
	ExpectWithOffset(1, named).To(BeNumerically("<", len(names)),
		"disclosure must not list every untracked file (%d names present); body:\n%s", named, body)

	kindCount := strings.Count(string(stdout), codesignal.DiagKindWorktreeChangesNotAnalyzed)
	ExpectWithOffset(1, kindCount).To(BeNumerically(">", 0))
	ExpectWithOffset(1, kindCount).To(BeNumerically("<", len(names)),
		"disclosure must not emit one %s diagnostic per file (got %d for %d files); stdout:\n%s",
		codesignal.DiagKindWorktreeChangesNotAnalyzed, kindCount, len(names), stdout)
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

func expectUnsupportedDirtyNotice(stdout []byte) {
	text := string(stdout)
	ExpectWithOffset(1, strings.ToLower(text)).To(ContainSubstring("working tree is not clean"),
		"uncommitted files of an unsupported language must be reported as a dirty working tree without implying analyzable work was skipped; stdout:\n%s", text)
	for _, forbidden := range []string{
		"not analyzed",
		"not_analyzed",
		"supported language",
		"supported-language",
	} {
		ExpectWithOffset(1, strings.ToLower(text)).NotTo(ContainSubstring(forbidden),
			"unsupported uncommitted files must not be described as skipped analyzable work (%q); stdout:\n%s", forbidden, text)
	}
	for _, forbidden := range []string{"incomplete", "skipped"} {
		ExpectWithOffset(1, hasWord(strings.ToLower(text), forbidden)).To(BeFalse(),
			"unsupported uncommitted files must not be described as skipped analyzable work (%q); stdout:\n%s", forbidden, text)
	}
}

package main

import (
	"fmt"
	"regexp"
	"strings"

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

func countKind(kinds []string, kind string) int {
	n := 0
	for _, got := range kinds {
		if got == kind {
			n++
		}
	}
	return n
}

// worktreeDisclosureBody is the diagnostic text whose kind is
// worktree_changes_not_analyzed. Callers assert the disclosure contract
// against this body so a file name that appears only as an analyzed signal
// does not satisfy it.
func worktreeDisclosureBody(stdout []byte, format string) string {
	if format == "json" {
		return reportWorktreeDisclosureBody(decodeCoachReport(stdout))
	}
	var b strings.Builder
	for _, line := range strings.Split(string(stdout), "\n") {
		if strings.Contains(line, codesignal.DiagKindWorktreeChangesNotAnalyzed) {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func expectedCleanBaselineText(revision string, tracked, analyzed int) string {
	return fmt.Sprintf("Repository Baseline for revision %s (not a diff comparison)\ntracked files discovered: %d, analyzed: %d, unsupported: 0, excluded: 0, unanalyzable: 0, active signals: 0, diagnostics: 0\nNo active CodeSignal findings.\n", revision, tracked, analyzed)
}

func expectedCleanDiffText(filesAnalyzed int) string {
	return fmt.Sprintf("scope: production, filtered: 0, files analyzed: %d, active signals: 0, diagnostics: 0\nNo active CodeSignal findings.\n", filesAnalyzed)
}

func expectedCleanBaselineJSON(revision string) string {
	return fmt.Sprintf("{\"schema_version\":\"1\",\"scope\":{\"revision\":%q,\"applied_scope\":\"production\",\"baseline\":true},\"summary\":{\"files_analyzed\":1,\"files_with_diagnostics\":0,\"active_signals\":0,\"introduced_signals\":0,\"existing_signals\":0,\"resolved_signals\":0,\"baseline_signals\":0,\"unknown_signals\":0},\"signals\":[],\"diagnostics\":[],\"coverage\":{\"tracked_files_discovered\":1,\"files_analyzed\":1,\"files_unanalyzable\":0,\"unsupported\":[],\"excluded\":[]}}\n", revision)
}

func expectedCleanBaseJSON(revision, base string) string {
	return fmt.Sprintf("{\"schema_version\":\"1\",\"scope\":{\"revision\":%q,\"base\":%q,\"applied_scope\":\"production\"},\"summary\":{\"files_analyzed\":1,\"files_with_diagnostics\":0,\"active_signals\":0,\"introduced_signals\":0,\"existing_signals\":0,\"resolved_signals\":0,\"baseline_signals\":0,\"unknown_signals\":0},\"signals\":[],\"diagnostics\":[],\"coverage\":{\"tracked_files_discovered\":0,\"files_analyzed\":0,\"files_unanalyzable\":0,\"unsupported\":[],\"excluded\":[]}}\n", revision, base)
}

func hasWord(text, word string) bool {
	return regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`).MatchString(text)
}

func reportWorktreeDisclosureBody(report *codesignal.Report) string {
	var b strings.Builder
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Kind == codesignal.DiagKindWorktreeChangesNotAnalyzed {
			fmt.Fprintf(&b, "%s %s\n", diagnostic.Path, diagnostic.Message)
		}
	}
	return b.String()
}

package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// TestAnalyzeChangesThreadsScopeAndCoverage verifies that AnalyzeChanges
// propagates its appliedScope and excluded parameters into the returned
// Report: a non-empty appliedScope becomes report.Scope.AppliedScope, a
// non-empty excluded becomes report.Coverage.Excluded, and an empty/nil
// excluded leaves report.Coverage nil (rather than a non-nil Coverage with an
// empty Excluded slice).
func TestAnalyzeChangesThreadsScopeAndCoverage(t *testing.T) {
	dir := gitfixture.Init(t)
	initialSHA := gitfixture.CommitFile(t, dir, "healthy.go", "package healthy\n")
	headSHA := gitfixture.CommitFile(t, dir, "healthy.go", "package healthy\n\nfunc Update(input *int) { *input = 1 }\n")

	files := []gitrepo.SelectedFile{
		{Path: "healthy.go", Status: "modified", Language: semantics.LanguageGo},
	}

	t.Run("non-empty scope and excluded", func(t *testing.T) {
		body_analyzePart4Test_nonEmptyScopeAndExcluded_28(t, dir, initialSHA, headSHA, files)
	})

	t.Run("nil excluded leaves Coverage nil", func(t *testing.T) {
		body_analyzePart4Test_nilExcludedLeavesCoverageNil_47(t, dir, initialSHA, headSHA, files)
	})
}

func TestParseChangedRanges(t *testing.T) {
	tests := []struct {
		name    string
		diff    string
		want    []codesignal.LineRange
		wantErr bool
	}{
		{
			name: "first-line insertion",
			diff: "diff --git a/f.go b/f.go\n" +
				"--- a/f.go\n" +
				"+++ b/f.go\n" +
				"@@ -0,0 +1,3 @@\n" +
				"+a\n+b\n+c\n",
			want: []codesignal.LineRange{{StartRow: 0, EndRow: 2}},
		},
		{
			name: "multi-line replacement",
			diff: "@@ -2,2 +2,4 @@\n" +
				"-old1\n-old2\n+new1\n+new2\n+new3\n+new4\n",
			want: []codesignal.LineRange{{StartRow: 1, EndRow: 4}},
		},
		{
			name: "deletion-only hunk emits no range",
			diff: "@@ -5,3 +4,0 @@\n" +
				"-a\n-b\n-c\n",
			want: nil,
		},
		{
			name: "two disjoint hunks",
			diff: "@@ -1,1 +1,1 @@\n" +
				"-a\n+z\n" +
				"@@ -10,1 +10,2 @@\n" +
				"-j\n+k\n+l\n",
			want: []codesignal.LineRange{
				{StartRow: 0, EndRow: 0},
				{StartRow: 9, EndRow: 10},
			},
		},
		{
			name: "eof-adjacent addition",
			diff: "@@ -20,0 +21,2 @@\n" +
				"+x\n+y\n",
			want: []codesignal.LineRange{{StartRow: 20, EndRow: 21}},
		},
		{
			name: "omitted count means 1",
			diff: "@@ -1 +1 @@\n" +
				"-a\n+b\n",
			want: []codesignal.LineRange{{StartRow: 0, EndRow: 0}},
		},
		{
			name: "no hunks",
			diff: "diff --git a/f.go b/f.go\n--- a/f.go\n+++ b/f.go\n",
			want: nil,
		},
		{
			name:    "malformed hunk header",
			diff:    "@@ garbage @@\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_analyzePart4Test_123(t, tt)
		})
	}
}

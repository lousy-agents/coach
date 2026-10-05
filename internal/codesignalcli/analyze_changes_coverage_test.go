package codesignalcli

import (
	"context"
	"reflect"
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
		analyzeChangesNonEmptyScopeExcluded(t, dir, initialSHA, headSHA, files)
	})

	t.Run("nil excluded leaves Coverage nil", func(t *testing.T) {
		nilExcludedLeavesCoverageNil(t, dir, initialSHA, headSHA, files)
	})
}

func analyzeChangesNonEmptyScopeExcluded(t *testing.T, dir string, initialSHA string, headSHA string, files []gitrepo.SelectedFile) {
	excluded := []codesignal.CoverageGroup{{Reason: "test_only", Language: "go", Count: 1}}

	report, err := AnalyzeChanges(context.Background(), dir, headSHA, initialSHA, files, nil, "production", excluded, nil)
	if err != nil {
		t.Fatalf("AnalyzeChanges: unexpected error: %v", err)
	}

	if report.Scope.AppliedScope != "production" {
		t.Errorf("report.Scope.AppliedScope = %q, want %q", report.Scope.AppliedScope, "production")
	}
	if report.Coverage == nil {
		t.Fatal("report.Coverage = nil, want non-nil")
	}
	if !reflect.DeepEqual(report.Coverage.Excluded, excluded) {
		t.Errorf("report.Coverage.Excluded = %#v, want %#v", report.Coverage.Excluded, excluded)
	}
}

func nilExcludedLeavesCoverageNil(t *testing.T, dir string, initialSHA string, headSHA string, files []gitrepo.SelectedFile) {
	report, err := AnalyzeChanges(context.Background(), dir, headSHA, initialSHA, files, nil, "", nil, nil)
	if err != nil {
		t.Fatalf("AnalyzeChanges: unexpected error: %v", err)
	}

	if report.Coverage != nil {
		t.Errorf("report.Coverage = %#v, want nil", report.Coverage)
	}
}

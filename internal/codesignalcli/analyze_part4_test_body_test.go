package codesignalcli

import (
	"context"

	"reflect"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func body_analyzePart4Test_nonEmptyScopeAndExcluded_28(t *testing.T, dir string, initialSHA string, headSHA string, files []SelectedFile) {
	excluded := []codesignal.CoverageGroup{{Reason: "test_only", Language: "go", Count: 1}}

	report, err := AnalyzeChanges(context.Background(), dir, headSHA, initialSHA, files, nil, nil, "production", excluded, nil)
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

func body_analyzePart4Test_nilExcludedLeavesCoverageNil_47(t *testing.T, dir string, initialSHA string, headSHA string, files []SelectedFile) {
	report, err := AnalyzeChanges(context.Background(), dir, headSHA, initialSHA, files, nil, nil, "", nil, nil)
	if err != nil {
		t.Fatalf("AnalyzeChanges: unexpected error: %v", err)
	}

	if report.Coverage != nil {
		t.Errorf("report.Coverage = %#v, want nil", report.Coverage)
	}
}

func body_analyzePart4Test_123(t *testing.T, tt struct {
	name    string
	diff    string
	want    []codesignal.LineRange
	wantErr bool
}) {
	got, err := parseChangedRanges([]byte(tt.diff))
	if tt.wantErr {
		if err == nil {
			t.Fatalf("parseChangedRanges(%q): want error, got nil", tt.diff)
		}
		return
	}
	if err != nil {
		t.Fatalf("parseChangedRanges(%q): unexpected error: %v", tt.diff, err)
	}
	if !rangesEqual(got, tt.want) {
		t.Errorf("parseChangedRanges(%q) = %#v, want %#v", tt.diff, got, tt.want)
	}
}

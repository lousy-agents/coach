package codesignal

import (
	"context"
	"testing"
)

func TestBuild_NilCoverageYieldsNilReportCoverage(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	report, err := b.Build(context.Background(), Input{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if report.Coverage != nil {
		t.Errorf("Report.Coverage: got %+v, want nil", report.Coverage)
	}
}

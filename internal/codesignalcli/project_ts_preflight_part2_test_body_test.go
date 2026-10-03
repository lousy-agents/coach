package codesignalcli

import (
	"context"

	"strings"
	"testing"
)

func body_projectTsPreflightPart2Test_aPolicyOnlyGapListPrintsNothingExtra_48(t *testing.T) {
	if got := AlsoFailingGapLines(&ReadinessResult{Gaps: []ReadinessGap{{Code: GapPolicyMissing}}}, "project.json"); len(got) != 0 {
		t.Fatalf("AlsoFailingGapLines(policy gap only) = %q, want none: the policy failure is already printed as the scan's own message", got)
	}
}

func body_projectTsPreflightPart2Test_nilReadinessPrintsNothingExtra_53(t *testing.T) {
	if got := AlsoFailingGapLines(nil, "project.json"); len(got) != 0 {
		t.Fatalf("AlsoFailingGapLines(nil readiness) = %q, want none", got)
	}
}

func body_projectTsPreflightPart2Test_75(t *testing.T, readiness *ReadinessResult) {
	var out strings.Builder
	result := RunCompilerSetupOffer(context.Background(), ".", "HEAD", "", GapTypescriptCompilerMissing, readiness, strings.NewReader(""), &out)
	if !result.NoChoicesOffered {
		t.Fatalf("NoChoicesOffered = false, want true: %+v", result)
	}
	if result.Cancelled || result.Succeeded || result.Choice != "" {
		t.Fatalf("expected every other outcome field to stay zero, got %+v", result)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no prompt output at all, got %q", out.String())
	}
}

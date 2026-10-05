package codesignalcli

import (
	"testing"
)

func body_projectReadinessPart5Test_66(t *testing.T, tc struct {
	code           string
	wantStatus     ReadinessStatus
	wantNextAction string
}) {
	if got := statusForGapCode(tc.code); got != tc.wantStatus {
		t.Fatalf("statusForGapCode(%q) = %q, want %q", tc.code, got, tc.wantStatus)
	}
	kind, ok := nextActionForGapCode(tc.code)
	if !ok {
		t.Fatalf("nextActionForGapCode(%q) returned ok=false, want %q", tc.code, tc.wantNextAction)
	}
	if kind != tc.wantNextAction {
		t.Fatalf("nextActionForGapCode(%q) = %q, want %q", tc.code, kind, tc.wantNextAction)
	}
}

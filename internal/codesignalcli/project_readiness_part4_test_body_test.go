package codesignalcli

import (
	"testing"
)

func body_projectReadinessPart4Test_89(t *testing.T, tc struct {
	name          string
	checks        ReadinessChecks
	dirtyRelevant bool
	wantStatus    ReadinessStatus
	wantGapCodes  []string
}) {
	status, gaps, _, _ := aggregateReadiness(tc.checks, tc.dirtyRelevant, nil)
	if status != tc.wantStatus {
		t.Fatalf("status = %q, want %q", status, tc.wantStatus)
	}
	if len(gaps) != len(tc.wantGapCodes) {
		t.Fatalf("gaps = %#v, want codes %v", gaps, tc.wantGapCodes)
	}
	for i, code := range tc.wantGapCodes {
		if gaps[i].Code != code {
			t.Fatalf("gaps[%d].Code = %q, want %q (gaps: %#v)", i, gaps[i].Code, code, gaps)
		}
	}
}

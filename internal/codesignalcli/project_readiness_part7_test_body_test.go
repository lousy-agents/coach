package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func body_projectReadinessPart7Test_59(t *testing.T, tc struct {
	name    string
	runtime projectreadiness.Check
}) {
	checks := projectreadiness.Checks{Runtime: tc.runtime, Node: nodeCompatibilityMirror(tc.runtime)}
	_, _, _, warnings := aggregateReadiness(checks, false, nil)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
}

func body_projectReadinessPart7Test_88(t *testing.T, major int) {
	if got, want := nodeMajorSupported(major), analysisNodeMajorAllowed(major); got != want {
		t.Fatalf("nodeMajorSupported(%d) = %t, analysisNodeMajorAllowed(%d) = %t: readiness and analysis gates disagree", major, got, major, want)
	}
}

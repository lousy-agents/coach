package main

import (
	"sort"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/tstestutil"
)

// TestTSTestutilAllowedNodeMajorsMatchesSupportedNodeMajors binds
// tstestutil's PATH-repair fallback (which redirects TS-dependent specs onto
// mise's Node 24 install whenever the host major is outside its own allowed
// set) to codesignalcli.SupportedNodeMajors, the production analysis gate.
// tstestutil cannot import codesignalcli directly (codesignalcli's own
// tests import tstestutil, so the reverse import would cycle); cmd/coach
// already imports both, so this is where the two copies are tied together.
// If the two sets ever diverge, this test must turn red -- otherwise
// tstestutil would silently keep redirecting TS-dependent specs, including
// the Node 26 CI leg's own specs, onto a Node major that production no
// longer certifies, and every wiring spec would stay green regardless.
func TestTSTestutilAllowedNodeMajorsMatchesSupportedNodeMajors(t *testing.T) {
	got := append([]int(nil), tstestutil.AllowedAnalysisNodeMajors...)
	want := append([]int(nil), codesignalcli.SupportedNodeMajors...)
	sort.Ints(got)
	sort.Ints(want)

	if len(got) != len(want) {
		t.Fatalf("tstestutil.AllowedAnalysisNodeMajors = %v, codesignalcli.SupportedNodeMajors = %v: length mismatch", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("tstestutil.AllowedAnalysisNodeMajors = %v, codesignalcli.SupportedNodeMajors = %v: disagree at index %d", got, want, i)
		}
	}
}
